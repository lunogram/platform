package auth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lunogram/platform/internal/jwks"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
)

// trustedIssuerAlgorithms are the asymmetric signing algorithms accepted for
// external JWTs. HMAC (HS*) and "none" are deliberately excluded: a trusted
// issuer authenticates with its own keys, never a secret shared with us.
var trustedIssuerAlgorithms = []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512", "PS256", "PS384", "PS512"}

// WithTrustedIssuer authenticates an externally-issued JWT against a
// trusted_issuer auth method. It resolves the method by the URL project together
// with the token's `iss` (an issuer is scoped to its project, so a token can only
// ever resolve within the project named in its URL), verifies the signature
// against the issuer's JWKS (cached) or configured PEM, enforces `exp`/`iss`/`aud`
// and an asymmetric algorithm, and builds an end-user actor carrying the verified
// subject.
func WithTrustedIssuer(mgmt *management.State, cache *jwks.Cache) Handler {
	return func(ctx context.Context, tokenString string) (context.Context, error) {
		if tokenString == "" {
			return ctx, ErrUnauthorized
		}

		// The URL names the project the token must belong to; resolution is scoped
		// to it so a self-asserted `iss` can never reach another project's method.
		projectID, ok := projectFromRequest(ctx)
		if !ok {
			return ctx, ErrUnauthorized
		}

		// Read the issuer without verifying, to select the verification keys.
		issuer, err := unverifiedIssuer(tokenString)
		if err != nil || issuer == "" {
			return ctx, ErrUnauthorized
		}

		method, err := mgmt.GetTrustedIssuer(ctx, projectID, issuer)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		claims, err := verifyTrustedIssuerToken(ctx, cache, method, tokenString)
		if err != nil {
			return ctx, trustedIssuerTokenError(err)
		}

		subject := claimString(claims, method.SubjectClaim)
		if subject == "" {
			return ctx, rejectTrustedIssuer(fmt.Sprintf("token is missing the %q subject claim", subjectClaimName(method.SubjectClaim)))
		}

		actor := rbac.NewActor(
			rbac.ActorEndUser,
			method.ID.String(),
			rbac.WithOrganizationID(method.OrganizationID),
			rbac.WithProjectID(method.ProjectID),
			rbac.WithSubject(subject, method.Issuer),
			rbac.WithScope(rbac.DataScope(method.SubjectScope)),
		)
		return rbac.WithActor(ctx, actor), nil
	}
}

// unverifiedIssuer extracts the `iss` claim without verifying the signature.
func unverifiedIssuer(tokenString string) (string, error) {
	var claims jwt.MapClaims
	if _, _, err := jwt.NewParser().ParseUnverified(tokenString, &claims); err != nil {
		return "", err
	}
	return claims.GetIssuer()
}

// verifyTrustedIssuerToken verifies the token's signature and standard claims,
// refreshing the JWKS once if the key id is unknown (key rotation).
func verifyTrustedIssuerToken(ctx context.Context, cache *jwks.Cache, method *management.TrustedIssuerAuthMethod, tokenString string) (jwt.MapClaims, error) {
	keyfunc, err := issuerKeyfunc(ctx, cache, method)
	if err != nil {
		return nil, err
	}

	opts := []jwt.ParserOption{
		jwt.WithValidMethods(trustedIssuerAlgorithms),
		jwt.WithIssuer(method.Issuer),
		jwt.WithExpirationRequired(),
	}
	if method.Audience != nil && *method.Audience != "" {
		opts = append(opts, jwt.WithAudience(*method.Audience))
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, keyfunc, opts...)
	if err != nil && method.JWKSURL != nil && *method.JWKSURL != "" {
		// An unknown key id usually means the issuer rotated keys; force a
		// refresh and retry once before giving up.
		if refreshed, rerr := cache.Refresh(ctx, *method.JWKSURL); rerr == nil {
			claims = jwt.MapClaims{}
			token, err = jwt.ParseWithClaims(tokenString, claims, refreshed, opts...)
		}
	}
	// Preserve the underlying cause (a golang-jwt sentinel such as
	// ErrTokenRequiredClaimMissing or ErrTokenExpired) so the caller can turn it
	// into a precise, debuggable message; describeTokenError keeps the surfaced
	// text safe.
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrTokenUnverifiable
	}
	return claims, nil
}

// issuerKeyfunc returns the verification keyfunc for a trusted issuer: the cached
// JWKS when a URL is configured, otherwise the configured PEM public key/cert.
func issuerKeyfunc(ctx context.Context, cache *jwks.Cache, method *management.TrustedIssuerAuthMethod) (jwt.Keyfunc, error) {
	if method.JWKSURL != nil && *method.JWKSURL != "" {
		return cache.Keyfunc(ctx, *method.JWKSURL)
	}
	if method.PublicCert != nil && *method.PublicCert != "" {
		key, err := parsePublicKeyPEM(*method.PublicCert)
		if err != nil {
			return nil, err
		}
		return func(*jwt.Token) (any, error) { return key, nil }, nil
	}
	return nil, errors.New("auth: trusted issuer has neither jwks_url nor public_cert")
}

// parsePublicKeyPEM parses a PEM-encoded public key (PKIX) or certificate and
// returns the public key for signature verification.
func parsePublicKeyPEM(pemData string) (any, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, errors.New("auth: invalid PEM public key")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		return key, nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.New("auth: unsupported public key/certificate PEM")
	}
	return cert.PublicKey, nil
}

// claimString returns the string value of the named claim (defaulting to "sub").
func claimString(claims jwt.MapClaims, name string) string {
	if v, ok := claims[subjectClaimName(name)].(string); ok {
		return v
	}
	return ""
}

// subjectClaimName resolves the configured subject claim, defaulting to "sub".
func subjectClaimName(name string) string {
	if name == "" {
		return "sub"
	}
	return name
}

// rejectedTokenError carries a human-readable reason a token was rejected.
// Unlike ErrUnauthorized it is deliberately NOT part of the generic unauthorized
// set, so the authentication middleware returns it (and its reason) to the caller
// instead of collapsing it into a bare "unauthorized". It is only produced once a
// token has proven to be authentically ours (a resolved trusted issuer, or a
// verified session signature), so the detail aids an integrator debugging their
// token without leaking anything they don't already know.
type rejectedTokenError struct{ msg string }

func (e *rejectedTokenError) Error() string { return e.msg }

func rejectTrustedIssuer(reason string) error {
	return &rejectedTokenError{msg: "trusted-issuer token rejected: " + reason}
}

// trustedIssuerTokenError decides what a verification failure surfaces. Resolving
// the method proves only that the token's self-asserted `iss` names an issuer
// configured for this project: `iss` is read with ParseUnverified, so anyone can
// forge it. Describing a failure on that basis alone would answer "is this issuer
// configured here?" for an unauthenticated caller. Only a claim-level failure
// (ErrTokenInvalidClaims), which the parser reaches solely after the signature has
// verified against the issuer's keys, proves the caller holds a token the issuer
// really minted; every other failure stays a bare ErrUnauthorized.
func trustedIssuerTokenError(err error) error {
	if err == nil || !errors.Is(err, jwt.ErrTokenInvalidClaims) {
		return ErrUnauthorized
	}
	return rejectTrustedIssuer(describeTokenError(err))
}

// describeTokenError maps a JWT claim-validation failure to a concise reason that
// is safe to return to the integrator. It names the standard-claim problems an
// integrator can actually fix (missing/expired/not-yet-valid/issuer/audience) and
// collapses anything else to a generic reason. Callers must first establish that
// the token is authentic (see sessionTokenError / trustedIssuerTokenError):
// signature, malformed-token and key-resolution failures never reach here, and
// must not, since whether they are described at all is itself observable.
func describeTokenError(err error) string {
	switch {
	case errors.Is(err, jwt.ErrTokenRequiredClaimMissing):
		// Both the mandatory "exp" and an enforced "iss"/"aud" land here; name the
		// claim golang-jwt actually reported rather than assuming "exp".
		return requiredClaimReason(err)
	case errors.Is(err, jwt.ErrTokenExpired):
		return "token has expired"
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return `token is not valid yet (its "nbf" not-before claim is in the future)`
	case errors.Is(err, jwt.ErrTokenUsedBeforeIssued):
		return `token was used before its "iat" issued-at time`
	case errors.Is(err, jwt.ErrTokenInvalidIssuer):
		return "token issuer does not match the expected issuer"
	case errors.Is(err, jwt.ErrTokenInvalidAudience):
		return "token audience does not match the expected audience"
	default:
		return "token could not be verified"
	}
}

// requiredClaimReason names the standard claim golang-jwt reported as missing.
// golang-jwt phrases the leaf as `<claim> claim is required`, so an integrator
// sees exactly what to add — most often the mandatory "exp" expiry claim. It
// falls back to a generic (but still exp-centric) message if the library ever
// changes that phrasing.
func requiredClaimReason(err error) string {
	const marker = "missing required claim: "
	msg := err.Error()
	if i := strings.LastIndex(msg, marker); i >= 0 {
		if claim := strings.TrimSuffix(msg[i+len(marker):], " claim is required"); claim != "" && !strings.ContainsRune(claim, ' ') {
			return fmt.Sprintf("token is missing the required %q claim", claim)
		}
	}
	return `token is missing a required claim (the "exp" expiry claim is mandatory)`
}

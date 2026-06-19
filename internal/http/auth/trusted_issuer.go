package auth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"

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
// trusted_issuer auth method. It resolves the method by the token's `iss`,
// verifies the signature against the issuer's JWKS (cached) or configured PEM,
// enforces `exp`/`iss`/`aud` and an asymmetric algorithm, and builds an end-user
// actor carrying the verified subject.
func WithTrustedIssuer(mgmt *management.State, cache *jwks.Cache) Handler {
	return func(ctx context.Context, tokenString string) (context.Context, error) {
		if tokenString == "" {
			return ctx, ErrUnauthorized
		}

		// Read the issuer without verifying, to select the verification keys.
		issuer, err := unverifiedIssuer(tokenString)
		if err != nil || issuer == "" {
			return ctx, ErrUnauthorized
		}

		method, err := mgmt.GetTrustedIssuerByIssuer(ctx, issuer)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		claims, err := verifyTrustedIssuerToken(ctx, cache, method, tokenString)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		subject := claimString(claims, method.SubjectClaim)
		if subject == "" {
			return ctx, ErrUnauthorized
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
	if err != nil || !token.Valid {
		return nil, errors.New("auth: invalid trusted-issuer token")
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
	if name == "" {
		name = "sub"
	}
	if v, ok := claims[name].(string); ok {
		return v
	}
	return ""
}

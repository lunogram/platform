package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
)

// defaultSessionIssuer is the `iss` stamped on session tokens when none is
// configured.
const defaultSessionIssuer = "https://lunogram.com"

// sessionMethodClaim carries the session auth method (policy) id, which defines
// the authorization the session confers.
const sessionMethodClaim = "amid"

// SessionSigner mints and verifies Lunogram-issued session tokens. Sessions are
// signed with ES256 using an EC private key the server holds; verification uses
// the corresponding public key. A nil *SessionSigner means sessions are
// disabled (no signing key configured).
type SessionSigner struct {
	key    *ecdsa.PrivateKey
	issuer string
}

// NewSessionSigner builds a signer from a PEM-encoded EC (P-256) private key,
// stamping tokens with issuer (defaulting to [defaultSessionIssuer]). A blank
// pemKey returns (nil, nil), disabling session minting and verification.
func NewSessionSigner(pemKey, issuer string) (*SessionSigner, error) {
	if pemKey == "" {
		return nil, nil
	}
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, errors.New("auth: session signing key is not valid PEM")
	}
	key, err := parseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	if issuer == "" {
		issuer = defaultSessionIssuer
	}
	return &SessionSigner{key: key, issuer: issuer}, nil
}

// parseECPrivateKey accepts either a SEC1 (`EC PRIVATE KEY`) or PKCS#8
// (`PRIVATE KEY`) EC private key.
func parseECPrivateKey(der []byte) (*ecdsa.PrivateKey, error) {
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if ec, ok := key.(*ecdsa.PrivateKey); ok {
			return ec, nil
		}
		return nil, errors.New("auth: session signing key is not an EC key")
	}
	return nil, errors.New("auth: unsupported session signing key (want a PEM EC private key)")
}

// Mint creates a signed, short-lived ES256 session token for the given subject
// under a session policy (auth method). The token's authorization derives from
// the policy; the subject identifies the end user.
func (s *SessionSigner) Mint(methodID uuid.UUID, subject string, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	now := time.Now()
	expiresAt = now.Add(ttl)
	claims := jwt.MapClaims{
		"iss":              s.issuer,
		"sub":              subject,
		sessionMethodClaim: methodID.String(),
		"iat":              now.Unix(),
		"exp":              expiresAt.Unix(),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodES256, claims).SignedString(s.key)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// SessionSubjectSource is the external-ID source for a session's subject. Like
// trusted issuers, session subjects are namespaced by their policy so they do
// not collide with other identifier sources.
func SessionSubjectSource(methodID uuid.UUID) string {
	return "session:" + methodID.String()
}

// WithSession authenticates a Lunogram-minted session token: it verifies the
// ES256 signature and standard claims, resolves the session policy the token was
// minted under, and builds an end-user actor scoped by that policy and carrying
// the token's subject. It declines when no signer is configured.
func WithSession(mgmt *management.State, signer *SessionSigner) Handler {
	return func(ctx context.Context, tokenString string) (context.Context, error) {
		if tokenString == "" || signer == nil {
			return ctx, ErrUnauthorized
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims,
			func(*jwt.Token) (any, error) { return &signer.key.PublicKey, nil },
			jwt.WithValidMethods([]string{"ES256"}),
			jwt.WithIssuer(signer.issuer),
			jwt.WithExpirationRequired(),
		)
		if err != nil || !token.Valid {
			return ctx, ErrUnauthorized
		}

		methodID, err := uuid.Parse(claimString(claims, sessionMethodClaim))
		if err != nil {
			return ctx, ErrUnauthorized
		}
		subject := claimString(claims, "sub")
		if subject == "" {
			return ctx, ErrUnauthorized
		}

		method, err := mgmt.GetSessionAuthMethod(methodID)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		// The session is only valid on its own project's URL; a token minted for
		// project A may not be presented on a project-B route.
		if err := enforceURLProject(ctx, SurfaceClient, method.ProjectID); err != nil {
			return ctx, err
		}

		actor := rbac.NewActor(
			rbac.ActorEndUser,
			method.ID.String(),
			rbac.WithOrganizationID(method.OrganizationID),
			rbac.WithProjectID(method.ProjectID),
			rbac.WithSubject(subject, SessionSubjectSource(method.ID)),
			rbac.WithScope(rbac.DataScope(method.SubjectScope)),
		)
		return rbac.WithActor(ctx, actor), nil
	}
}

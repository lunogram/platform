package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/store/management"
)

// Console token claims beyond the registered set.
//
// `sid` is the revocation handle: it names the admin_sessions row the request
// resolves against, so a session can be ended server-side instead of being
// trusted until it expires. `amr` records how the admin proved who they are.
// `act` names the impersonator, and only appears on an impersonated session.
const (
	consoleSessionClaim      = "sid"
	consoleMethodsClaim      = "amr"
	consoleActorClaim        = "act"
	consoleActorSubjectClaim = "sub"
)

// ErrConsoleSigningKeyMissing is the startup failure for a configured auth
// driver with no console signing key. It is fatal on purpose: auto-generating an
// ephemeral key would log every admin out on each restart and would break
// multi-replica deployments silently, since each replica would mint tokens the
// others cannot verify.
var ErrConsoleSigningKeyMissing = errors.New(
	"auth: AUTH_DRIVER is set but AUTH_CONSOLE_SIGNING_KEY is empty; " +
		"generate one with: openssl ecparam -name prime256v1 -genkey -noout")

// ConsoleSigner mints and verifies the Lunogram console session token -- the one
// credential every admin login is exchanged for.
//
// The token deliberately carries no email, role, or organization. Those are
// authorization inputs and must be re-read per request: cached in a bearer
// credential, a demoted admin would keep their old role until the token expired.
type ConsoleSigner struct {
	keyring     *Keyring
	issuer      string
	audience    string
	idleTTL     time.Duration
	absoluteTTL time.Duration
}

// NewConsoleSigner builds the signer from configuration. A blank signing key
// yields (nil, nil) so a deployment with no auth driver can start; callers that
// do configure a driver must reject a nil signer via
// [ErrConsoleSigningKeyMissing].
func NewConsoleSigner(cfg config.ConsoleAuth) (*ConsoleSigner, error) {
	keyring, err := NewKeyring(cfg.SigningKey, cfg.PreviousSigningKeys)
	if err != nil {
		return nil, err
	}
	if keyring == nil {
		return nil, nil
	}
	if cfg.Audience == "" {
		return nil, errors.New("auth: console audience must not be empty")
	}
	return &ConsoleSigner{
		keyring:     keyring,
		issuer:      cfg.Issuer,
		audience:    cfg.Audience,
		idleTTL:     cfg.IdleTTL,
		absoluteTTL: cfg.AbsoluteTTL,
	}, nil
}

// IdleTTL is how long a fresh or refreshed session survives unused.
func (s *ConsoleSigner) IdleTTL() time.Duration { return s.idleTTL }

// AbsoluteTTL caps a session's total life however often it is refreshed.
func (s *ConsoleSigner) AbsoluteTTL() time.Duration { return s.absoluteTTL }

// ConsoleClaims is a verified console token, reduced to what the middleware
// acts on. Everything here was proved by the signature; nothing is taken from
// the request.
type ConsoleClaims struct {
	// AdminID is the `sub`, and is ALWAYS a Lunogram admin UUID -- never an
	// upstream provider's user id. That is the invariant that removed the
	// "parse as UUID, else treat as external id" branches this design replaced.
	AdminID   uuid.UUID
	SessionID uuid.UUID
	Methods   []string
	// ImpersonatorSubject is the upstream `act.sub`. It is attribution only:
	// authorization is evaluated entirely as the impersonated admin.
	ImpersonatorSubject string
	ExpiresAt           time.Time
}

// Impersonated reports whether the token was minted for an impersonated session.
func (c *ConsoleClaims) Impersonated() bool { return c.ImpersonatorSubject != "" }

// Mint issues the token for a recorded session. Everything in it derives from
// the session row, so a token can never assert more than the row it names.
func (s *ConsoleSigner) Mint(session *management.AdminSession, methods []string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":               s.issuer,
		"aud":               s.audience,
		"sub":               session.AdminID.String(),
		consoleSessionClaim: session.ID.String(),
		"iat":               now.Unix(),
		"nbf":               now.Unix(),
		"exp":               session.ExpiresAt.Unix(),
	}
	if len(methods) > 0 {
		claims[consoleMethodsClaim] = methods
	}
	if session.Impersonated && session.ImpersonatorSubject != nil {
		claims[consoleActorClaim] = map[string]any{consoleActorSubjectClaim: *session.ImpersonatorSubject}
	}

	return s.keyring.Sign(claims)
}

// Verify checks a console token and returns its claims.
//
// Every property that could otherwise be negotiated by the token itself is
// pinned here: ES256 only (so an `alg: none` or HS256 forgery is rejected before
// any key is consulted), the exact issuer and audience (so a client session
// token -- or a token from another Lunogram deployment -- cannot be replayed at
// the console), an expiry that must be present, and a `kid` that must name a key
// this ring holds.
func (s *ConsoleSigner) Verify(token string) (*ConsoleClaims, error) {
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, s.keyring.Keyfunc,
		jwt.WithValidMethods([]string{"ES256"}),
		jwt.WithIssuer(s.issuer),
		jwt.WithAudience(s.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: console token rejected: %w", err)
	}
	if !parsed.Valid {
		return nil, errors.New("auth: console token is not valid")
	}

	adminID, err := uuid.Parse(claimString(claims, "sub"))
	if err != nil {
		return nil, errors.New("auth: console token subject is not an admin id")
	}
	sessionID, err := uuid.Parse(claimString(claims, consoleSessionClaim))
	if err != nil {
		return nil, errors.New("auth: console token carries no session id")
	}

	expiresAt, err := claims.GetExpirationTime()
	if err != nil || expiresAt == nil {
		return nil, errors.New("auth: console token has no expiry")
	}

	return &ConsoleClaims{
		AdminID:             adminID,
		SessionID:           sessionID,
		Methods:             claimStrings(claims, consoleMethodsClaim),
		ImpersonatorSubject: actorSubject(claims),
		ExpiresAt:           expiresAt.Time,
	}, nil
}

// actorSubject reads `act.sub`. A missing or malformed claim simply means the
// session is not impersonated: whether a given upstream template emits `act` is
// not something a login may fail on.
func actorSubject(claims jwt.MapClaims) string {
	act, ok := claims[consoleActorClaim].(map[string]any)
	if !ok {
		return ""
	}
	subject, _ := act[consoleActorSubjectClaim].(string)
	return subject
}

func claimStrings(claims jwt.MapClaims, key string) []string {
	raw, ok := claims[key].([]any)
	if !ok {
		return nil
	}
	values := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			values = append(values, s)
		}
	}
	return values
}

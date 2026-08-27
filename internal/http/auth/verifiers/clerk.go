package verifiers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/store/management"
	svix "github.com/svix/svix-webhooks/go"
	"go.uber.org/zap"
)

// Clerk verifies a Clerk-issued session token and mirrors Clerk user events.
// It proves the credential and stops: resolving the admin, provisioning and
// session minting all belong to [auth.Exchanger].
type Clerk struct {
	config        config.ClerkAuth
	mgmt          *management.State
	webhookClient *svix.Webhook
	logger        *zap.Logger
	users         *user.Client
	keyFunc       jwt.Keyfunc
	provisioner   Provisioner
}

func NewClerk(cfg config.ClerkAuth, mgmt *management.State, logger *zap.Logger, keyFunc jwt.Keyfunc, provisioner Provisioner) (*Clerk, error) {
	verifier := &Clerk{
		config: cfg,
		mgmt:   mgmt,
		logger: logger,
		users: user.NewClient(&clerk.ClientConfig{
			BackendConfig: clerk.BackendConfig{
				Key: &cfg.SecretKey,
			},
		}),
		keyFunc:     keyFunc,
		provisioner: provisioner,
	}

	if cfg.WebhookSecret != "" {
		var err error
		verifier.webhookClient, err = svix.NewWebhook(cfg.WebhookSecret)
		if err != nil {
			return nil, err
		}
	}

	return verifier, nil
}

func (c *Clerk) Driver() string { return "clerk" }

// clerkClaims is the subset of a Clerk session token this verifier reads.
// Everything here is read only after the signature has been checked.
type clerkClaims struct {
	jwt.RegisteredClaims
	Email         string `json:"email"`
	EmailVerified *bool  `json:"email_verified"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	ImageURL      string `json:"image_url"`
	// Act is present when the Clerk session is itself impersonated. Whether a
	// given Clerk token template emits it is not something we control, so its
	// absence simply means the session is not impersonated -- never a failed
	// login.
	Act *struct {
		Subject string `json:"sub"`
	} `json:"act"`
}

// Verify proves a Clerk session token.
//
// RS256 is pinned. The previous implementation also accepted HS256 against the
// same JWKS keyfunc, which is the shape of the classic algorithm-confusion
// attack; there is no reason for this verifier to accept a symmetric algorithm
// at all, so it does not.
func (c *Clerk) Verify(ctx context.Context, r *http.Request) (*auth.VerifiedIdentity, error) {
	session := auth.GetSession(r)
	if session == "" {
		return nil, ErrNoSession
	}

	var claims clerkClaims
	token, err := jwt.ParseWithClaims(session, &claims, c.keyFunc,
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	if claims.Issuer == "" || claims.Subject == "" {
		return nil, ErrInvalidToken
	}

	identity := &auth.VerifiedIdentity{
		Issuer:        claims.Issuer,
		Subject:       claims.Subject,
		Provider:      management.IdentityProviderClerk,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified != nil && *claims.EmailVerified,
		FirstName:     optional(claims.FirstName),
		LastName:      optional(claims.LastName),
		ImageURL:      optional(claims.ImageURL),
	}
	if claims.ExpiresAt != nil {
		identity.ExpiresAt = claims.ExpiresAt.Time
	}
	if claims.Act != nil && claims.Act.Subject != "" {
		identity.Actor = &auth.VerifiedActor{Subject: claims.Act.Subject}
	}

	// The default Clerk session token carries no profile claims, so the address
	// is fetched from the API when the token does not assert one. The fetched
	// address is trusted for linking only when Clerk reports it as verified.
	if identity.Email == "" {
		if err := c.enrichFromAPI(ctx, identity); err != nil {
			return nil, err
		}
	}

	return identity, nil
}

// enrichFromAPI fills in the profile Clerk's default session token omits.
func (c *Clerk) enrichFromAPI(ctx context.Context, identity *auth.VerifiedIdentity) error {
	remote, err := c.users.Get(ctx, identity.Subject)
	if err != nil {
		return err
	}

	email, verified := primaryEmail(*remote)
	identity.Email = email
	identity.EmailVerified = verified
	if identity.FirstName == nil {
		identity.FirstName = remote.FirstName
	}
	if identity.LastName == nil {
		identity.LastName = remote.LastName
	}
	if identity.ImageURL == nil {
		identity.ImageURL = remote.ImageURL
	}
	return nil
}

// clerkWebhookEvent represents a Clerk webhook event
type clerkWebhookEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Webhook mirrors Clerk user events. It is the [auth.WebhookVerifier] half of
// this driver; the basic verifier simply does not implement that interface,
// rather than carrying a method whose only job is to refuse.
func (c *Clerk) Webhook(ctx context.Context, r *http.Request) error {
	if c.webhookClient == nil {
		return errors.New("webhook client not configured")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	headers := http.Header{
		"svix-id":        []string{r.Header.Get("svix-id")},
		"svix-timestamp": []string{r.Header.Get("svix-timestamp")},
		"svix-signature": []string{r.Header.Get("svix-signature")},
	}

	if err := c.webhookClient.Verify(body, headers); err != nil {
		c.logger.Error("webhook signature verification failed", zap.Error(err))
		return ErrWebhookDenied
	}

	var event clerkWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return err
	}

	switch event.Type {
	case "user.created":
		return c.handleUserCreated(ctx, event.Data)
	case "user.updated":
		return c.handleUserUpdated(ctx, event.Data)
	case "user.deleted":
		return c.handleUserDeleted(ctx, event.Data)
	default:
		c.logger.Debug("ignoring webhook event", zap.String("type", event.Type))
		return nil
	}
}

// handleUserCreated mirrors a new Clerk user ahead of their first login. It goes
// through the same resolution the login path uses, so a webhook can never create
// an admin that a login would have linked to an existing one.
func (c *Clerk) handleUserCreated(ctx context.Context, data json.RawMessage) error {
	remote, err := decodeClerkUser(data)
	if err != nil {
		return err
	}

	identity, err := c.identityFromUser(*remote)
	if err != nil {
		return err
	}

	existing, err := c.mgmt.GetAdminIdentity(ctx, identity.Issuer, identity.Subject)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if existing != nil {
		return nil
	}

	if c.provisioner == nil {
		// Nothing to do: the admin will be provisioned by the exchange on first
		// login. Mirroring early is an optimisation, never a requirement.
		return nil
	}

	_, err = c.provisioner.Provision(ctx, identity)
	return err
}

func (c *Clerk) handleUserUpdated(ctx context.Context, data json.RawMessage) error {
	remote, err := decodeClerkUser(data)
	if err != nil {
		return err
	}

	identity, err := c.identityFromUser(*remote)
	if err != nil {
		return err
	}

	admin, err := c.resolveAdmin(ctx, identity.Issuer, identity.Subject)
	if err != nil || admin == nil {
		return err
	}

	update := management.AdminUpdate{
		Email:     &identity.Email,
		FirstName: identity.FirstName,
		LastName:  identity.LastName,
	}
	return c.mgmt.UpdateAdmin(ctx, *admin, update)
}

func (c *Clerk) handleUserDeleted(ctx context.Context, data json.RawMessage) error {
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if payload.ID == "" {
		return nil
	}

	admin, err := c.resolveAdmin(ctx, c.issuer(), payload.ID)
	if err != nil || admin == nil {
		return err
	}

	// DeleteAdmin also revokes every session the admin holds, so an upstream
	// deletion takes effect on the next request rather than at token expiry.
	return c.mgmt.DeleteAdmin(ctx, *admin)
}

// resolveAdmin finds the admin behind a Clerk subject. It looks the subject up
// under the configured issuer first and falls back to the sentinel issuer that
// the dropped external_id column was backfilled under, so a webhook still
// reaches an admin who has not logged in since the migration.
func (c *Clerk) resolveAdmin(ctx context.Context, issuer, subject string) (*uuid.UUID, error) {
	for _, candidate := range []string{issuer, management.LegacyExternalIDIssuer} {
		if candidate == "" {
			continue
		}
		identity, err := c.mgmt.GetAdminIdentity(ctx, candidate, subject)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return &identity.AdminID, nil
	}
	return nil, nil
}

// identityFromUser builds a verified identity from a webhook payload. The
// payload is trusted only because the svix signature was checked first.
func (c *Clerk) identityFromUser(remote clerk.User) (*auth.VerifiedIdentity, error) {
	email, verified := primaryEmail(remote)
	if email == "" {
		c.logger.Error("clerk user has no primary email", zap.String("id", remote.ID))
		return nil, ErrInvalidEmail
	}

	return &auth.VerifiedIdentity{
		Issuer:        c.issuer(),
		Subject:       remote.ID,
		Provider:      management.IdentityProviderClerk,
		Email:         email,
		EmailVerified: verified,
		FirstName:     remote.FirstName,
		LastName:      remote.LastName,
		ImageURL:      remote.ImageURL,
		ExpiresAt:     time.Time{},
	}, nil
}

// issuer is the `iss` Clerk stamps on this instance's session tokens. A webhook
// payload does not carry one, so it has to be configured for webhook-driven
// mirroring to key identities the same way a login does.
func (c *Clerk) issuer() string { return c.config.Issuer }

// primaryEmail returns the user's primary address and whether Clerk considers it
// verified. A non-primary or unverified address is never treated as verified,
// because a verified address is what permits linking an identity onto an
// existing account.
func primaryEmail(remote clerk.User) (email string, verified bool) {
	if remote.PrimaryEmailAddressID == nil {
		return "", false
	}
	for _, address := range remote.EmailAddresses {
		if address.ID != *remote.PrimaryEmailAddressID {
			continue
		}
		return address.EmailAddress, address.Verification != nil && address.Verification.Status == "verified"
	}
	return "", false
}

func decodeClerkUser(data json.RawMessage) (*clerk.User, error) {
	var remote clerk.User
	if err := json.Unmarshal(data, &remote); err != nil {
		return nil, err
	}
	return &remote, nil
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

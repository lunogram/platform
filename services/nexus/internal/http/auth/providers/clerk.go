package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http/auth"
	"github.com/lunogram/platform/services/nexus/internal/store"
	svix "github.com/svix/svix-webhooks/go"
	"go.uber.org/zap"
)

// ClerkProvider implements Clerk-based authentication
type ClerkProvider struct {
	config        config.ClerkAuth
	stores        *store.State
	webhookClient *svix.Webhook
	logger        *zap.Logger
	users         *user.Client
	keyFunc       jwt.Keyfunc
}

// NewClerkProvider creates a new Clerk auth provider
func NewClerkProvider(cfg config.ClerkAuth, stores *store.State, logger *zap.Logger, keyFunc jwt.Keyfunc) (_ *ClerkProvider, err error) {
	clerk.SetKey(cfg.SecretKey)

	provider := &ClerkProvider{
		config:  cfg,
		stores:  stores,
		logger:  logger,
		users:   user.NewClient(&clerk.ClientConfig{}),
		keyFunc: keyFunc,
	}

	if cfg.WebhookSecret != "" {
		provider.webhookClient, err = svix.NewWebhook(cfg.WebhookSecret)
		if err != nil {
			return nil, err
		}
	}

	return provider, nil
}

// Driver returns the driver identifier
func (p *ClerkProvider) Driver() string {
	return "clerk"
}

func (p *ClerkProvider) Authenticate(ctx context.Context, w http.ResponseWriter, r *http.Request) (context.Context, error) {
	session := auth.GetSession(r)
	if session == "" {
		return ctx, ErrNoSession
	}

	claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(session, &claims, p.keyFunc, jwt.WithValidMethods([]string{"RS256", "HS256"}))
	if err != nil {
		return ctx, err
	}

	if !token.Valid {
		return ctx, ErrInvalidToken
	}

	admin, err := p.stores.GetAdminByExternalID(ctx, claims.Subject)
	if err == nil && admin != nil {
		return ctx, nil
	}

	if err != nil {
		return nil, err
	}

	user, err := p.users.Get(ctx, claims.Subject)
	if err != nil {
		return nil, err
	}

	admin = &store.Admin{
		Email: p.getPrimaryEmail(*user),
		Role:  "owner",
	}

	admin.ExternalID = &claims.Subject
	admin.OrganizationID, err = p.stores.CreateOrganization(ctx, "Default Organization")
	if err != nil {
		return nil, err
	}

	admin.ID, err = p.stores.CreateAdmin(ctx, *admin)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}

// clerkWebhookEvent represents a Clerk webhook event
type clerkWebhookEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Webhook handles Clerk webhook events for user synchronization
func (p *ClerkProvider) Webhook(ctx context.Context, r *http.Request) error {
	if p.webhookClient == nil {
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

	err = p.webhookClient.Verify(body, headers)
	if err != nil {
		p.logger.Error("webhook signature verification failed", zap.Error(err))
		return ErrWebhookDenied
	}

	var event clerkWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return err
	}

	switch event.Type {
	case "user.created":
		return p.handleUserCreated(ctx, event.Data)
	case "user.updated":
		return p.handleUserUpdated(ctx, event.Data)
	case "user.deleted":
		return p.handleUserDeleted(ctx, event.Data)
	default:
		p.logger.Debug("ignoring webhook event", zap.String("type", event.Type))
		return nil
	}
}

func (p *ClerkProvider) handleUserCreated(ctx context.Context, data json.RawMessage) error {
	var user clerk.User
	if err := json.Unmarshal(data, &user); err != nil {
		return err
	}

	admin, _ := p.stores.GetAdminByExternalID(ctx, user.ID)
	if admin != nil {
		return nil
	}

	email := p.getPrimaryEmail(user)
	if email == "" {
		p.logger.Error("clerk user has no primary email", zap.String("id", user.ID))
		return ErrInvalidEmail
	}

	orgID, err := p.stores.CreateOrganization(ctx, "Default Organization")
	if err != nil {
		return err
	}

	externalID := user.ID

	newAdmin := store.Admin{
		OrganizationID: orgID,
		ExternalID:     &externalID,
		Email:          email,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		ImageURL:       user.ImageURL,
		Role:           "owner",
	}

	_, err = p.stores.CreateAdmin(ctx, newAdmin)
	return err
}

func (p *ClerkProvider) handleUserUpdated(ctx context.Context, data json.RawMessage) error {
	var user clerk.User
	if err := json.Unmarshal(data, &user); err != nil {
		return err
	}

	admin, err := p.stores.GetAdminByExternalID(ctx, user.ID)
	if err != nil || admin == nil {
		return nil
	}

	email := p.getPrimaryEmail(user)
	if email == "" {
		p.logger.Error("clerk user has no primary email", zap.String("id", user.ID))
		return ErrInvalidEmail
	}

	update := store.AdminUpdate{
		Email:     &email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}

	return p.stores.UpdateAdmin(ctx, admin.ID, update)
}

func (p *ClerkProvider) handleUserDeleted(ctx context.Context, data json.RawMessage) error {
	var userData struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &userData); err != nil {
		return err
	}

	if userData.ID == "" {
		return nil
	}

	admin, err := p.stores.GetAdminByExternalID(ctx, userData.ID)
	if err != nil || admin == nil {
		return nil
	}

	return p.stores.DeleteAdmin(ctx, admin.ID)
}

func (p *ClerkProvider) getPrimaryEmail(user clerk.User) string {
	if user.PrimaryEmailAddressID == nil {
		return ""
	}
	for _, emailAddr := range user.EmailAddresses {
		if emailAddr.ID == *user.PrimaryEmailAddressID {
			return emailAddr.EmailAddress
		}
	}
	return ""
}

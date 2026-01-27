package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
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
	jwtHandler    auth.Handler
}

// NewClerkProvider creates a new Clerk auth provider
func NewClerkProvider(cfg config.ClerkAuth, stores *store.State, logger *zap.Logger, jwtHandler auth.Handler) (_ *ClerkProvider, err error) {
	provider := &ClerkProvider{
		config:     cfg,
		stores:     stores,
		logger:     logger,
		jwtHandler: jwtHandler,
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

// Validate validates the Clerk JWT token and creates/retrieves admin
func (p *ClerkProvider) Validate(ctx context.Context, r *http.Request) (*store.Admin, error) {
	token := auth.RetrieveAuthToken(r)
	if token == "" {
		return nil, ErrNoToken
	}

	validatedCtx, err := p.jwtHandler(ctx, token)
	if err != nil {
		p.logger.Error("failed to verify clerk token", zap.Error(err))
		return nil, ErrInvalidToken
	}

	claims, err := auth.ParseTokenClaims(token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	subject := claims.Subject()
	if subject == "" {
		return nil, ErrInvalidToken
	}

	admin, err := p.stores.GetAdminByExternalID(validatedCtx, subject)
	if err == nil && admin != nil {
		return admin, nil
	}

	return p.createAdminFromSubject(validatedCtx, subject, r)
}

// createAdminFromSubject creates a new admin when one doesn't exist
func (p *ClerkProvider) createAdminFromSubject(ctx context.Context, subject string, r *http.Request) (*store.Admin, error) {
	var reqBody struct {
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		ImageURL  string `json:"image_url"`
	}

	if r.Body != nil {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &reqBody)
	}

	orgID, err := p.stores.CreateOrganization(ctx, "Default Organization")
	if err != nil {
		return nil, err
	}

	externalID := subject

	admin := store.Admin{
		OrganizationID: orgID,
		ExternalID:     &externalID,
		Email:          reqBody.Email,
		Role:           "owner",
	}

	if reqBody.FirstName != "" {
		admin.FirstName = &reqBody.FirstName
	}
	if reqBody.LastName != "" {
		admin.LastName = &reqBody.LastName
	}
	if reqBody.ImageURL != "" {
		admin.ImageURL = &reqBody.ImageURL
	}

	adminID, err := p.stores.CreateAdmin(ctx, admin)
	if err != nil {
		return nil, err
	}

	admin.ID = adminID
	return &admin, nil
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

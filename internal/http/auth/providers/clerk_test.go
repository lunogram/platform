package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/cloudproud/graceful"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestClerkProviderDriver(t *testing.T) {
	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, nil, zaptest.NewLogger(t), nil)
	require.NoError(t, err)
	require.Equal(t, "clerk", provider.Driver())
}

func TestClerkProviderAuthenticateNoToken(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, nil, logger, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/login/clerk/callback", nil)
	w := httptest.NewRecorder()

	_, err = provider.Authenticate(context.Background(), w, req)
	require.ErrorIs(t, err, ErrNoSession)
}

func TestClerkProviderGetPrimaryEmail(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, nil, logger, nil)
	require.NoError(t, err)

	type test struct {
		user     clerk.User
		expected string
	}

	primaryID := "email_123"
	otherID := "email_456"

	tests := map[string]test{
		"has primary email": {
			user: clerk.User{
				PrimaryEmailAddressID: &primaryID,
				EmailAddresses: []*clerk.EmailAddress{
					{ID: "email_456", EmailAddress: "other@example.com"},
					{ID: "email_123", EmailAddress: "primary@example.com"},
				},
			},
			expected: "primary@example.com",
		},
		"no primary email id": {
			user: clerk.User{
				PrimaryEmailAddressID: nil,
				EmailAddresses: []*clerk.EmailAddress{
					{ID: "email_123", EmailAddress: "test@example.com"},
				},
			},
			expected: "",
		},
		"primary id not found": {
			user: clerk.User{
				PrimaryEmailAddressID: &otherID,
				EmailAddresses: []*clerk.EmailAddress{
					{ID: "email_123", EmailAddress: "test@example.com"},
				},
			},
			expected: "",
		},
		"empty email addresses": {
			user: clerk.User{
				PrimaryEmailAddressID: &primaryID,
				EmailAddresses:        []*clerk.EmailAddress{},
			},
			expected: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := provider.getPrimaryEmail(tc.user)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestClerkProviderWebhookNotConfigured(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, nil, logger, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/clerk/webhook", nil)
	err = provider.Webhook(context.Background(), req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "webhook client not configured")
}

func TestClerkProviderHandleUserCreated(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(management.Config{URI: postgresURI})
	require.NoError(t, err)

	db, err := management.New(ctx, logger, management.Config{URI: postgresURI})
	require.NoError(t, err)

	stores := management.NewState(db)

	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, stores, logger, nil)
	require.NoError(t, err)

	primaryID := "email_primary"
	userData := clerk.User{
		ID:                    "user_clerk123",
		PrimaryEmailAddressID: &primaryID,
		EmailAddresses: []*clerk.EmailAddress{
			{ID: "email_primary", EmailAddress: "clerk@example.com"},
		},
		FirstName: clerk.String("John"),
		LastName:  clerk.String("Doe"),
	}

	data, err := json.Marshal(userData)
	require.NoError(t, err)

	err = provider.handleUserCreated(ctx, data)
	require.NoError(t, err)

	admin, err := stores.GetAdminByExternalID(ctx, "user_clerk123")
	require.NoError(t, err)
	require.NotNil(t, admin)
	require.Equal(t, "clerk@example.com", admin.Email)
	require.Equal(t, "John", *admin.FirstName)
	require.Equal(t, "Doe", *admin.LastName)
	require.Equal(t, "owner", admin.Role)
}

func TestClerkProviderHandleUserCreatedAlreadyExists(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(management.Config{URI: postgresURI})
	require.NoError(t, err)

	db, err := management.New(ctx, logger, management.Config{URI: postgresURI})
	require.NoError(t, err)

	stores := management.NewState(db)

	orgID, err := stores.CreateOrganization(ctx, "Existing Org")
	require.NoError(t, err)

	externalID := "user_existing"
	_, err = stores.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		ExternalID:     &externalID,
		Email:          "existing@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)

	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, stores, logger, nil)
	require.NoError(t, err)

	primaryID := "email_primary"
	userData := clerk.User{
		ID:                    "user_existing",
		PrimaryEmailAddressID: &primaryID,
		EmailAddresses: []*clerk.EmailAddress{
			{ID: "email_primary", EmailAddress: "new@example.com"},
		},
	}

	data, err := json.Marshal(userData)
	require.NoError(t, err)

	err = provider.handleUserCreated(ctx, data)
	require.NoError(t, err)

	admin, err := stores.GetAdminByExternalID(ctx, "user_existing")
	require.NoError(t, err)
	require.Equal(t, "existing@example.com", admin.Email)
}

func TestClerkProviderHandleUserCreatedNoEmail(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(management.Config{URI: postgresURI})
	require.NoError(t, err)

	db, err := management.New(ctx, logger, management.Config{URI: postgresURI})
	require.NoError(t, err)

	stores := management.NewState(db)

	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, stores, logger, nil)
	require.NoError(t, err)

	userData := clerk.User{
		ID:             "user_no_email",
		EmailAddresses: []*clerk.EmailAddress{},
	}

	data, err := json.Marshal(userData)
	require.NoError(t, err)

	err = provider.handleUserCreated(ctx, data)
	require.ErrorIs(t, err, ErrInvalidEmail)
}

func TestClerkProviderHandleUserUpdated(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(management.Config{URI: postgresURI})
	require.NoError(t, err)

	db, err := management.New(ctx, logger, management.Config{URI: postgresURI})
	require.NoError(t, err)

	stores := management.NewState(db)

	orgID, err := stores.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	externalID := "user_to_update"
	firstName := "Old"
	lastName := "Name"
	_, err = stores.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		ExternalID:     &externalID,
		Email:          "old@example.com",
		FirstName:      &firstName,
		LastName:       &lastName,
		Role:           "owner",
	})
	require.NoError(t, err)

	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, stores, logger, nil)
	require.NoError(t, err)

	primaryID := "email_primary"
	userData := clerk.User{
		ID:                    "user_to_update",
		PrimaryEmailAddressID: &primaryID,
		EmailAddresses: []*clerk.EmailAddress{
			{ID: "email_primary", EmailAddress: "new@example.com"},
		},
		FirstName: clerk.String("New"),
		LastName:  clerk.String("Name"),
	}

	data, err := json.Marshal(userData)
	require.NoError(t, err)

	err = provider.handleUserUpdated(ctx, data)
	require.NoError(t, err)

	admin, err := stores.GetAdminByExternalID(ctx, "user_to_update")
	require.NoError(t, err)
	require.Equal(t, "new@example.com", admin.Email)
	require.Equal(t, "New", *admin.FirstName)
}

func TestClerkProviderHandleUserUpdatedNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(management.Config{URI: postgresURI})
	require.NoError(t, err)

	db, err := management.New(ctx, logger, management.Config{URI: postgresURI})
	require.NoError(t, err)

	stores := management.NewState(db)

	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, stores, logger, nil)
	require.NoError(t, err)

	primaryID := "email_primary"
	userData := clerk.User{
		ID:                    "user_nonexistent",
		PrimaryEmailAddressID: &primaryID,
		EmailAddresses: []*clerk.EmailAddress{
			{ID: "email_primary", EmailAddress: "new@example.com"},
		},
	}

	data, err := json.Marshal(userData)
	require.NoError(t, err)

	err = provider.handleUserUpdated(ctx, data)
	require.NoError(t, err)
}

func TestClerkProviderHandleUserDeleted(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(management.Config{URI: postgresURI})
	require.NoError(t, err)

	db, err := management.New(ctx, logger, management.Config{URI: postgresURI})
	require.NoError(t, err)

	stores := management.NewState(db)

	orgID, err := stores.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	externalID := "user_to_delete"
	adminID, err := stores.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		ExternalID:     &externalID,
		Email:          "delete@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)

	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, stores, logger, nil)
	require.NoError(t, err)

	userData := struct {
		ID string `json:"id"`
	}{
		ID: "user_to_delete",
	}

	data, err := json.Marshal(userData)
	require.NoError(t, err)

	err = provider.handleUserDeleted(ctx, data)
	require.NoError(t, err)

	_, err = stores.GetAdmin(ctx, adminID)
	require.Error(t, err)
}

func TestClerkProviderHandleUserDeletedNotFound(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(management.Config{URI: postgresURI})
	require.NoError(t, err)

	db, err := management.New(ctx, logger, management.Config{URI: postgresURI})
	require.NoError(t, err)

	stores := management.NewState(db)

	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, stores, logger, nil)
	require.NoError(t, err)

	userData := struct {
		ID string `json:"id"`
	}{
		ID: "user_nonexistent",
	}

	data, err := json.Marshal(userData)
	require.NoError(t, err)

	err = provider.handleUserDeleted(ctx, data)
	require.NoError(t, err)
}

func TestClerkProviderAuthenticateInvalidToken(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(management.Config{URI: postgresURI})
	require.NoError(t, err)

	db, err := management.New(ctx, logger, management.Config{URI: postgresURI})
	require.NoError(t, err)

	stores := management.NewState(db)

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	}

	cfg := config.ClerkAuth{SecretKey: "test"}
	provider, err := NewClerkProvider(cfg, stores, logger, keyFunc)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/login/clerk/callback", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	_, err = provider.Authenticate(ctx, w, req)
	require.Error(t, err)
}

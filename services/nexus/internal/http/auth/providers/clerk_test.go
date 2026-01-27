package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http/auth"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestClerkProviderDriver(t *testing.T) {
	t.Parallel()

	provider, err := NewClerkProvider(config.ClerkAuth{SecretKey: "test"}, nil, zaptest.NewLogger(t), nil)
	require.NoError(t, err)
	require.Equal(t, "clerk", provider.Driver())
}

func TestClerkProviderValidateNoToken(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	provider, err := NewClerkProvider(config.ClerkAuth{SecretKey: "test"}, nil, logger, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/login/clerk/callback", nil)

	_, err = provider.Validate(context.Background(), req)
	require.ErrorIs(t, err, ErrNoToken)
}

func TestClerkProviderGetPrimaryEmail(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	provider, err := NewClerkProvider(config.ClerkAuth{SecretKey: "test"}, nil, logger, nil)
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
	t.Parallel()

	logger := zaptest.NewLogger(t)
	provider, err := NewClerkProvider(config.ClerkAuth{
		SecretKey: "test",
	}, nil, logger, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/clerk/webhook", nil)
	err = provider.Webhook(context.Background(), req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "webhook client not configured")
}

func TestClerkProviderHandleUserCreated(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	stores := store.NewState(db)

	provider, err := NewClerkProvider(config.ClerkAuth{
		SecretKey: "test",
	}, stores, logger, nil)
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
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	stores := store.NewState(db)

	orgID, err := stores.CreateOrganization(ctx, "Existing Org")
	require.NoError(t, err)

	externalID := "user_existing"
	_, err = stores.CreateAdmin(ctx, store.Admin{
		OrganizationID: orgID,
		ExternalID:     &externalID,
		Email:          "existing@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)

	provider, err := NewClerkProvider(config.ClerkAuth{
		SecretKey: "test",
	}, stores, logger, nil)
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
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	stores := store.NewState(db)

	provider, err := NewClerkProvider(config.ClerkAuth{
		SecretKey: "test",
	}, stores, logger, nil)
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
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	stores := store.NewState(db)

	orgID, err := stores.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	externalID := "user_to_update"
	firstName := "Old"
	lastName := "Name"
	_, err = stores.CreateAdmin(ctx, store.Admin{
		OrganizationID: orgID,
		ExternalID:     &externalID,
		Email:          "old@example.com",
		FirstName:      &firstName,
		LastName:       &lastName,
		Role:           "owner",
	})
	require.NoError(t, err)

	provider, err := NewClerkProvider(config.ClerkAuth{
		SecretKey: "test",
	}, stores, logger, nil)
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
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	stores := store.NewState(db)

	provider, err := NewClerkProvider(config.ClerkAuth{
		SecretKey: "test",
	}, stores, logger, nil)
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
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	stores := store.NewState(db)

	orgID, err := stores.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	externalID := "user_to_delete"
	adminID, err := stores.CreateAdmin(ctx, store.Admin{
		OrganizationID: orgID,
		ExternalID:     &externalID,
		Email:          "delete@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)

	provider, err := NewClerkProvider(config.ClerkAuth{
		SecretKey: "test",
	}, stores, logger, nil)
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
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	stores := store.NewState(db)

	provider, err := NewClerkProvider(config.ClerkAuth{
		SecretKey: "test",
	}, stores, logger, nil)
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

func TestClerkProviderCreateAdminFromSubject(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	stores := store.NewState(db)

	provider, err := NewClerkProvider(config.ClerkAuth{
		SecretKey: "test",
	}, stores, logger, nil)
	require.NoError(t, err)

	reqBody := map[string]string{
		"email":      "newuser@example.com",
		"first_name": "New",
		"last_name":  "User",
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/login/clerk/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	admin, err := provider.createAdminFromSubject(ctx, "subject_123", req)
	require.NoError(t, err)
	require.NotNil(t, admin)
	require.Equal(t, "newuser@example.com", admin.Email)
	require.Equal(t, "New", *admin.FirstName)
	require.Equal(t, "User", *admin.LastName)
	require.Equal(t, "subject_123", *admin.ExternalID)
	require.Equal(t, "owner", admin.Role)
}

func TestClerkProviderValidateWithJWTHandler(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	stores := store.NewState(db)

	jwtHandler := func(ctx context.Context, token string) (context.Context, error) {
		return ctx, auth.ErrUnauthorized
	}

	provider, err := NewClerkProvider(config.ClerkAuth{
		SecretKey: "test",
	}, stores, logger, jwtHandler)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/login/clerk/callback", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	_, err = provider.Validate(ctx, req)
	require.ErrorIs(t, err, ErrInvalidToken)
}

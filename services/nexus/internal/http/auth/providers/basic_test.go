package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/container"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestBasicProviderDriver(t *testing.T) {
	t.Parallel()

	generator := NewHMACJWTGenerator("test-secret", time.Hour)
	provider := NewBasicProvider(config.BasicAuth{}, nil, generator)
	require.Equal(t, "basic", provider.Driver())
}

func TestBasicProviderAuthenticate(t *testing.T) {
	t.Parallel()

	type test struct {
		config      config.BasicAuth
		requestBody map[string]string
		expectErr   error
	}

	tests := map[string]test{
		"missing email": {
			config: config.BasicAuth{
				Email:    "admin@example.com",
				Password: "secret",
			},
			requestBody: map[string]string{
				"password": "secret",
			},
			expectErr: ErrMissingCredentials,
		},
		"missing password": {
			config: config.BasicAuth{
				Email:    "admin@example.com",
				Password: "secret",
			},
			requestBody: map[string]string{
				"email": "admin@example.com",
			},
			expectErr: ErrMissingCredentials,
		},
		"invalid email": {
			config: config.BasicAuth{
				Email:    "admin@example.com",
				Password: "secret",
			},
			requestBody: map[string]string{
				"email":    "wrong@example.com",
				"password": "secret",
			},
			expectErr: ErrInvalidCredentials,
		},
		"invalid password": {
			config: config.BasicAuth{
				Email:    "admin@example.com",
				Password: "secret",
			},
			requestBody: map[string]string{
				"email":    "admin@example.com",
				"password": "wrongpassword",
			},
			expectErr: ErrInvalidCredentials,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			generator := NewHMACJWTGenerator("test-secret", time.Hour)
			provider := NewBasicProvider(tc.config, nil, generator)

			body, err := json.Marshal(tc.requestBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/auth/login/basic/callback", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			_, err = provider.Authenticate(context.Background(), w, req)
			require.ErrorIs(t, err, tc.expectErr)
		})
	}
}

func TestBasicProviderAuthenticateWithExistingAdmin(t *testing.T) {
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

	_, err = stores.CreateAdmin(ctx, store.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)

	providerConfig := config.BasicAuth{
		Email:    "admin@example.com",
		Password: "secret",
	}
	generator := NewHMACJWTGenerator("test-secret", time.Hour)
	provider := NewBasicProvider(providerConfig, stores, generator)

	body, err := json.Marshal(map[string]string{
		"email":    "admin@example.com",
		"password": "secret",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/login/basic/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	_, err = provider.Authenticate(ctx, w, req)
	require.NoError(t, err)
}

func TestBasicProviderAuthenticateCreatesNewAdmin(t *testing.T) {
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

	providerConfig := config.BasicAuth{
		Email:    "newadmin@example.com",
		Password: "secret",
	}
	generator := NewHMACJWTGenerator("test-secret", time.Hour)
	provider := NewBasicProvider(providerConfig, stores, generator)

	body, err := json.Marshal(map[string]string{
		"email":    "newadmin@example.com",
		"password": "secret",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/login/basic/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	_, err = provider.Authenticate(ctx, w, req)
	require.NoError(t, err)

	admin, err := stores.GetAdminByEmail(ctx, "newadmin@example.com")
	require.NoError(t, err)
	require.NotNil(t, admin)
	require.Equal(t, "newadmin@example.com", admin.Email)
	require.Equal(t, "owner", admin.Role)
}

func TestBasicProviderWebhook(t *testing.T) {
	t.Parallel()

	generator := NewHMACJWTGenerator("test-secret", time.Hour)
	provider := NewBasicProvider(config.BasicAuth{}, nil, generator)

	req := httptest.NewRequest(http.MethodPost, "/auth/basic/webhook", nil)
	err := provider.Webhook(context.Background(), req)

	require.Error(t, err)
	require.Contains(t, err.Error(), "webhooks not supported")
}

func TestBasicProviderEmptyBody(t *testing.T) {
	t.Parallel()

	cfg := config.BasicAuth{
		Email:    "admin@example.com",
		Password: "secret",
	}
	generator := NewHMACJWTGenerator("test-secret", time.Hour)
	provider := NewBasicProvider(cfg, nil, generator)

	req := httptest.NewRequest(http.MethodPost, "/auth/login/basic/callback", nil)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	_, err := provider.Authenticate(context.Background(), w, req)
	require.ErrorIs(t, err, ErrMissingCredentials)
}

func TestBasicProviderInvalidJSON(t *testing.T) {
	t.Parallel()

	cfg := config.BasicAuth{
		Email:    "admin@example.com",
		Password: "secret",
	}
	generator := NewHMACJWTGenerator("test-secret", time.Hour)
	provider := NewBasicProvider(cfg, nil, generator)

	req := httptest.NewRequest(http.MethodPost, "/auth/login/basic/callback", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	_, err := provider.Authenticate(context.Background(), w, req)
	require.ErrorIs(t, err, ErrMissingCredentials)
}

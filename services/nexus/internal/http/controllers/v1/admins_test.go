package v1

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/claim"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGetProfileWithInternalAdmin(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	adminsStore := store.NewAdminsStore(db)
	adminID, err := adminsStore.CreateAdmin(ctx, store.Admin{
		Email: "admin@example.com",
		Role:  "member",
	})
	require.NoError(t, err)

	admins := NewAdminsController(logger, db)

	type test struct {
		session claim.Session
		code    int
	}

	tests := map[string]test{
		"success with UUID subject": {
			session: claim.Session{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: adminID.String(),
				},
			},
			code: 200,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/admin/profile", nil)

			ctx := claim.WithSession(req.Context(), tt.session)
			req = req.WithContext(ctx)

			admins.GetProfile(res, req)

			require.Equal(t, tt.code, res.Code, res.Body.String())
		})
	}
}

func TestGetProfileWithExternalAdmin(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	externalID := "user_2abc123def"
	adminsStore := store.NewAdminsStore(db)
	adminID, err := adminsStore.CreateAdmin(ctx, store.Admin{
		ExternalID: &externalID,
		Email:      "external@example.com",
		Role:       "owner",
	})
	require.NoError(t, err)

	admins := NewAdminsController(logger, db)

	type test struct {
		session claim.Session
		code    int
	}

	tests := map[string]test{
		"success with external ID": {
			session: claim.Session{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: externalID,
					Issuer:  "https://clerk.example.com",
				},
			},
			code: 200,
		},
		"fallback to UUID when external ID not found": {
			session: claim.Session{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: adminID.String(),
					Issuer:  "https://clerk.example.com",
				},
			},
			code: 200,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/admin/profile", nil)

			ctx := claim.WithSession(req.Context(), tt.session)
			req = req.WithContext(ctx)

			admins.GetProfile(res, req)

			require.Equal(t, tt.code, res.Code, res.Body.String())
		})
	}
}

func TestGetProfileErrors(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	admins := NewAdminsController(logger, db)

	type test struct {
		setupContext func(context.Context) context.Context
		code         int
	}

	tests := map[string]test{
		"no session in context": {
			setupContext: func(ctx context.Context) context.Context {
				return ctx
			},
			code: 401,
		},
		"empty subject": {
			setupContext: func(ctx context.Context) context.Context {
				session := claim.Session{
					RegisteredClaims: jwt.RegisteredClaims{
						Subject: "",
					},
				}
				return claim.WithSession(ctx, session)
			},
			code: 401,
		},
		"admin not found": {
			setupContext: func(ctx context.Context) context.Context {
				session := claim.Session{
					RegisteredClaims: jwt.RegisteredClaims{
						Subject: uuid.New().String(),
					},
				}
				return claim.WithSession(ctx, session)
			},
			code: 404,
		},
		"invalid UUID format": {
			setupContext: func(ctx context.Context) context.Context {
				session := claim.Session{
					RegisteredClaims: jwt.RegisteredClaims{
						Subject: "not-a-valid-uuid",
					},
				}
				return claim.WithSession(ctx, session)
			},
			code: 401,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/admin/profile", nil)

			ctx := tt.setupContext(req.Context())
			req = req.WithContext(ctx)

			admins.GetProfile(res, req)

			require.Equal(t, tt.code, res.Code, res.Body.String())
		})
	}
}

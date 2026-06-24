package v1

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/lunogram/platform/internal/config"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGetAuthMethods(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Auth: config.Auth{
			Driver: "clerk",
			Clerk: config.ClerkAuth{
				SecretKey: "sk_test_xxx",
			},
		},
	}

	controller, err := NewAuthController(logger, mgmt, cfg, nil)
	require.NoError(t, err)

	type test struct {
		code int
	}

	tests := map[string]test{
		"success": {
			code: 200,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/auth/methods", nil)
			controller.GetAuthMethods(res, req)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result []string
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Contains(t, result, "clerk")
			}
		})
	}
}

func TestAuthCallbackWithInvalidDriver(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Auth: config.Auth{
			Driver: "clerk",
			Clerk: config.ClerkAuth{
				SecretKey: "sk_test_xxx",
			},
		},
	}

	controller, err := NewAuthController(logger, mgmt, cfg, nil)
	require.NoError(t, err)

	type test struct {
		driver oapi.AuthCallbackParamsDriver
		code   int
	}

	tests := map[string]test{
		"invalid driver": {
			driver: "invalid",
			code:   404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/auth/callback/"+string(test.driver), nil)
			controller.AuthCallback(res, req, test.driver)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestAuthWebhookWithInvalidDriver(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Auth: config.Auth{
			Driver: "clerk",
			Clerk: config.ClerkAuth{
				SecretKey: "sk_test_xxx",
			},
		},
	}

	controller, err := NewAuthController(logger, mgmt, cfg, nil)
	require.NoError(t, err)

	type test struct {
		driver oapi.AuthWebhookParamsDriver
		code   int
	}

	tests := map[string]test{
		"invalid driver": {
			driver: "invalid",
			code:   404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/auth/webhook/"+string(test.driver), nil)
			controller.AuthWebhook(res, req, test.driver)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

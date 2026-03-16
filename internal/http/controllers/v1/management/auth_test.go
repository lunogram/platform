package v1

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
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

func TestValidateRedirect(t *testing.T) {
	t.Parallel()

	type test struct {
		redirect             string
		allowedRedirectHosts []string
		wantErr              bool
	}

	tests := map[string]test{
		"relative path is always allowed": {
			redirect:             "/dashboard",
			allowedRedirectHosts: nil,
			wantErr:              false,
		},
		"relative root is always allowed": {
			redirect:             "/",
			allowedRedirectHosts: nil,
			wantErr:              false,
		},
		"protocol-relative url is rejected": {
			redirect:             "//evil.com/steal",
			allowedRedirectHosts: nil,
			wantErr:              true,
		},
		"absolute url with allowed host": {
			redirect:             "https://app.example.com/dashboard",
			allowedRedirectHosts: []string{"app.example.com"},
			wantErr:              false,
		},
		"absolute url with disallowed host": {
			redirect:             "https://evil.example.com/steal",
			allowedRedirectHosts: []string{"app.example.com"},
			wantErr:              true,
		},
		"absolute url with no allowed hosts configured": {
			redirect:             "https://app.example.com/dashboard",
			allowedRedirectHosts: nil,
			wantErr:              true,
		},
		"absolute url host must match exactly": {
			redirect:             "https://notapp.example.com/path",
			allowedRedirectHosts: []string{"app.example.com"},
			wantErr:              true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := &AuthController{
				allowedRedirectHosts: test.allowedRedirectHosts,
			}
			err := c.validateRedirect(test.redirect)
			if test.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAuthCallbackRejectsDisallowedRedirect(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Auth: config.Auth{
			Driver: "clerk",
			Clerk: config.ClerkAuth{
				SecretKey: "sk_test_xxx",
			},
			AllowedRedirectHosts: []string{"app.example.com"},
		},
	}

	controller, err := NewAuthController(logger, mgmt, cfg, nil)
	require.NoError(t, err)

	type test struct {
		redirect string
		code     int
	}

	tests := map[string]test{
		"disallowed absolute redirect returns 400": {
			redirect: "https://evil.com/steal",
			code:     400,
		},
		"allowed absolute redirect passes redirect validation": {
			redirect: "https://app.example.com/dashboard",
			code:     401,
		},
		"relative redirect always passes": {
			redirect: "/dashboard",
			code:     401,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := strings.NewReader(`{"redirect":"` + test.redirect + `"}`)
			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/auth/callback/clerk", body)
			req.Header.Set("Content-Type", "application/json")
			controller.AuthCallback(res, req, oapi.AuthCallbackParamsDriverClerk)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnforceScope(t *testing.T) {
	t.Parallel()

	ptr := func(s string) *string { return &s }

	withOrigin := func() context.Context {
		r := httptest.NewRequest(http.MethodPost, "/api/client/v1/users", nil)
		r.Header.Set("Origin", "https://app.example.com")
		return withRequest(context.Background(), r)
	}
	withoutOrigin := func() context.Context {
		r := httptest.NewRequest(http.MethodPost, "/api/client/v1/users", nil)
		return withRequest(context.Background(), r)
	}

	tests := map[string]struct {
		surface Surface
		scope   *string
		ctx     context.Context
		wantErr bool
	}{
		"management rejects public key":                   {SurfaceManagement, ptr(ScopePublic), context.Background(), true},
		"management allows secret key":                    {SurfaceManagement, ptr(ScopeSecret), context.Background(), false},
		"management treats missing scope as secret":       {SurfaceManagement, nil, context.Background(), false},
		"client rejects secret key from browser":          {SurfaceClient, ptr(ScopeSecret), withOrigin(), true},
		"client allows secret key server-side":            {SurfaceClient, ptr(ScopeSecret), withoutOrigin(), false},
		"client treats missing scope as secret (browser)": {SurfaceClient, nil, withOrigin(), true},
		"client allows public key from browser":           {SurfaceClient, ptr(ScopePublic), withOrigin(), false},
		"client allows public key server-side":            {SurfaceClient, ptr(ScopePublic), withoutOrigin(), false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := enforceScope(tc.ctx, tc.surface, &management.APIKey{Scope: tc.scope})
			if tc.wantErr {
				require.ErrorIs(t, err, ErrUnauthorized)
				return
			}
			assert.NoError(t, err)
		})
	}
}

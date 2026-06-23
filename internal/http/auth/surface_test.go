package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnforceSurface(t *testing.T) {
	t.Parallel()

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
		ctx     context.Context
		wantErr bool
	}{
		"management allows a key (browser-originated)": {SurfaceManagement, withOrigin(), false},
		"management allows a key (server-side)":        {SurfaceManagement, withoutOrigin(), false},
		"client rejects a key from the browser":        {SurfaceClient, withOrigin(), true},
		"client allows a key server-side":              {SurfaceClient, withoutOrigin(), false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := enforceSurface(tc.ctx, tc.surface)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrUnauthorized)
				return
			}
			assert.NoError(t, err)
		})
	}
}

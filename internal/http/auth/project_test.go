package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ctxWithChiProject builds a request context carrying the given projectID as a
// chi URL param, mirroring the middleware position where the route is matched.
func ctxWithChiProject(projectID string) context.Context {
	r := httptest.NewRequest(http.MethodPost, "/api/client/projects/"+projectID+"/users", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("projectID", projectID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	return withRequest(context.Background(), r)
}

func TestProjectFromRequest(t *testing.T) {
	t.Parallel()

	want := uuid.New()

	t.Run("resolves from chi route context", func(t *testing.T) {
		got, ok := projectFromRequest(ctxWithChiProject(want.String()))
		require.True(t, ok)
		assert.Equal(t, want, got)
	})

	t.Run("falls back to the URL path", func(t *testing.T) {
		// No chi route context, only the raw path.
		r := httptest.NewRequest(http.MethodPost, "/api/client/projects/"+want.String()+"/users/events", nil)
		got, ok := projectFromRequest(withRequest(context.Background(), r))
		require.True(t, ok)
		assert.Equal(t, want, got)
	})

	t.Run("fails closed without a project", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/management/projects", nil)
		_, ok := projectFromRequest(withRequest(context.Background(), r))
		assert.False(t, ok)
	})

	t.Run("fails closed on a malformed project", func(t *testing.T) {
		_, ok := projectFromRequest(ctxWithChiProject("not-a-uuid"))
		assert.False(t, ok)
	})
}

func TestEnforceURLProject(t *testing.T) {
	t.Parallel()

	credential := uuid.New()
	other := uuid.New()

	t.Run("management surface ignores the URL project", func(t *testing.T) {
		// No request/project on the context at all.
		err := enforceURLProject(context.Background(), SurfaceManagement, credential)
		assert.NoError(t, err)
	})

	t.Run("client surface accepts a matching project", func(t *testing.T) {
		err := enforceURLProject(ctxWithChiProject(credential.String()), SurfaceClient, credential)
		assert.NoError(t, err)
	})

	t.Run("client surface rejects a credential for another project", func(t *testing.T) {
		err := enforceURLProject(ctxWithChiProject(other.String()), SurfaceClient, credential)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("client surface fails closed when the URL has no project", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/client/users", nil)
		err := enforceURLProject(withRequest(context.Background(), r), SurfaceClient, credential)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})
}

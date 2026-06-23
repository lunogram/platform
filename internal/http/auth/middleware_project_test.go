package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/jwks"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the client-auth middleware (WithKey, WithTrustedIssuer,
// WithSession) end-to-end against a real management store, proving that the URL
// project binding closes the cross-tenant gap: a credential resolved for project
// A can never act on project B's URL, and a credential fails closed when the URL
// names no project. The path helpers (projectFromRequest / enforceURLProject)
// are unit-tested in project_test.go; this file proves the handlers actually
// invoke them on the resolved credential's project.

// clientRequestCtx builds a context carrying an in-flight client request whose
// URL (and chi route context) names urlProject, mirroring the middleware
// position after chi has matched /api/client/projects/{projectID}/...
//
// A blank urlProject yields a client URL with no project segment, used for the
// "URL has no project" fail-closed cases.
func clientRequestCtx(urlProject string) context.Context {
	path := "/api/client/projects/" + urlProject + "/users"
	if urlProject == "" {
		path = "/api/client/users"
	}
	r := httptest.NewRequest(http.MethodPost, path, nil)
	if urlProject != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("projectID", urlProject)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
	return withRequest(context.Background(), r)
}

// twoProjects seeds two independent organizations/projects so a credential made
// in one can be presented on the other's URL.
func twoProjects(t *testing.T, mgmt *management.State) (a, b uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	orgA, err := mgmt.CreateOrganization(ctx, "Org A")
	require.NoError(t, err)
	a, err = mgmt.CreateProject(ctx, management.Project{
		OrganizationID: &orgA, Name: "Project A", Timezone: "UTC", Locale: "en",
	})
	require.NoError(t, err)

	orgB, err := mgmt.CreateOrganization(ctx, "Org B")
	require.NoError(t, err)
	b, err = mgmt.CreateProject(ctx, management.Project{
		OrganizationID: &orgB, Name: "Project B", Timezone: "UTC", Locale: "en",
	})
	require.NoError(t, err)

	return a, b
}

// newMgmt builds a real (Postgres-backed) management State with no Redis cache,
// so the auth lookups read straight from the database.
func newMgmt(t *testing.T) *management.State {
	t.Helper()
	db, _, _ := teststore.RunPostgreSQL(t)
	return management.NewState(db)
}

func TestWithKeyURLProject(t *testing.T) {
	t.Parallel()
	mgmt := newMgmt(t)
	ctx := context.Background()
	projectA, projectB := twoProjects(t, mgmt)

	created, err := mgmt.CreateAuthMethod(ctx, projectA, management.CreateAuthMethodInput{
		Type: management.MethodTypeAPIKey,
		Name: "server key",
		Role: "client",
	})
	require.NoError(t, err)
	require.NotNil(t, created.Secret)
	secret := *created.Secret

	handler := WithKey(mgmt, SurfaceClient)

	t.Run("a key for project A is rejected on project B's URL", func(t *testing.T) {
		_, err := handler(clientRequestCtx(projectB.String()), secret)
		assert.ErrorIs(t, err, ErrUnauthorized, "a project-A key must not act on project B")
	})

	t.Run("a key for project A succeeds on project A's URL", func(t *testing.T) {
		got, err := handler(clientRequestCtx(projectA.String()), secret)
		require.NoError(t, err)
		actor := rbac.FromContext(got)
		require.NotNil(t, actor)
		assert.Equal(t, rbac.ActorAPIKey, actor.Type)
		assert.Equal(t, projectA, actor.ProjectID, "the actor is scoped to its own project")
	})

	t.Run("a valid key fails closed when the URL names no project", func(t *testing.T) {
		_, err := handler(clientRequestCtx(""), secret)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("a valid key fails closed on a malformed project segment", func(t *testing.T) {
		_, err := handler(clientRequestCtx("not-a-uuid"), secret)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("an unknown secret is rejected before any project check", func(t *testing.T) {
		_, err := handler(clientRequestCtx(projectA.String()), "sk_does_not_exist")
		assert.ErrorIs(t, err, ErrUnauthorized)
	})
}

func TestWithTrustedIssuerURLProject(t *testing.T) {
	t.Parallel()
	mgmt := newMgmt(t)
	ctx := context.Background()
	projectA, projectB := twoProjects(t, mgmt)

	// Register the issuer under project A only, verifying with its PEM public key.
	key, method := rsaIssuer(t)
	_, err := mgmt.CreateAuthMethod(ctx, projectA, management.CreateAuthMethodInput{
		Type:         management.MethodTypeTrustedIssuer,
		Name:         "idp",
		Role:         "client",
		SubjectScope: management.SubjectScopeOwn,
		TrustedIssuer: &management.TrustedIssuer{
			PublicCert:   *method.PublicCert,
			Issuer:       testIssuer,
			SubjectClaim: "sub",
		},
	})
	require.NoError(t, err)

	cache := jwks.New(jwks.Config{}, nil, nil, nil) // PEM path; JWKS unused
	handler := WithTrustedIssuer(mgmt, cache)

	token := signRS256(t, key, jwt.MapClaims{
		"iss": testIssuer,
		"sub": "end-user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	t.Run("fails closed when the URL names no project", func(t *testing.T) {
		_, err := handler(clientRequestCtx(""), token)
		assert.ErrorIs(t, err, ErrUnauthorized, "an issuer must not resolve without a URL project")
	})

	t.Run("rejects a token on a project where the issuer is not registered", func(t *testing.T) {
		// The issuer is registered only under A; resolution is project-scoped, so a
		// self-asserted iss cannot reach project A's method via project B's URL.
		_, err := handler(clientRequestCtx(projectB.String()), token)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("authenticates on the matching project URL", func(t *testing.T) {
		got, err := handler(clientRequestCtx(projectA.String()), token)
		require.NoError(t, err)
		actor := rbac.FromContext(got)
		require.NotNil(t, actor)
		assert.Equal(t, rbac.ActorEndUser, actor.Type)
		assert.Equal(t, projectA, actor.ProjectID)
		assert.Equal(t, "end-user-1", actor.Subject)
	})
}

func TestWithSessionURLProject(t *testing.T) {
	t.Parallel()
	mgmt := newMgmt(t)
	ctx := context.Background()
	projectA, projectB := twoProjects(t, mgmt)

	sessionMethod, err := mgmt.CreateAuthMethod(ctx, projectA, management.CreateAuthMethodInput{
		Type:         management.MethodTypeSession,
		Name:         "web sessions",
		Role:         "client",
		SubjectScope: management.SubjectScopeOwn,
		Session:      &management.Session{TTLSeconds: 600},
	})
	require.NoError(t, err)

	signer := testSigner(t, "")
	handler := WithSession(mgmt, signer)

	token, _, err := signer.Mint(sessionMethod.ID, "end-user-9", time.Hour)
	require.NoError(t, err)

	t.Run("a session minted for project A is rejected on project B's URL", func(t *testing.T) {
		_, err := handler(clientRequestCtx(projectB.String()), token)
		assert.ErrorIs(t, err, ErrUnauthorized, "a project-A session must not act on project B")
	})

	t.Run("fails closed when the URL names no project", func(t *testing.T) {
		_, err := handler(clientRequestCtx(""), token)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("authenticates on the matching project URL", func(t *testing.T) {
		got, err := handler(clientRequestCtx(projectA.String()), token)
		require.NoError(t, err)
		actor := rbac.FromContext(got)
		require.NotNil(t, actor)
		assert.Equal(t, rbac.ActorEndUser, actor.Type)
		assert.Equal(t, projectA, actor.ProjectID)
		assert.Equal(t, "end-user-9", actor.Subject)
	})

	t.Run("a nil signer declines (sessions disabled)", func(t *testing.T) {
		_, err := WithSession(mgmt, nil)(clientRequestCtx(projectA.String()), token)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})
}

// TestDeleteAuthMethodNoOpKeepsIssuerSlot proves that a no-op delete (wrong
// project, or an already-deleted method) does not free the trusted-issuer slot:
// only a delete that actually soft-deletes the parent hard-deletes the child
// issuer row. This guards the cross-tenant boundary at the store layer — a
// caller scoped to project B must not be able to free project A's issuer slot.
func TestDeleteAuthMethodNoOpKeepsIssuerSlot(t *testing.T) {
	t.Parallel()
	mgmt := newMgmt(t)
	ctx := context.Background()
	projectA, projectB := twoProjects(t, mgmt)

	const issuer = "https://keep-slot.example"
	created, err := mgmt.CreateAuthMethod(ctx, projectA, management.CreateAuthMethodInput{
		Type:          management.MethodTypeTrustedIssuer,
		Name:          "idp",
		Role:          "client",
		TrustedIssuer: &management.TrustedIssuer{JWKSURL: "https://keep-slot.example/jwks.json", Issuer: issuer},
	})
	require.NoError(t, err)

	t.Run("a delete scoped to the wrong project does not free the slot", func(t *testing.T) {
		// Deleting project A's method through project B is a no-op.
		require.NoError(t, mgmt.DeleteAuthMethod(ctx, projectB, created.ID))

		// The issuer still resolves under project A.
		resolved, err := mgmt.GetTrustedIssuer(ctx, projectA, issuer)
		require.NoError(t, err)
		assert.Equal(t, created.ID, resolved.ID)

		// And the slot is still occupied: re-registering the same issuer in A fails.
		_, err = mgmt.CreateAuthMethod(ctx, projectA, management.CreateAuthMethodInput{
			Type:          management.MethodTypeTrustedIssuer,
			Name:          "dup idp",
			Role:          "client",
			TrustedIssuer: &management.TrustedIssuer{JWKSURL: "https://keep-slot.example/jwks.json", Issuer: issuer},
		})
		assert.Error(t, err, "the issuer slot must remain occupied after a wrong-project delete")
	})

	t.Run("a real delete frees the slot; a repeat delete is a harmless no-op", func(t *testing.T) {
		require.NoError(t, mgmt.DeleteAuthMethod(ctx, projectA, created.ID))
		_, err := mgmt.GetTrustedIssuer(ctx, projectA, issuer)
		assert.Error(t, err, "a deleted issuer no longer resolves")

		// Re-registration now succeeds because the slot was freed.
		reAdded, err := mgmt.CreateAuthMethod(ctx, projectA, management.CreateAuthMethodInput{
			Type:          management.MethodTypeTrustedIssuer,
			Name:          "idp again",
			Role:          "client",
			TrustedIssuer: &management.TrustedIssuer{JWKSURL: "https://keep-slot.example/jwks.json", Issuer: issuer},
		})
		require.NoError(t, err)

		// Deleting the original id again is a no-op and must not touch the live row
		// that now owns the slot.
		require.NoError(t, mgmt.DeleteAuthMethod(ctx, projectA, created.ID))
		resolved, err := mgmt.GetTrustedIssuer(ctx, projectA, issuer)
		require.NoError(t, err, "the re-added issuer must survive a stale repeat delete")
		assert.Equal(t, reAdded.ID, resolved.ID)
	})
}

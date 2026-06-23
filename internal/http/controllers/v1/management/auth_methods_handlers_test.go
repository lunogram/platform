package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rbac/access"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// authMethodsFixture wires up a controller backed by a real management store
// (testcontainers postgres) and an in-memory RBAC engine, with one organization
// and one project that the actor belongs to.
type authMethodsFixture struct {
	controller *AuthMethodsController
	engine     *rbac.Engine
	store      *management.State
	orgID      uuid.UUID
	projectID  uuid.UUID
	actor      *rbac.Actor
}

// setupAuthMethodsController creates the fixture. orgRole is the actor's role on
// its organization ("member", "admin", "owner", or "" to grant no role, which
// makes the actor a non-member for permission checks).
func setupAuthMethodsController(t *testing.T, orgRole string) (authMethodsFixture, context.Context) {
	t.Helper()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	store := management.NewState(mgmt)

	orgID, err := store.OrganizationsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := store.ProjectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           DefaultProject.Name,
		Timezone:       DefaultProject.Timezone,
		Locale:         DefaultProject.Locale,
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, orgRole, "")

	// Link the project to its org and every resource type so that
	// ProjectResourceScope permission checks (used to assert provisioning
	// outcomes) resolve through the method's project role.
	require.NoError(t, access.ProvisionProject(ctx, engine, orgID, projectID))

	controller := NewAuthMethodsController(logger, mgmt, engine)

	return authMethodsFixture{
		controller: controller,
		engine:     engine,
		store:      store,
		orgID:      orgID,
		projectID:  projectID,
		actor:      actor,
	}, actorCtx
}

// createAPIKey persists an api_key method directly through the store and
// provisions its RBAC role tuple, mirroring what a successful create handler
// would leave behind.
func (f authMethodsFixture) createAPIKey(t *testing.T, ctx context.Context, role string) *management.AuthMethod {
	t.Helper()
	method, err := f.store.CreateAuthMethod(ctx, f.projectID, management.CreateAuthMethodInput{
		Type:         management.MethodTypeAPIKey,
		Name:         "key-" + role,
		Role:         role,
		SubjectScope: management.SubjectScopeAll,
	})
	require.NoError(t, err)
	require.NoError(t, access.ProvisionApiKey(ctx, f.engine, method.ID, f.projectID, role))
	return method
}

func mustJSON(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	bb, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(bb)
}

func listAll() store.Pagination {
	return store.Pagination{Limit: 100, Offset: 0}
}

// --- Create -----------------------------------------------------------------

func TestCreateAuthMethodHandler(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "admin")

	body := oapi.CreateAuthMethodJSONRequestBody{
		Type: oapi.AuthMethodTypeApiKey,
		Name: "backend",
		Role: ptr.To(oapi.ProjectRoleSupport),
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/auth_methods", mustJSON(t, body))
	req = req.WithContext(ctx)
	f.controller.CreateAuthMethod(res, req, f.projectID)

	require.Equal(t, 201, res.Code, res.Body.String())

	var out oapi.AuthMethod
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &out))
	assert.Equal(t, "backend", out.Name)
	assert.Equal(t, oapi.AuthMethodTypeApiKey, out.Type)
	require.NotNil(t, out.Secret, "create must return the secret exactly once")

	// The method is persisted and provisioned: a permission check via its
	// identity resolves through the support role.
	stored, err := f.store.GetAuthMethod(ctx, f.projectID, out.Id)
	require.NoError(t, err)
	assert.Equal(t, "support", stored.Role)

	allowed, err := f.engine.Check(ctx, "user:"+out.Id.String(), string(rbac.Read), rbac.ProjectResourceScope("inbox", f.projectID))
	require.NoError(t, err)
	assert.True(t, allowed, "support role should grant read")
}

func TestCreateAuthMethodHandlerInvalidBody(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "admin")

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/auth_methods", bytes.NewReader([]byte("{not json")))
	req = req.WithContext(ctx)
	f.controller.CreateAuthMethod(res, req, f.projectID)

	require.Equal(t, 400, res.Code, res.Body.String())
}

func TestCreateAuthMethodHandlerValidationError(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "admin")

	// api_key may not carry trusted_issuer config; buildCreateAuthMethodInput
	// rejects it before the store is touched.
	body := oapi.CreateAuthMethodJSONRequestBody{
		Type:          oapi.AuthMethodTypeApiKey,
		Name:          "backend",
		TrustedIssuer: &oapi.TrustedIssuer{Iss: ptr.To("https://idp.example")},
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/auth_methods", mustJSON(t, body))
	req = req.WithContext(ctx)
	f.controller.CreateAuthMethod(res, req, f.projectID)

	require.Equal(t, 400, res.Code, res.Body.String())
}

// TestCreateAuthMethodHandlerRollsBackOnProvisionFailure exercises the riskiest
// path: when RBAC provisioning fails, the just-created method must be rolled
// back so no usable credential is left without its authorization. We force a
// provision failure by creating the method with a role that is not a valid
// relation in the OpenFGA model; the store happily persists the string, but the
// tuple write is rejected.
func TestCreateAuthMethodHandlerRollsBackOnProvisionFailure(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "admin")

	body := oapi.CreateAuthMethodJSONRequestBody{
		Type: oapi.AuthMethodTypeApiKey,
		Name: "doomed",
		Role: ptr.To(oapi.ProjectRole("not_a_real_role")),
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/auth_methods", mustJSON(t, body))
	req = req.WithContext(ctx)
	f.controller.CreateAuthMethod(res, req, f.projectID)

	require.NotEqual(t, 201, res.Code, "provisioning must fail")
	require.GreaterOrEqual(t, res.Code, 400, res.Body.String())

	// The method must not survive: listing the project returns nothing.
	methods, total, err := f.store.ListAuthMethods(ctx, f.projectID, listAll())
	require.NoError(t, err)
	assert.Equal(t, 0, total, "rolled-back method must not remain")
	assert.Empty(t, methods)
}

// --- List / Get -------------------------------------------------------------

func TestListAuthMethodsHandler(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "admin")
	f.createAPIKey(t, ctx, "support")
	f.createAPIKey(t, ctx, "editor")

	limit := oapi.Limit(10)
	offset := oapi.Offset(0)
	params := oapi.ListAuthMethodsParams{Limit: &limit, Offset: &offset}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/auth_methods", nil)
	req = req.WithContext(ctx)
	f.controller.ListAuthMethods(res, req, f.projectID, params)

	require.Equal(t, 200, res.Code, res.Body.String())

	var out oapi.AuthMethodListResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &out))
	assert.Equal(t, 2, out.Total)
	require.Len(t, out.Results, 2)
	for _, m := range out.Results {
		assert.Nil(t, m.Secret, "list must never expose secrets")
	}
}

func TestGetAuthMethodHandler(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "admin")
	method := f.createAPIKey(t, ctx, "support")

	t.Run("found", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/v1/auth_methods/"+method.ID.String(), nil)
		req = req.WithContext(ctx)
		f.controller.GetAuthMethod(res, req, f.projectID, method.ID)

		require.Equal(t, 200, res.Code, res.Body.String())
		var out oapi.AuthMethod
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &out))
		assert.Equal(t, method.ID, out.Id)
		assert.Nil(t, out.Secret, "get must never expose the secret")
	})

	t.Run("not found", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/v1/auth_methods/x", nil)
		req = req.WithContext(ctx)
		f.controller.GetAuthMethod(res, req, f.projectID, uuid.New())

		require.Equal(t, 404, res.Code, res.Body.String())
	})
}

// --- Update -----------------------------------------------------------------

func TestUpdateAuthMethodHandler(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "admin")
	method := f.createAPIKey(t, ctx, "support")

	body := oapi.UpdateAuthMethodJSONRequestBody{
		Name: ptr.To("renamed"),
		Role: ptr.To(oapi.ProjectRoleEditor),
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/v1/auth_methods/"+method.ID.String(), mustJSON(t, body))
	req = req.WithContext(ctx)
	f.controller.UpdateAuthMethod(res, req, f.projectID, method.ID)

	require.Equal(t, 200, res.Code, res.Body.String())

	var out oapi.AuthMethod
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &out))
	assert.Equal(t, "renamed", out.Name)
	assert.Equal(t, oapi.ProjectRole("editor"), out.Role)

	// Re-provision moved the scope: the old support tuple is gone and the new
	// editor tuple grants create.
	allowed, err := f.engine.Check(ctx, "user:"+method.ID.String(), string(rbac.Create), rbac.ProjectResourceScope("inbox", f.projectID))
	require.NoError(t, err)
	assert.True(t, allowed, "editor role should grant create")
}

func TestUpdateAuthMethodHandlerNotFound(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "admin")

	body := oapi.UpdateAuthMethodJSONRequestBody{Name: ptr.To("x")}
	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/v1/auth_methods/x", mustJSON(t, body))
	req = req.WithContext(ctx)
	f.controller.UpdateAuthMethod(res, req, f.projectID, uuid.New())

	require.Equal(t, 404, res.Code, res.Body.String())
}

func TestUpdateAuthMethodHandlerInvalidGrant(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "admin")
	method := f.createAPIKey(t, ctx, "support")

	body := oapi.UpdateAuthMethodJSONRequestBody{
		Grants: &[]oapi.PermissionGrant{{Resource: "not_a_resource", Verb: "read"}},
	}
	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/v1/auth_methods/"+method.ID.String(), mustJSON(t, body))
	req = req.WithContext(ctx)
	f.controller.UpdateAuthMethod(res, req, f.projectID, method.ID)

	require.Equal(t, 400, res.Code, res.Body.String())
}

// TestUpdateAuthMethodHandlerSubjectScope covers subjectScopeFor on the update
// path: an api_key may not be confined to "own".
func TestUpdateAuthMethodHandlerSubjectScope(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "admin")

	t.Run("api_key rejects own scope", func(t *testing.T) {
		method := f.createAPIKey(t, ctx, "support")
		body := oapi.UpdateAuthMethodJSONRequestBody{
			SubjectScope: ptr.To(oapi.SubjectScope("own")),
		}
		res := httptest.NewRecorder()
		req := httptest.NewRequest("PATCH", "/v1/auth_methods/"+method.ID.String(), mustJSON(t, body))
		req = req.WithContext(ctx)
		f.controller.UpdateAuthMethod(res, req, f.projectID, method.ID)

		require.Equal(t, 400, res.Code, res.Body.String())
	})

	t.Run("trusted_issuer accepts all scope", func(t *testing.T) {
		method, err := f.store.CreateAuthMethod(ctx, f.projectID, management.CreateAuthMethodInput{
			Type:         management.MethodTypeTrustedIssuer,
			Name:         "idp-scope",
			Role:         "support",
			SubjectScope: management.SubjectScopeOwn,
			TrustedIssuer: &management.TrustedIssuer{
				JWKSURL: "https://idp.example/jwks.json",
				Issuer:  "https://idp-scope.example",
			},
		})
		require.NoError(t, err)
		require.NoError(t, access.ProvisionApiKey(ctx, f.engine, method.ID, f.projectID, "support"))

		body := oapi.UpdateAuthMethodJSONRequestBody{
			SubjectScope: ptr.To(oapi.SubjectScope("all")),
		}
		res := httptest.NewRecorder()
		req := httptest.NewRequest("PATCH", "/v1/auth_methods/"+method.ID.String(), mustJSON(t, body))
		req = req.WithContext(ctx)
		f.controller.UpdateAuthMethod(res, req, f.projectID, method.ID)

		require.Equal(t, 200, res.Code, res.Body.String())
		var out oapi.AuthMethod
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &out))
		require.NotNil(t, out.SubjectScope)
		assert.Equal(t, oapi.SubjectScope("all"), *out.SubjectScope)
	})
}

// TestUpdateAuthMethodHandlerRestoresScopeOnReprovisionFailure covers the
// restore-on-error branch of reprovision: when provisioning the new scope fails,
// the previous scope is restored so a failed update never strips the method of
// its authorization.
func TestUpdateAuthMethodHandlerRestoresScopeOnReprovisionFailure(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "admin")
	method := f.createAPIKey(t, ctx, "support")

	// Update to an invalid role: store update succeeds, deprovision of the old
	// support tuple succeeds, provision of the bogus role fails, and reprovision
	// restores the support tuple.
	body := oapi.UpdateAuthMethodJSONRequestBody{
		Role: ptr.To(oapi.ProjectRole("not_a_real_role")),
	}
	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/v1/auth_methods/"+method.ID.String(), mustJSON(t, body))
	req = req.WithContext(ctx)
	f.controller.UpdateAuthMethod(res, req, f.projectID, method.ID)

	require.GreaterOrEqual(t, res.Code, 400, res.Body.String())

	// The original support tuple must still resolve.
	allowed, err := f.engine.Check(ctx, "user:"+method.ID.String(), string(rbac.Read), rbac.ProjectResourceScope("inbox", f.projectID))
	require.NoError(t, err)
	assert.True(t, allowed, "support scope must be restored after reprovision failure")
}

// --- Delete -----------------------------------------------------------------

func TestDeleteAuthMethodHandler(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "owner") // delete requires owner
	method := f.createAPIKey(t, ctx, "support")

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/auth_methods/"+method.ID.String(), nil)
	req = req.WithContext(ctx)
	f.controller.DeleteAuthMethod(res, req, f.projectID, method.ID)

	require.Equal(t, 204, res.Code, res.Body.String())

	_, err := f.store.GetAuthMethod(ctx, f.projectID, method.ID)
	require.Error(t, err, "method must be soft-deleted")
}

func TestDeleteAuthMethodHandlerNotFound(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "owner") // delete requires owner

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/auth_methods/x", nil)
	req = req.WithContext(ctx)
	f.controller.DeleteAuthMethod(res, req, f.projectID, uuid.New())

	require.Equal(t, 404, res.Code, res.Body.String())
}

// --- authorizeProject -------------------------------------------------------

// TestAuthorizeProjectDeniesNonMember verifies the org-permission check denies
// an actor with no role on its organization.
func TestAuthorizeProjectDeniesNonMember(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "") // no org role granted

	limit := oapi.Limit(10)
	offset := oapi.Offset(0)
	params := oapi.ListAuthMethodsParams{Limit: &limit, Offset: &offset}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/auth_methods", nil)
	req = req.WithContext(ctx)
	f.controller.ListAuthMethods(res, req, f.projectID, params)

	require.Equal(t, 403, res.Code, res.Body.String())
}

// TestAuthorizeProjectDeniesInsufficientRole verifies that a verb the actor's
// org role does not grant (create needs admin) is forbidden for a mere member.
func TestAuthorizeProjectDeniesInsufficientRole(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "member") // member can read, not create

	body := oapi.CreateAuthMethodJSONRequestBody{
		Type: oapi.AuthMethodTypeApiKey,
		Name: "backend",
	}
	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/auth_methods", mustJSON(t, body))
	req = req.WithContext(ctx)
	f.controller.CreateAuthMethod(res, req, f.projectID)

	require.Equal(t, 403, res.Code, res.Body.String())
}

// TestAuthorizeProjectCrossOrgReturns404 verifies that accessing a project that
// exists but belongs to a different organization is reported as 404 so its
// existence is not revealed (cross-organization access via a guessed projectID).
func TestAuthorizeProjectCrossOrgReturns404(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "owner")

	// A project in a different organization.
	otherOrgID, err := f.store.OrganizationsStore.CreateOrganization(ctx, "Other Org")
	require.NoError(t, err)
	otherProjectID, err := f.store.ProjectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &otherOrgID,
		Name:           DefaultProject.Name,
		Timezone:       DefaultProject.Timezone,
		Locale:         DefaultProject.Locale,
	})
	require.NoError(t, err)

	limit := oapi.Limit(10)
	offset := oapi.Offset(0)
	params := oapi.ListAuthMethodsParams{Limit: &limit, Offset: &offset}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/auth_methods", nil)
	req = req.WithContext(ctx)
	f.controller.ListAuthMethods(res, req, otherProjectID, params)

	require.Equal(t, 404, res.Code, res.Body.String())
}

// TestAuthorizeProjectMissingReturns404 verifies that a guessed, non-existent
// projectID returns 404.
func TestAuthorizeProjectMissingReturns404(t *testing.T) {
	t.Parallel()

	f, ctx := setupAuthMethodsController(t, "owner")

	limit := oapi.Limit(10)
	offset := oapi.Offset(0)
	params := oapi.ListAuthMethodsParams{Limit: &limit, Offset: &offset}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/auth_methods", nil)
	req = req.WithContext(ctx)
	f.controller.ListAuthMethods(res, req, uuid.New(), params)

	require.Equal(t, 404, res.Code, res.Body.String())
}

// --- pure validators (no Docker needed) -------------------------------------

// TestValidateGrantsUnknownVerb covers the unknown-verb branch of validateGrants
// (a known resource paired with a verb that is not one of read/create/update/
// delete). The unknown-resource branch is already covered elsewhere.
func TestValidateGrantsUnknownVerb(t *testing.T) {
	t.Parallel()

	resources := rbac.Resources()
	require.NotEmpty(t, resources, "model must expose at least one resource")
	known := resources[0]

	t.Run("rejects an unknown verb on a known resource", func(t *testing.T) {
		err := validateGrants([]management.Grant{{Resource: known, Verb: "list"}})
		assert.Error(t, err)
	})

	t.Run("accepts known resource and verb", func(t *testing.T) {
		err := validateGrants([]management.Grant{{Resource: known, Verb: "read"}})
		assert.NoError(t, err)
	})

	t.Run("empty grant set is a no-op", func(t *testing.T) {
		assert.NoError(t, validateGrants(nil))
	})
}

func TestValidateJWKSURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		raw string
		ok  bool
	}{
		"https public host":          {raw: "https://idp.example/jwks.json", ok: true},
		"non-https scheme":           {raw: "http://idp.example/jwks.json"},
		"missing host":               {raw: "https:///jwks.json"},
		"parse error":                {raw: "https://exa mple.com/jwks"},
		"loopback literal":           {raw: "https://127.0.0.1/jwks.json"},
		"private literal":            {raw: "https://10.0.0.5/jwks.json"},
		"link-local literal":         {raw: "https://169.254.169.254/jwks.json"},
		"unspecified literal":        {raw: "https://0.0.0.0/jwks.json"},
		"public ip literal accepted": {raw: "https://8.8.8.8/jwks.json", ok: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateJWKSURL(tt.raw)
			if tt.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

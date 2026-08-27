package rbac

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteTupleGrantsAccess(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	ctx := context.Background()
	orgID := uuid.New()
	obj := "organization:" + orgID.String()

	require.NoError(t, engine.WriteTuple(ctx, "user:u1", "member", obj))

	allowed, err := engine.Check(ctx, "user:u1", "member", obj)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestWriteTupleIfAbsentIsIdempotent(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	ctx := context.Background()
	orgID := uuid.New()
	obj := "organization:" + orgID.String()

	// First write creates the tuple; the duplicate is treated as success.
	require.NoError(t, engine.WriteTupleIfAbsent(ctx, "user:u1", "member", obj))
	require.NoError(t, engine.WriteTupleIfAbsent(ctx, "user:u1", "member", obj),
		"a duplicate write must be idempotent, not an error")

	allowed, err := engine.Check(ctx, "user:u1", "member", obj)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestWriteTupleIfAbsentSurfacesRealErrors(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	ctx := context.Background()
	obj := "organization:" + uuid.New().String()

	// A relation that does not exist in the authorization model is a genuine
	// validation failure, not a duplicate, and must be surfaced rather than
	// swallowed as "already exists".
	err := engine.WriteTupleIfAbsent(ctx, "user:u1", "not_a_real_relation", obj)
	require.Error(t, err, "a non-duplicate write failure must surface")
}

func TestDeleteTupleRevokesAccess(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	ctx := context.Background()
	orgID := uuid.New()
	obj := "organization:" + orgID.String()

	require.NoError(t, engine.WriteTuple(ctx, "user:u1", "member", obj))
	require.NoError(t, engine.DeleteTuple(ctx, "user:u1", "member", obj))

	allowed, err := engine.Check(ctx, "user:u1", "member", obj)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestWriteTuplesBatchGrantsAccess(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	ctx := context.Background()
	orgID := uuid.New()
	obj := "organization:" + orgID.String()

	tuples := []Tuple{
		{User: "user:a", Relation: "member", Object: obj},
		{User: "user:b", Relation: "admin", Object: obj},
	}
	require.NoError(t, engine.WriteTuples(ctx, tuples))

	allowed, err := engine.Check(ctx, "user:a", "member", obj)
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = engine.Check(ctx, "user:b", "admin", obj)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestDeleteTuplesBatchRevokesAccess(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	ctx := context.Background()
	orgID := uuid.New()
	obj := "organization:" + orgID.String()

	tuples := []Tuple{
		{User: "user:a", Relation: "member", Object: obj},
		{User: "user:b", Relation: "member", Object: obj},
	}
	require.NoError(t, engine.WriteTuples(ctx, tuples))
	require.NoError(t, engine.DeleteTuples(ctx, tuples))

	allowed, err := engine.Check(ctx, "user:a", "member", obj)
	require.NoError(t, err)
	assert.False(t, allowed)

	allowed, err = engine.Check(ctx, "user:b", "member", obj)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestWriteAndDeleteTuplesNoopOnEmpty(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	ctx := context.Background()

	assert.NoError(t, engine.WriteTuples(ctx, nil))
	assert.NoError(t, engine.WriteTuples(ctx, []Tuple{}))
	assert.NoError(t, engine.DeleteTuples(ctx, nil))
	assert.NoError(t, engine.DeleteTuples(ctx, []Tuple{}))
}

func TestAllowedReturnsUnauthorizedWithoutActor(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	err := engine.Allowed(context.Background(), Read, OrganizationScope(uuid.New()))
	assert.Error(t, err)
}

func TestAllowedReturnsForbiddenWithoutPermission(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	actor := NewActor(ActorAdmin, "u1", WithOrganizationID(orgID))
	engine := NewTestEngine(t)
	ctx := WithActor(context.Background(), actor)

	// No tuples written -- actor has no roles.
	err := engine.Allowed(ctx, Read, OrganizationScope(orgID))
	assert.Error(t, err)
}

func TestAllowedSucceedsWithPermission(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	actor := NewActor(ActorAdmin, "u1", WithOrganizationID(orgID))
	engine := NewTestEngine(t)
	ctx := WithActor(context.Background(), actor)

	require.NoError(t, engine.WriteTuple(context.Background(), actor.UserKey(), "member", OrganizationScope(orgID)))

	assert.NoError(t, engine.Allowed(ctx, Read, OrganizationScope(orgID)))
}

func TestOrgMemberCanOnlyRead(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	actor := NewActor(ActorAdmin, "u1", WithOrganizationID(orgID))
	engine, ctx := TestSetup(t, context.Background(), actor, "member", "")

	scope := OrganizationScope(orgID)
	assert.NoError(t, engine.Allowed(ctx, Read, scope))
	assert.Error(t, engine.Allowed(ctx, Create, scope))
	assert.Error(t, engine.Allowed(ctx, Update, scope))
	assert.Error(t, engine.Allowed(ctx, Delete, scope))
}

func TestOrgAdminCanCreateAndUpdate(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	actor := NewActor(ActorAdmin, "u1", WithOrganizationID(orgID))
	engine, ctx := TestSetup(t, context.Background(), actor, "admin", "")

	scope := OrganizationScope(orgID)
	assert.NoError(t, engine.Allowed(ctx, Read, scope))
	assert.NoError(t, engine.Allowed(ctx, Create, scope))
	assert.NoError(t, engine.Allowed(ctx, Update, scope))
	assert.Error(t, engine.Allowed(ctx, Delete, scope))
}

func TestOrgOwnerCanDelete(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	actor := NewActor(ActorAdmin, "u1", WithOrganizationID(orgID))
	engine, ctx := TestSetup(t, context.Background(), actor, "owner", "")

	scope := OrganizationScope(orgID)
	assert.NoError(t, engine.Allowed(ctx, Read, scope))
	assert.NoError(t, engine.Allowed(ctx, Create, scope))
	assert.NoError(t, engine.Allowed(ctx, Update, scope))
	assert.NoError(t, engine.Allowed(ctx, Delete, scope))
}

func TestProjectSupportCanOnlyRead(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()
	actor := NewActor(ActorAPIKey, "k1",
		WithOrganizationID(orgID),
		WithProjectID(projectID),
	)
	engine, ctx := TestSetup(t, context.Background(), actor, "member", "support")

	scope := ProjectResourceScope("users", projectID)
	assert.NoError(t, engine.Allowed(ctx, Read, scope))
	assert.Error(t, engine.Allowed(ctx, Create, scope))
	assert.Error(t, engine.Allowed(ctx, Update, scope))
	assert.Error(t, engine.Allowed(ctx, Delete, scope))
}

func TestProjectClientCanCreateAndUpdateClientResources(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()
	actor := NewActor(ActorAPIKey, "k1",
		WithOrganizationID(orgID),
		WithProjectID(projectID),
	)
	engine, ctx := TestSetup(t, context.Background(), actor, "member", "client")

	// Client role can create and update users, events, and organizations
	// but has no read access.
	for _, resource := range []string{"users", "events", "organizations"} {
		scope := ProjectResourceScope(resource, projectID)
		assert.Error(t, engine.Allowed(ctx, Read, scope), "read %s", resource)
		assert.NoError(t, engine.Allowed(ctx, Create, scope), "create %s", resource)
		assert.NoError(t, engine.Allowed(ctx, Update, scope), "update %s", resource)
	}

	// Client role can also delete users and organizations.
	for _, resource := range []string{"users", "organizations"} {
		scope := ProjectResourceScope(resource, projectID)
		assert.NoError(t, engine.Allowed(ctx, Delete, scope), "delete %s", resource)
	}
}

func TestProjectClientCannotReadAnyResource(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()
	actor := NewActor(ActorAPIKey, "k1",
		WithOrganizationID(orgID),
		WithProjectID(projectID),
	)
	engine, ctx := TestSetup(t, context.Background(), actor, "member", "client")

	// Client role has no read access to any resource.
	for _, resource := range Resources() {
		scope := ProjectResourceScope(resource, projectID)
		assert.Error(t, engine.Allowed(ctx, Read, scope), "read %s", resource)
	}
}

func TestProjectClientCannotMutateNonClientResources(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()
	actor := NewActor(ActorAPIKey, "k1",
		WithOrganizationID(orgID),
		WithProjectID(projectID),
	)
	engine, ctx := TestSetup(t, context.Background(), actor, "member", "client")

	// Client role cannot read, create, update, or delete resources that
	// require editor or admin (e.g. campaigns, templates, providers).
	for _, resource := range []string{"campaigns", "templates", "journeys", "lists", "tags", "documents", "locales", "providers"} {
		scope := ProjectResourceScope(resource, projectID)
		assert.Error(t, engine.Allowed(ctx, Read, scope), "read %s", resource)
		assert.Error(t, engine.Allowed(ctx, Create, scope), "create %s", resource)
		assert.Error(t, engine.Allowed(ctx, Update, scope), "update %s", resource)
		assert.Error(t, engine.Allowed(ctx, Delete, scope), "delete %s", resource)
	}
}

func TestProjectClientCannotDeleteEvents(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()
	actor := NewActor(ActorAPIKey, "k1",
		WithOrganizationID(orgID),
		WithProjectID(projectID),
	)
	engine, ctx := TestSetup(t, context.Background(), actor, "member", "client")

	// Client role cannot delete events (delete requires editor).
	scope := ProjectResourceScope("events", projectID)
	assert.Error(t, engine.Allowed(ctx, Delete, scope), "delete events")
}

func TestProjectEditorCanCRUDStandardResources(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()
	actor := NewActor(ActorAPIKey, "k1",
		WithOrganizationID(orgID),
		WithProjectID(projectID),
	)
	engine, ctx := TestSetup(t, context.Background(), actor, "member", "editor")

	for _, resource := range []string{"users", "campaigns", "templates", "journeys"} {
		scope := ProjectResourceScope(resource, projectID)
		assert.NoError(t, engine.Allowed(ctx, Read, scope), "read %s", resource)
		assert.NoError(t, engine.Allowed(ctx, Create, scope), "create %s", resource)
		assert.NoError(t, engine.Allowed(ctx, Update, scope), "update %s", resource)
		assert.NoError(t, engine.Allowed(ctx, Delete, scope), "delete %s", resource)
	}
}

func TestProjectAdminCanCRUDAllResources(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()
	actor := NewActor(ActorAPIKey, "k1",
		WithOrganizationID(orgID),
		WithProjectID(projectID),
	)
	engine, ctx := TestSetup(t, context.Background(), actor, "member", "admin")

	for _, resource := range Resources() {
		scope := ProjectResourceScope(resource, projectID)
		assert.NoError(t, engine.Allowed(ctx, Read, scope), "read %s", resource)
		assert.NoError(t, engine.Allowed(ctx, Create, scope), "create %s", resource)
		assert.NoError(t, engine.Allowed(ctx, Update, scope), "update %s", resource)
		assert.NoError(t, engine.Allowed(ctx, Delete, scope), "delete %s", resource)
	}
}

func TestEditorCannotMutateProviders(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()
	actor := NewActor(ActorAPIKey, "k1",
		WithOrganizationID(orgID),
		WithProjectID(projectID),
	)
	engine, ctx := TestSetup(t, context.Background(), actor, "member", "editor")

	scope := ProjectResourceScope("providers", projectID)
	assert.NoError(t, engine.Allowed(ctx, Read, scope))
	assert.Error(t, engine.Allowed(ctx, Create, scope))
	assert.Error(t, engine.Allowed(ctx, Update, scope))
	assert.Error(t, engine.Allowed(ctx, Delete, scope))
}

func TestEditorCannotDeleteSubscriptions(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()
	actor := NewActor(ActorAPIKey, "k1",
		WithOrganizationID(orgID),
		WithProjectID(projectID),
	)
	engine, ctx := TestSetup(t, context.Background(), actor, "member", "editor")

	scope := ProjectResourceScope("subscriptions", projectID)
	assert.NoError(t, engine.Allowed(ctx, Read, scope))
	assert.NoError(t, engine.Allowed(ctx, Create, scope))
	assert.NoError(t, engine.Allowed(ctx, Update, scope))
	assert.Error(t, engine.Allowed(ctx, Delete, scope))
}

func TestOrgOwnerInheritsProjectAdmin(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()
	actor := NewActor(ActorAdmin, "u1",
		WithOrganizationID(orgID),
		WithProjectID(projectID),
	)

	// Give org "owner" role but no explicit project role.
	engine, ctx := TestSetup(t, context.Background(), actor, "owner", "")

	// Manually link project → org and resources → project (TestSetup skips
	// these when projectRole is empty).
	bg := context.Background()
	projectObj := ProjectScope(projectID)
	orgObj := OrganizationScope(orgID)

	require.NoError(t, engine.WriteTuple(bg, orgObj, "organization", projectObj))
	for _, resource := range Resources() {
		require.NoError(t, engine.WriteTuple(bg, projectObj, "project", resource+":"+projectID.String()))
	}

	// Org owner should have full admin on project resources.
	scope := ProjectResourceScope("users", projectID)
	assert.NoError(t, engine.Allowed(ctx, Read, scope))
	assert.NoError(t, engine.Allowed(ctx, Create, scope))
	assert.NoError(t, engine.Allowed(ctx, Update, scope))
	assert.NoError(t, engine.Allowed(ctx, Delete, scope))
}

func TestProjectRolesDoNotLeakAcrossProjects(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	project1 := uuid.New()
	project2 := uuid.New()
	actor := NewActor(ActorAPIKey, "k1",
		WithOrganizationID(orgID),
		WithProjectID(project1),
	)

	engine, ctx := TestSetup(t, context.Background(), actor, "member", "admin")

	// Actor has admin on project1 but no tuples for project2.
	assert.NoError(t, engine.Allowed(ctx, Read, ProjectResourceScope("users", project1)))
	assert.Error(t, engine.Allowed(ctx, Read, ProjectResourceScope("users", project2)))
}

func TestSetupWithTuplesGrantsCustomAccess(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	actor := NewActor(ActorAdmin, "u1", WithOrganizationID(orgID))
	orgObj := OrganizationScope(orgID)

	tuples := []Tuple{
		{User: actor.UserKey(), Relation: "owner", Object: orgObj},
	}

	engine, ctx := TestSetupWithTuples(t, context.Background(), actor, tuples)

	assert.NoError(t, engine.Allowed(ctx, Delete, OrganizationScope(orgID)))
}

func TestDeleteTupleIfPresentIsIdempotent(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	ctx := context.Background()

	projectID := uuid.New()
	user := "user:" + uuid.New().String()
	object := ProjectScope(projectID)

	require.NoError(t, engine.WriteTuple(ctx, user, ProjectEditor, object))
	require.NoError(t, engine.DeleteTupleIfPresent(ctx, user, ProjectEditor, object))
	// Revoking access that is already gone satisfies the caller's intent and
	// must not fail, or one stale record blocks a whole revocation cascade.
	require.NoError(t, engine.DeleteTupleIfPresent(ctx, user, ProjectEditor, object))

	allowed, err := engine.Check(ctx, user, ProjectEditor, object)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestProjectRoleResolvesHighestHeldRole(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	ctx := context.Background()
	orgID := uuid.New()
	projectID := uuid.New()
	object := ProjectScope(projectID)

	require.NoError(t, engine.WriteTuple(ctx, OrganizationScope(orgID), "organization", object))

	for role, want := range map[string]string{
		ProjectSupport: ProjectSupport,
		ProjectClient:  ProjectClient,
		ProjectEditor:  ProjectEditor,
		ProjectAdmin:   ProjectAdmin,
	} {
		user := "user:" + uuid.New().String()
		require.NoError(t, engine.WriteTuple(ctx, user, role, object))

		got, err := engine.ProjectRole(ctx, user, projectID)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}

	// An organization owner is a project admin by inheritance, with no direct
	// project grant of their own.
	inheritor := "user:" + uuid.New().String()
	require.NoError(t, engine.WriteTuple(ctx, inheritor, "owner", OrganizationScope(orgID)))
	got, err := engine.ProjectRole(ctx, inheritor, projectID)
	require.NoError(t, err)
	assert.Equal(t, ProjectAdmin, got)

	// A subject carrying only a direct grant on a resource object holds no
	// project role, so it ranks below every role in a ceiling comparison.
	scoped := "user:" + uuid.New().String()
	require.NoError(t, engine.WriteTuple(ctx, ProjectScope(projectID), "project", "members:"+projectID.String()))
	require.NoError(t, engine.WriteTuple(ctx, scoped, "update", "members:"+projectID.String()))
	got, err = engine.ProjectRole(ctx, scoped, projectID)
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

// TestWriteTuplesIfAbsentIsIdempotent covers the batch form: a batch containing
// a tuple that already exists must still write the rest, because OpenFGA writes
// are all-or-nothing and would otherwise reject the whole request.
func TestWriteTuplesIfAbsentIsIdempotent(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	ctx := context.Background()
	obj := OrganizationScope(uuid.New())

	first := []Tuple{{User: "user:batch-1", Relation: "member", Object: obj}}
	require.NoError(t, engine.WriteTuplesIfAbsent(ctx, first))
	require.NoError(t, engine.WriteTuplesIfAbsent(ctx, first), "re-writing the same tuple must succeed")

	// A batch that mixes an existing tuple with a new one must leave both present.
	mixed := []Tuple{
		{User: "user:batch-1", Relation: "member", Object: obj},
		{User: "user:batch-2", Relation: "member", Object: obj},
	}
	require.NoError(t, engine.WriteTuplesIfAbsent(ctx, mixed))

	for _, user := range []string{"user:batch-1", "user:batch-2"} {
		allowed, err := engine.Check(ctx, user, "member", obj)
		require.NoError(t, err)
		assert.True(t, allowed, "%s should have been granted", user)
	}
}

// TestWriteTuplesIfAbsentSurfacesRealErrors pins that duplicate tolerance is not
// blanket error tolerance.
func TestWriteTuplesIfAbsentSurfacesRealErrors(t *testing.T) {
	t.Parallel()

	engine := NewTestEngine(t)
	ctx := context.Background()
	obj := OrganizationScope(uuid.New())

	err := engine.WriteTuplesIfAbsent(ctx, []Tuple{
		{User: "user:u1", Relation: "not_a_real_relation", Object: obj},
	})
	require.Error(t, err)
}

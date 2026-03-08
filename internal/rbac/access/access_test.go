package access

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationRoleTuples(t *testing.T) {
	t.Parallel()

	adminID := uuid.New()
	orgID := uuid.New()

	type test struct {
		role     string
		expected []rbac.Tuple
	}

	tests := map[string]test{
		"owner": {
			role: "owner",
			expected: []rbac.Tuple{
				{User: "user:" + adminID.String(), Relation: "owner", Object: "organization:" + orgID.String()},
			},
		},
		"admin": {
			role: "admin",
			expected: []rbac.Tuple{
				{User: "user:" + adminID.String(), Relation: "admin", Object: "organization:" + orgID.String()},
			},
		},
		"member": {
			role: "member",
			expected: []rbac.Tuple{
				{User: "user:" + adminID.String(), Relation: "member", Object: "organization:" + orgID.String()},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tuples := OrganizationRoleTuples(adminID, orgID, tc.role)
			assert.Equal(t, tc.expected, tuples)
		})
	}
}

func TestProjectTuples(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()

	tuples := ProjectTuples(orgID, projectID)

	// First tuple must link the project to its organization.
	require.NotEmpty(t, tuples)
	assert.Equal(t, rbac.Tuple{
		User:     "organization:" + orgID.String(),
		Relation: "organization",
		Object:   rbac.ProjectScope(projectID),
	}, tuples[0])

	// Remaining tuples must link every resource type to the project.
	resourceNames := rbac.Resources()
	require.Len(t, tuples, 1+len(resourceNames), "expected 1 org tuple + %d resource tuples", len(resourceNames))

	projectObject := rbac.ProjectScope(projectID)
	for i, resource := range resourceNames {
		expected := rbac.Tuple{
			User:     projectObject,
			Relation: "project",
			Object:   resource + ":" + projectID.String(),
		}
		assert.Equal(t, expected, tuples[1+i], "resource tuple %d (%s)", i, resource)
	}
}

func TestProjectTuplesContainsAllResourceTypes(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()
	tuples := ProjectTuples(orgID, projectID)

	// Collect the resource names referenced in the tuples.
	found := make(map[string]bool)
	for _, tuple := range tuples[1:] {
		// Object is "<resource>:<project-id>", extract the resource name.
		assert.Contains(t, tuple.Object, ":")
		resource := tuple.Object[:len(tuple.Object)-len(":"+projectID.String())]
		found[resource] = true
	}

	for _, name := range rbac.Resources() {
		assert.True(t, found[name], "missing resource type %q in project tuples", name)
	}
}

func TestProvisionProject(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()

	adminID := uuid.New()
	orgID := uuid.New()
	projectID := uuid.New()

	// Grant admin the owner role on the org.
	require.NoError(t, engine.WriteTuples(ctx, OrganizationRoleTuples(adminID, orgID, "owner")))

	// Provision the project.
	err := ProvisionProject(ctx, engine, orgID, projectID)
	require.NoError(t, err)

	// The org owner should now inherit project admin and be able to read all
	// resource types through the tuple-to-userset chain.
	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	actorCtx := rbac.WithActor(ctx, actor)

	for _, resource := range rbac.Resources() {
		assert.NoError(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope(resource, projectID)),
			"org owner should be able to read %s", resource)
	}
}

func TestProvisionProjectResourcePermissions(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()

	adminID := uuid.New()
	orgID := uuid.New()
	projectID := uuid.New()

	require.NoError(t, engine.WriteTuples(ctx, OrganizationRoleTuples(adminID, orgID, "owner")))
	require.NoError(t, ProvisionProject(ctx, engine, orgID, projectID))

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	actorCtx := rbac.WithActor(ctx, actor)

	// Org owner inherits project admin, which should have full CRUD on all resources.
	for _, resource := range rbac.Resources() {
		assert.NoError(t, engine.Allowed(actorCtx, rbac.Create, rbac.ProjectResourceScope(resource, projectID)),
			"create %s", resource)
		assert.NoError(t, engine.Allowed(actorCtx, rbac.Update, rbac.ProjectResourceScope(resource, projectID)),
			"update %s", resource)
		assert.NoError(t, engine.Allowed(actorCtx, rbac.Delete, rbac.ProjectResourceScope(resource, projectID)),
			"delete %s", resource)
	}
}

func TestDeprovisionProject(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()

	adminID := uuid.New()
	orgID := uuid.New()
	projectID := uuid.New()

	require.NoError(t, engine.WriteTuples(ctx, OrganizationRoleTuples(adminID, orgID, "owner")))
	require.NoError(t, ProvisionProject(ctx, engine, orgID, projectID))

	// Deprovision.
	err := DeprovisionProject(ctx, engine, orgID, projectID)
	require.NoError(t, err)

	// Permissions should no longer resolve.
	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	actorCtx := rbac.WithActor(ctx, actor)

	for _, resource := range rbac.Resources() {
		assert.Error(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope(resource, projectID)),
			"read %s should fail after deprovision", resource)
	}
}

func TestProvisionAndDeprovisionAreInverse(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()

	adminID := uuid.New()
	orgID := uuid.New()
	projectID := uuid.New()

	require.NoError(t, engine.WriteTuples(ctx, OrganizationRoleTuples(adminID, orgID, "owner")))

	// Provision then deprovision: should return to the initial state.
	require.NoError(t, ProvisionProject(ctx, engine, orgID, projectID))
	require.NoError(t, DeprovisionProject(ctx, engine, orgID, projectID))

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	actorCtx := rbac.WithActor(ctx, actor)

	// All resource checks should fail.
	for _, resource := range rbac.Resources() {
		assert.Error(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope(resource, projectID)))
	}
}

func TestProjectsDoNotLeakAcrossOrganizations(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()

	adminA := uuid.New()
	orgA := uuid.New()
	adminB := uuid.New()
	orgB := uuid.New()
	projectA := uuid.New()

	require.NoError(t, engine.WriteTuples(ctx, OrganizationRoleTuples(adminA, orgA, "owner")))
	require.NoError(t, engine.WriteTuples(ctx, OrganizationRoleTuples(adminB, orgB, "owner")))
	require.NoError(t, ProvisionProject(ctx, engine, orgA, projectA))

	// Admin B (different org) should NOT have access to project A's resources.
	actorB := rbac.NewActor(rbac.ActorAdmin, adminB.String(),
		rbac.WithOrganizationID(orgB),
		rbac.WithProjectID(projectA),
	)
	actorBCtx := rbac.WithActor(ctx, actorB)

	for _, resource := range rbac.Resources() {
		assert.Error(t, engine.Allowed(actorBCtx, rbac.Read, rbac.ProjectResourceScope(resource, projectA)),
			"admin from org B should not access org A's project resource %s", resource)
	}
}

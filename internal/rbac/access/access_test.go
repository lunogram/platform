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

func TestApiKeyRoleTuples(t *testing.T) {
	t.Parallel()

	keyID := uuid.New()
	projectID := uuid.New()

	type test struct {
		role     string
		expected []rbac.Tuple
	}

	tests := map[string]test{
		"support": {
			role: "support",
			expected: []rbac.Tuple{
				{User: "user:" + keyID.String(), Relation: "support", Object: "project:" + projectID.String()},
			},
		},
		"client": {
			role: "client",
			expected: []rbac.Tuple{
				{User: "user:" + keyID.String(), Relation: "client", Object: "project:" + projectID.String()},
			},
		},
		"editor": {
			role: "editor",
			expected: []rbac.Tuple{
				{User: "user:" + keyID.String(), Relation: "editor", Object: "project:" + projectID.String()},
			},
		},
		"admin": {
			role: "admin",
			expected: []rbac.Tuple{
				{User: "user:" + keyID.String(), Relation: "admin", Object: "project:" + projectID.String()},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tuples := ApiKeyRoleTuples(keyID, projectID, tc.role)
			assert.Equal(t, tc.expected, tuples)
		})
	}
}

func TestProvisionApiKey(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()

	orgID := uuid.New()
	projectID := uuid.New()
	keyID := uuid.New()

	// Provision the project first so resource tuples exist.
	require.NoError(t, engine.WriteTuples(ctx, ProjectTuples(orgID, projectID)))

	// Provision the API key with "client" role.
	err := ProvisionApiKey(ctx, engine, keyID, projectID, "client")
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAPIKey, keyID.String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	actorCtx := rbac.WithActor(ctx, actor)

	// "client" has no read access to any resource.
	for _, resource := range rbac.Resources() {
		assert.Error(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope(resource, projectID)),
			"client key should not be able to read %s", resource)
	}

	// "client" should be able to create and update client-level resources.
	for _, resource := range []string{"users", "events", "organizations"} {
		assert.NoError(t, engine.Allowed(actorCtx, rbac.Create, rbac.ProjectResourceScope(resource, projectID)),
			"client key should be able to create %s", resource)
		assert.NoError(t, engine.Allowed(actorCtx, rbac.Update, rbac.ProjectResourceScope(resource, projectID)),
			"client key should be able to update %s", resource)
	}

	// "client" should NOT be able to create "campaigns" (requires editor).
	assert.Error(t, engine.Allowed(actorCtx, rbac.Create, rbac.ProjectResourceScope("campaigns", projectID)))
}

func TestDeprovisionApiKey(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()

	orgID := uuid.New()
	projectID := uuid.New()
	keyID := uuid.New()

	require.NoError(t, engine.WriteTuples(ctx, ProjectTuples(orgID, projectID)))
	require.NoError(t, ProvisionApiKey(ctx, engine, keyID, projectID, "editor"))

	actor := rbac.NewActor(rbac.ActorAPIKey, keyID.String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	actorCtx := rbac.WithActor(ctx, actor)

	// Verify access works before deprovisioning.
	assert.NoError(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope("campaigns", projectID)))

	// Deprovision.
	err := DeprovisionApiKey(ctx, engine, keyID, projectID, "editor")
	require.NoError(t, err)

	// Permissions should no longer resolve.
	for _, resource := range rbac.Resources() {
		assert.Error(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope(resource, projectID)),
			"read %s should fail after deprovision", resource)
	}
}

func TestUpdateApiKeyRole(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()

	orgID := uuid.New()
	projectID := uuid.New()
	keyID := uuid.New()

	require.NoError(t, engine.WriteTuples(ctx, ProjectTuples(orgID, projectID)))
	require.NoError(t, ProvisionApiKey(ctx, engine, keyID, projectID, "support"))

	actor := rbac.NewActor(rbac.ActorAPIKey, keyID.String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	actorCtx := rbac.WithActor(ctx, actor)

	// "support" can read but not create campaigns.
	assert.NoError(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope("campaigns", projectID)))
	assert.Error(t, engine.Allowed(actorCtx, rbac.Create, rbac.ProjectResourceScope("campaigns", projectID)))

	// Upgrade to "editor".
	err := UpdateApiKeyRole(ctx, engine, keyID, projectID, "support", "editor")
	require.NoError(t, err)

	// Now create campaigns should work.
	assert.NoError(t, engine.Allowed(actorCtx, rbac.Create, rbac.ProjectResourceScope("campaigns", projectID)))
}

func TestUpdateApiKeyRoleSameRoleIsNoop(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()

	orgID := uuid.New()
	projectID := uuid.New()
	keyID := uuid.New()

	require.NoError(t, engine.WriteTuples(ctx, ProjectTuples(orgID, projectID)))
	require.NoError(t, ProvisionApiKey(ctx, engine, keyID, projectID, "editor"))

	actor := rbac.NewActor(rbac.ActorAPIKey, keyID.String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	actorCtx := rbac.WithActor(ctx, actor)

	// Update with same role should be a no-op and not break anything.
	err := UpdateApiKeyRole(ctx, engine, keyID, projectID, "editor", "editor")
	require.NoError(t, err)

	// Permissions should still work.
	assert.NoError(t, engine.Allowed(actorCtx, rbac.Create, rbac.ProjectResourceScope("campaigns", projectID)))
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

func TestProjectRoleLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	orgID := uuid.New()
	projectID := uuid.New()

	members := rbac.ProjectResourceScope("members", projectID)

	t.Run("provision grants the role and is idempotent", func(t *testing.T) {
		engine := rbac.NewTestEngine(t)
		adminID := uuid.New()
		require.NoError(t, engine.WriteTuples(ctx, ProjectTuples(orgID, projectID)))

		require.NoError(t, ProvisionProjectRole(ctx, engine, adminID, projectID, rbac.ProjectSupport))
		// Re-running must not fail: the tuple write happens after the Postgres
		// commit, so a provisioning step is legitimately replayed to reconcile.
		require.NoError(t, ProvisionProjectRole(ctx, engine, adminID, projectID, rbac.ProjectSupport))

		allowed, err := engine.Check(ctx, "user:"+adminID.String(), string(rbac.Read), members)
		require.NoError(t, err)
		assert.True(t, allowed, "support may read the member roster")

		allowed, err = engine.Check(ctx, "user:"+adminID.String(), string(rbac.Update), members)
		require.NoError(t, err)
		assert.False(t, allowed, "support may not change roles")
	})

	t.Run("update swaps the role", func(t *testing.T) {
		engine := rbac.NewTestEngine(t)
		adminID := uuid.New()
		require.NoError(t, engine.WriteTuples(ctx, ProjectTuples(orgID, projectID)))
		require.NoError(t, ProvisionProjectRole(ctx, engine, adminID, projectID, rbac.ProjectSupport))

		require.NoError(t, UpdateProjectRole(ctx, engine, adminID, projectID, rbac.ProjectSupport, rbac.ProjectAdmin))
		allowed, err := engine.Check(ctx, "user:"+adminID.String(), string(rbac.Update), members)
		require.NoError(t, err)
		assert.True(t, allowed, "promotion must take effect")

		require.NoError(t, UpdateProjectRole(ctx, engine, adminID, projectID, rbac.ProjectAdmin, rbac.ProjectSupport))
		allowed, err = engine.Check(ctx, "user:"+adminID.String(), string(rbac.Update), members)
		require.NoError(t, err)
		assert.False(t, allowed, "demotion must withdraw the privilege")
	})

	t.Run("deprovision revokes access", func(t *testing.T) {
		engine := rbac.NewTestEngine(t)
		adminID := uuid.New()
		require.NoError(t, engine.WriteTuples(ctx, ProjectTuples(orgID, projectID)))
		require.NoError(t, ProvisionProjectRole(ctx, engine, adminID, projectID, rbac.ProjectEditor))

		require.NoError(t, DeprovisionProjectRole(ctx, engine, adminID, projectID, rbac.ProjectEditor))
		allowed, err := engine.Check(ctx, "user:"+adminID.String(), string(rbac.Read), members)
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("bulk deprovision revokes grants across projects", func(t *testing.T) {
		engine := rbac.NewTestEngine(t)
		adminID := uuid.New()
		other := uuid.New()
		require.NoError(t, engine.WriteTuples(ctx, ProjectTuples(orgID, projectID)))
		require.NoError(t, engine.WriteTuples(ctx, ProjectTuples(orgID, other)))
		require.NoError(t, ProvisionProjectRole(ctx, engine, adminID, projectID, rbac.ProjectEditor))
		require.NoError(t, ProvisionProjectRole(ctx, engine, adminID, other, rbac.ProjectAdmin))

		require.NoError(t, DeprovisionProjectRoles(ctx, engine, []ProjectRoleGrant{
			{UserID: adminID, ProjectID: projectID, Role: rbac.ProjectEditor},
			{UserID: adminID, ProjectID: other, Role: rbac.ProjectAdmin},
		}))

		for _, id := range []uuid.UUID{projectID, other} {
			allowed, err := engine.Check(ctx, "user:"+adminID.String(), string(rbac.Read), rbac.ProjectResourceScope("members", id))
			require.NoError(t, err)
			assert.False(t, allowed, "every grant in the batch must be revoked")
		}
	})
}

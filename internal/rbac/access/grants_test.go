package access

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyGrantTuples(t *testing.T) {
	t.Parallel()

	policyID := uuid.New()
	projectID := uuid.New()

	grants := []Grant{
		{Resource: "inbox", Verb: rbac.Read},
		{Resource: "users", Verb: rbac.Read},
	}

	expected := []rbac.Tuple{
		{User: "user:" + policyID.String(), Relation: "read", Object: "inbox:" + projectID.String()},
		{User: "user:" + policyID.String(), Relation: "read", Object: "users:" + projectID.String()},
	}

	assert.Equal(t, expected, PolicyGrantTuples(policyID, projectID, grants))
}

func TestPolicyGrantTuplesEmpty(t *testing.T) {
	t.Parallel()
	assert.Empty(t, PolicyGrantTuples(uuid.New(), uuid.New(), nil))
}

// TestProvisionPolicyGrants verifies that a custom permission set resolves
// through the direct-grant path independently of the four role presets: only
// the exact (resource, verb) pairs granted are allowed, and deprovisioning
// removes them.
func TestProvisionPolicyGrants(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()

	orgID := uuid.New()
	projectID := uuid.New()
	policyID := uuid.New()

	// A custom read-only scope on inbox + users only — no role preset, and no
	// project resource tuples written: the grants must resolve on their own.
	grants := []Grant{
		{Resource: "inbox", Verb: rbac.Read},
		{Resource: "users", Verb: rbac.Read},
	}
	require.NoError(t, ProvisionPolicyGrants(ctx, engine, policyID, projectID, grants))

	actor := rbac.NewActor(rbac.ActorAPIKey, policyID.String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	actorCtx := rbac.WithActor(ctx, actor)

	// Granted: read on inbox and users.
	assert.NoError(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope("inbox", projectID)))
	assert.NoError(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope("users", projectID)))

	// Not granted: write verbs on the same resources, or read on others.
	assert.Error(t, engine.Allowed(actorCtx, rbac.Create, rbac.ProjectResourceScope("inbox", projectID)))
	assert.Error(t, engine.Allowed(actorCtx, rbac.Update, rbac.ProjectResourceScope("users", projectID)))
	assert.Error(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope("campaigns", projectID)))

	// Deprovisioning removes the grants.
	require.NoError(t, DeprovisionPolicyGrants(ctx, engine, policyID, projectID, grants))
	assert.Error(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope("inbox", projectID)))
	assert.Error(t, engine.Allowed(actorCtx, rbac.Read, rbac.ProjectResourceScope("users", projectID)))
}

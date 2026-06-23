// Package access provides domain-level RBAC provisioning operations on top of
// the low-level [rbac.Engine]. While the engine deals in raw tuples and checks,
// this package understands the Lunogram domain model (organizations, projects,
// resource types) and exposes high-level functions for building and writing the
// relationship tuples that wire up the authorization model.
//
// All tuple-building helpers are exported as pure functions so they can be
// tested independently of the engine.
package access

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/rbac"
	"go.uber.org/zap"
)

// OrganizationRoleTuples returns the tuples needed to grant the given role to
// an admin within an organization.
func OrganizationRoleTuples(adminID, organizationID uuid.UUID, role string) []rbac.Tuple {
	return []rbac.Tuple{
		{
			User:     "user:" + adminID.String(),
			Relation: role,
			Object:   rbac.OrganizationScope(organizationID),
		},
	}
}

// ApiKeyRoleTuples returns the tuple that grants a project-level role to an
// API key. The keyID is the API key's UUID (used as the RBAC user identity)
// and role must be one of "support", "client", "editor", or "admin".
func ApiKeyRoleTuples(keyID, projectID uuid.UUID, role string) []rbac.Tuple {
	return []rbac.Tuple{
		{
			User:     "user:" + keyID.String(),
			Relation: role,
			Object:   rbac.ProjectScope(projectID),
		},
	}
}

// ProvisionApiKey writes the RBAC tuple that grants the given project role to
// an API key. This must be called whenever an API key is created so that
// project-scoped permission checks resolve correctly for requests
// authenticated with that key.
func ProvisionApiKey(ctx context.Context, engine *rbac.Engine, keyID, projectID uuid.UUID, role string) error {
	tuples := ApiKeyRoleTuples(keyID, projectID, role)
	if err := engine.WriteTuples(ctx, tuples); err != nil {
		return fmt.Errorf("access: failed to provision API key %s: %w", keyID, err)
	}
	return nil
}

// DeprovisionApiKey removes the RBAC tuple that was created by
// [ProvisionApiKey]. Call this when an API key is deleted to clean up the
// authorization store.
func DeprovisionApiKey(ctx context.Context, engine *rbac.Engine, keyID, projectID uuid.UUID, role string) error {
	tuples := ApiKeyRoleTuples(keyID, projectID, role)
	if err := engine.DeleteTuples(ctx, tuples); err != nil {
		return fmt.Errorf("access: failed to deprovision API key %s: %w", keyID, err)
	}
	return nil
}

// UpdateApiKeyRole removes the old role tuple and writes a new one for an API
// key. This must be called whenever the role of an existing API key changes.
func UpdateApiKeyRole(ctx context.Context, engine *rbac.Engine, keyID, projectID uuid.UUID, oldRole, newRole string) error {
	if oldRole == newRole {
		return nil
	}
	if err := DeprovisionApiKey(ctx, engine, keyID, projectID, oldRole); err != nil {
		return err
	}
	return ProvisionApiKey(ctx, engine, keyID, projectID, newRole)
}

// Grant is a single (resource, verb) entry in a custom permission set. It maps
// to one direct relation tuple written onto the project-scoped resource object.
//
//	{Resource: "inbox", Verb: rbac.Read}  →  user:<policy-id> read inbox:<project-id>
type Grant struct {
	Resource string
	Verb     rbac.Permission
}

// PolicyGrantTuples returns the tuples that grant an access policy a custom
// permission set. Each grant becomes a direct relation tuple on the
// project-scoped resource object. The policyID is the access policy's UUID,
// used as the RBAC user identity so that checks via [rbac.Actor.UserKey]
// resolve against these grants.
//
// Use this for policies that carry an explicit scope rather than one of the
// four role presets (see [ApiKeyRoleTuples]).
func PolicyGrantTuples(policyID, projectID uuid.UUID, grants []Grant) []rbac.Tuple {
	user := "user:" + policyID.String()
	tuples := make([]rbac.Tuple, 0, len(grants))
	for _, g := range grants {
		tuples = append(tuples, rbac.Tuple{
			User:     user,
			Relation: string(g.Verb),
			Object:   rbac.ProjectResourceScope(g.Resource, projectID),
		})
	}
	return tuples
}

// ProvisionPolicyGrants writes the custom permission-set tuples for an access
// policy. Call this when a policy with a custom scope is created. It is a no-op
// when grants is empty.
func ProvisionPolicyGrants(ctx context.Context, engine *rbac.Engine, policyID, projectID uuid.UUID, grants []Grant) error {
	if err := engine.WriteTuples(ctx, PolicyGrantTuples(policyID, projectID, grants)); err != nil {
		return fmt.Errorf("access: failed to provision grants for policy %s: %w", policyID, err)
	}
	return nil
}

// DeprovisionPolicyGrants removes the custom permission-set tuples created by
// [ProvisionPolicyGrants]. Call this when a policy is deleted, or before
// rewriting its scope. It is a no-op when grants is empty.
func DeprovisionPolicyGrants(ctx context.Context, engine *rbac.Engine, policyID, projectID uuid.UUID, grants []Grant) error {
	if err := engine.DeleteTuples(ctx, PolicyGrantTuples(policyID, projectID, grants)); err != nil {
		return fmt.Errorf("access: failed to deprovision grants for policy %s: %w", policyID, err)
	}
	return nil
}

// ProjectTuples returns the tuples that link a project to its parent
// organization and connect every resource type to the project. These tuples
// are required for project-scoped permission checks to resolve correctly
// through the OpenFGA tuple-to-userset chain.
//
// The returned tuples are:
//
//   - organization:<orgID> → organization → project:<projectID>
//   - project:<projectID>  → project      → <resource>:<projectID>  (for every resource type)
func ProjectTuples(organizationID, projectID uuid.UUID) []rbac.Tuple {
	orgObject := rbac.OrganizationScope(organizationID)
	projectObject := rbac.ProjectScope(projectID)

	tuples := []rbac.Tuple{
		{User: orgObject, Relation: "organization", Object: projectObject},
	}

	for _, resource := range rbac.Resources() {
		tuples = append(tuples, rbac.Tuple{
			User:     projectObject,
			Relation: "project",
			Object:   resource + ":" + projectID.String(),
		})
	}

	return tuples
}

// ProvisionProject writes the RBAC tuples that link a newly created project
// to its parent organization and connect every resource type to the project.
// This must be called whenever a project is created so that project-scoped
// permission checks (e.g. rbac.ProjectResourceScope("providers", projectID)) resolve
// correctly.
func ProvisionProject(ctx context.Context, engine *rbac.Engine, organizationID, projectID uuid.UUID) error {
	tuples := ProjectTuples(organizationID, projectID)
	if err := engine.WriteTuples(ctx, tuples); err != nil {
		return fmt.Errorf("access: failed to provision project %s: %w", projectID, err)
	}
	return nil
}

// DeprovisionProject removes the RBAC tuples that were created by
// [ProvisionProject]. Call this when a project is deleted to clean up the
// authorization store.
func DeprovisionProject(ctx context.Context, engine *rbac.Engine, organizationID, projectID uuid.UUID) error {
	tuples := ProjectTuples(organizationID, projectID)
	if err := engine.DeleteTuples(ctx, tuples); err != nil {
		return fmt.Errorf("access: failed to deprovision project %s: %w", projectID, err)
	}
	return nil
}

// BackfillProjectTuples re-provisions RBAC resource tuples for all existing
// projects. This must be called when the authorization model changes (e.g. a
// new resource type is added) to ensure that existing projects have the
// relationship tuples needed for permission checks on the new resource type.
//
// The function queries the database for all (organization_id, id) pairs in
// the projects table and writes each resource tuple individually. Tuples that
// already exist produce an error from OpenFGA which is silently skipped,
// making this operation idempotent.
func BackfillProjectTuples(ctx context.Context, logger *zap.Logger, engine *rbac.Engine, db *sqlx.DB) error {
	type row struct {
		OrganizationID uuid.UUID `db:"organization_id"`
		ID             uuid.UUID `db:"id"`
	}

	var projects []row
	if err := db.SelectContext(ctx, &projects, "SELECT organization_id, id FROM projects"); err != nil {
		return fmt.Errorf("access: failed to list projects for backfill: %w", err)
	}

	logger.Info("backfilling RBAC resource tuples for existing projects", zap.Int("count", len(projects)))

	resources := rbac.Resources()
	var failures int
	for _, p := range projects {
		projectObject := rbac.ProjectScope(p.ID)
		for _, resource := range resources {
			object := resource + ":" + p.ID.String()
			// WriteTupleIfAbsent treats an already-exists duplicate as success
			// (the idempotent case), so any error here is a real failure — a
			// datastore or validation problem that would otherwise leave the
			// project without a tuple and silently fail-closed on later checks.
			if err := engine.WriteTupleIfAbsent(ctx, projectObject, "project", object); err != nil {
				failures++
				logger.Warn("failed to backfill RBAC resource tuple",
					zap.String("object", object), zap.Error(err))
			}
		}
	}

	if failures > 0 {
		logger.Warn("RBAC resource tuple backfill completed with failures", zap.Int("failures", failures))
	} else {
		logger.Info("RBAC resource tuple backfill complete")
	}
	return nil
}

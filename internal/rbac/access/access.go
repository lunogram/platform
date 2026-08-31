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

// revoke removes tuples, treating one that is already absent as revoked. The
// organization- and project-role revocations go through it because they are
// replayed: the ordering rule is that a tuple is removed BEFORE the record that
// justified it, so a failure part-way leaves the record in place and the retry
// re-attempts deletes that may already have succeeded. A strict delete would
// turn that retry into a permanent failure.
//
// The API-key and policy-grant paths still use a strict delete; they have not
// been moved onto this ordering.
func revoke(ctx context.Context, engine *rbac.Engine, tuples []rbac.Tuple) error {
	var firstErr error
	for _, t := range tuples {
		if err := engine.DeleteTupleIfPresent(ctx, t.User, t.Relation, t.Object); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

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

// ProjectRoleTuples returns the tuples needed to grant the given role to an
// admin within a project.
func ProjectRoleTuples(userID, projectID uuid.UUID, role string) []rbac.Tuple {
	return []rbac.Tuple{
		{
			User:     "user:" + userID.String(),
			Relation: role,
			Object:   rbac.ProjectScope(projectID),
		},
	}
}

// DeprovisionOrganizationRole removes the organization role tuple granted by
// [ProvisionMembership]. Call it when a membership is revoked, before the
// membership row itself is removed.
func DeprovisionOrganizationRole(ctx context.Context, engine *rbac.Engine, adminID, organizationID uuid.UUID, role string) error {
	if err := revoke(ctx, engine, OrganizationRoleTuples(adminID, organizationID, role)); err != nil {
		return fmt.Errorf("access: failed to deprovision organization role for admin %s: %w", adminID, err)
	}
	return nil
}

// ProjectRoleGrant pairs an RBAC subject with the project role it holds. It
// exists so a set of grants spanning several projects (revoking one admin from
// every project of an organization) or several admins (deleting a project) can
// be revoked in a single batch.
type ProjectRoleGrant struct {
	UserID    uuid.UUID
	ProjectID uuid.UUID
	Role      string
}

// ProjectRoleGrantTuples returns the tuples for a set of project role grants.
func ProjectRoleGrantTuples(grants []ProjectRoleGrant) []rbac.Tuple {
	tuples := make([]rbac.Tuple, 0, len(grants))
	for _, g := range grants {
		tuples = append(tuples, ProjectRoleTuples(g.UserID, g.ProjectID, g.Role)...)
	}
	return tuples
}

// ProvisionProjectRole writes the RBAC tuple that grants an admin the given
// role in a project. It must be called whenever a project_admins row is created
// so that project-scoped permission checks resolve for that admin; a row
// without its tuple is a member the authorization model cannot see.
//
// The write is idempotent: OpenFGA rejects re-writing an existing tuple, and
// because the tuple write happens after the Postgres commit (the two stores
// cannot share a transaction) a provisioning step may legitimately be re-run to
// reconcile a membership whose tuples were never written.
//
// It must be [rbac.Engine.WriteTuplesIfAbsent], which decides presence from the
// write, and never a Check-then-write: Check resolves through the model, so an
// organization owner already looks like a project admin by inheritance and the
// direct grant would never be written — leaving them to lose the project the
// moment their organization role changes, which is the very defect this exists
// to close.
func ProvisionProjectRole(ctx context.Context, engine *rbac.Engine, adminID, projectID uuid.UUID, role string) error {
	tuples := ProjectRoleTuples(adminID, projectID, role)
	if err := engine.WriteTuplesIfAbsent(ctx, tuples); err != nil {
		return fmt.Errorf("access: failed to provision project role for admin %s: %w", adminID, err)
	}
	return nil
}

// DeprovisionProjectRole removes the RBAC tuple written by
// [ProvisionProjectRole]. Call this whenever a project_admins row is removed:
// the row is only the record of membership, the tuple is the access itself, so
// a delete that skips this leaves the removed member with working API access.
func DeprovisionProjectRole(ctx context.Context, engine *rbac.Engine, adminID, projectID uuid.UUID, role string) error {
	tuples := ProjectRoleTuples(adminID, projectID, role)
	if err := revoke(ctx, engine, tuples); err != nil {
		return fmt.Errorf("access: failed to deprovision project role for admin %s: %w", adminID, err)
	}
	return nil
}

// DeprovisionProjectRoles revokes several project role grants at once. Use it
// when a single event invalidates many grants (an admin removed from an
// organization, or a project deleted).
//
// Each grant is revoked independently and a grant whose tuple is already gone
// counts as revoked, so one stale record cannot block the revocation of the
// others. The first real failure is returned after every grant has been
// attempted — a revocation cascade must get as far as it can and still report
// that it did not finish.
func DeprovisionProjectRoles(ctx context.Context, engine *rbac.Engine, grants []ProjectRoleGrant) error {
	if err := revoke(ctx, engine, ProjectRoleGrantTuples(grants)); err != nil {
		return fmt.Errorf("access: failed to deprovision %d project role grants: %w", len(grants), err)
	}
	return nil
}

// SetProjectRole makes role the only project role the admin holds on the
// project, revoking every other one first.
//
// Use it where the caller knows the role the membership should end up with but
// not the role it currently has — reviving a membership that was removed
// earlier, most of all. A soft-deleted project_admins row is invisible to every
// read, so the accept flow cannot tell a returning member from a new one and has
// no old role to hand [UpdateProjectRole]. Adding the new grant on its own would
// leave whatever the previous membership wrote sitting alongside it, and the
// subject would hold the union of the two: a demotion that never took effect, or
// a promotion that authorization does not honour because the check resolves the
// stale tuple instead.
//
// Revoking runs before the grant for the same reason it does in
// [UpdateProjectRole]: failing halfway must leave less access, never more.
func SetProjectRole(ctx context.Context, engine *rbac.Engine, adminID, projectID uuid.UUID, role string) error {
	for _, other := range rbac.ProjectRoles() {
		if other == role {
			continue
		}
		if err := DeprovisionProjectRole(ctx, engine, adminID, projectID, other); err != nil {
			return err
		}
	}
	return ProvisionProjectRole(ctx, engine, adminID, projectID, role)
}

// UpdateProjectRole removes the old role tuple and writes the new one. The old
// tuple is removed FIRST: a promotion that fails halfway must not leave the
// admin holding both roles, and on a demotion the stale higher-privilege tuple
// is the one that must not survive.
//
// Use this where the role change is recorded before the tuples are touched (the
// invite-accept flow, whose Postgres work is already committed by then). Where
// the record is written by the same handler, split the two halves around it
// instead — see UpdateProjectAdmin — so the revoke precedes the write and the
// grant follows it.
func UpdateProjectRole(ctx context.Context, engine *rbac.Engine, adminID, projectID uuid.UUID, oldRole, newRole string) error {
	if oldRole == newRole {
		return nil
	}
	if oldRole != "" {
		if err := DeprovisionProjectRole(ctx, engine, adminID, projectID, oldRole); err != nil {
			return err
		}
	}
	return ProvisionProjectRole(ctx, engine, adminID, projectID, newRole)
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
	if err := revoke(ctx, engine, tuples); err != nil {
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

// BackfillProjectRoleTuples writes the per-admin project role tuples for every
// active project_admins row. It repairs the divergence left by paths that
// recorded a project role in Postgres without writing the matching tuple
// (project creation, historically): such a member holds a role on paper that
// the authorization model cannot see, and only reaches the project at all while
// some other grant — usually organization owner/admin inheritance — happens to
// cover them.
//
// Writes are idempotent, so this is safe to run on every start alongside
// [BackfillProjectTuples]. It only ever grants what the database already says
// the admin has; it never invents a grant.
func BackfillProjectRoleTuples(ctx context.Context, logger *zap.Logger, engine *rbac.Engine, db *sqlx.DB) error {
	type row struct {
		ProjectID uuid.UUID `db:"project_id"`
		AdminID   uuid.UUID `db:"admin_id"`
		Role      string    `db:"role"`
	}

	query := `
	SELECT pa.project_id, pa.admin_id, pa.role
	FROM project_admins pa
	JOIN projects p ON p.id = pa.project_id AND p.deleted_at IS NULL
	WHERE pa.deleted_at IS NULL`

	var grants []row
	if err := db.SelectContext(ctx, &grants, query); err != nil {
		return fmt.Errorf("access: failed to list project admins for backfill: %w", err)
	}

	logger.Info("backfilling RBAC project role tuples", zap.Int("count", len(grants)))

	var failures int
	for _, g := range grants {
		// An unranked role is not a relation in the authorization model; writing
		// it would fail anyway, and skipping it keeps one bad row from failing
		// the whole backfill.
		if !rbac.IsKnownProjectRole(g.Role) {
			failures++
			logger.Warn("skipping project role tuple with unknown role",
				zap.String("project_id", g.ProjectID.String()), zap.String("role", g.Role))
			continue
		}
		if err := engine.WriteTupleIfAbsent(ctx, "user:"+g.AdminID.String(), g.Role, rbac.ProjectScope(g.ProjectID)); err != nil {
			failures++
			logger.Warn("failed to backfill RBAC project role tuple",
				zap.String("project_id", g.ProjectID.String()), zap.Error(err))
		}
	}

	if failures > 0 {
		logger.Warn("RBAC project role tuple backfill completed with failures", zap.Int("failures", failures))
	} else {
		logger.Info("RBAC project role tuple backfill complete")
	}
	return nil
}

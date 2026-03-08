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
	"github.com/lunogram/platform/internal/rbac"
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

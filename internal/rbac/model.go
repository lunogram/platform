package rbac

import (
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
)

// direct represents a directly assigned relation.
var direct = &openfgav1.Userset{Userset: &openfgav1.Userset_This{}}

// typeRef returns a RelationReference for a bare type (e.g. "user").
func typeRef(typeName string) *openfgav1.RelationReference {
	return &openfgav1.RelationReference{Type: typeName}
}

// directlyRelated builds a RelationMetadata that lists the given type
// references as allowed direct assignments.
func directlyRelated(refs ...*openfgav1.RelationReference) *openfgav1.RelationMetadata {
	return &openfgav1.RelationMetadata{DirectlyRelatedUserTypes: refs}
}

// comp creates a computed userset that references another relation on the same object.
func comp(relation string) *openfgav1.Userset {
	return &openfgav1.Userset{
		Userset: &openfgav1.Userset_ComputedUserset{
			ComputedUserset: &openfgav1.ObjectRelation{Relation: relation},
		},
	}
}

// ttu creates a tuple-to-userset that follows a tupleset relation and then
// evaluates a computed relation on the target object.
func ttu(tupleset, computedRelation string) *openfgav1.Userset {
	return &openfgav1.Userset{
		Userset: &openfgav1.Userset_TupleToUserset{
			TupleToUserset: &openfgav1.TupleToUserset{
				Tupleset:        &openfgav1.ObjectRelation{Relation: tupleset},
				ComputedUserset: &openfgav1.ObjectRelation{Relation: computedRelation},
			},
		},
	}
}

// union creates a userset that is the union of the given children.
func union(children ...*openfgav1.Userset) *openfgav1.Userset {
	return &openfgav1.Userset{
		Userset: &openfgav1.Userset_Union{
			Union: &openfgav1.Usersets{Child: children},
		},
	}
}

// Model returns the OpenFGA type definitions that describe the full RBAC model
// for Lunogram. The model is built from three layers:
//
// # Types
//
//   - user:         Represents an authenticated identity. Has no relations of its own.
//   - organization: Groups users under org-level roles and CRUD permissions.
//   - project:      Belongs to an organization and defines a four-level role hierarchy.
//   - <resource>:   Each project resource (users, campaigns, etc.) is its own type
//     with read/create/update/delete relations resolved through the
//     parent project's role hierarchy.
//
// # Organization roles
//
// Roles form a hierarchy where each level includes all privileges of the levels
// below it:
//
//	member < admin < owner
//
// CRUD permissions on the organization itself are computed from these roles:
//
//	read:   member
//	create: admin
//	update: admin
//	delete: owner
//
// # Project roles
//
// Projects define a four-level role hierarchy. The "client" and "support" roles
// are independent branches under "editor":
//
//	          ┌─ client  (write-only: create/update)
//	editor ──┤
//	          └─ support (read-only)
//	editor < admin
//
// The "client" role is designed for API keys that power the client-facing API.
// It grants create and update access to the resources used by client endpoints
// (users, events, organizations) but has no read access to any resource. This
// ensures client API keys can be used safely in public contexts (e.g. websites)
// without exposing data.
//
// The "support" role grants read-only access to all resources.
//
// Organization owners and admins automatically inherit the project admin role
// through a tuple-to-userset rewrite on the "organization" relation.
//
// # Resource types
//
// Each resource (users, campaigns, journeys, etc.) is its own OpenFGA type with
// a "project" parent relation. The CRUD relations resolve through the parent
// project's roles:
//
//	read:   resolved from the project role required for read access (e.g. support)
//	create: resolved from the project role required for create access (e.g. editor)
//	update: resolved from the project role required for update access (e.g. editor)
//	delete: resolved from the project role required for delete access (e.g. admin)
//
// This follows the Google Zanzibar pattern where each resource type inherits
// permissions from its parent container.
//
// # Custom permission sets
//
// Each resource CRUD relation is the union of a direct assignment and the
// project-role path:
//
//	read: this OR project.<role-required-for-read>
//
// The project-role path expresses the four role presets (support/client/editor/
// admin). The direct assignment lets an access policy be granted an explicit,
// arbitrary scope by writing a tuple straight onto the resource, e.g.
//
//	user:<policy-id> → read → inbox:<project-id>
//
// This is how access policies carry a custom permission set (resource + verb
// pairs) without being pinned to one of the four presets. See the access
// package for the tuple-building helpers.
func Model() []*openfgav1.TypeDefinition {
	types := []*openfgav1.TypeDefinition{
		{Type: "user"},
		{
			Type: "organization",
			Relations: map[string]*openfgav1.Userset{
				"member": union(direct, comp("admin")),
				"admin":  union(direct, comp("owner")),
				"owner":  direct,

				"read":   comp("member"),
				"create": comp("admin"),
				"update": comp("admin"),
				"delete": comp("owner"),
			},
			Metadata: &openfgav1.Metadata{
				Relations: map[string]*openfgav1.RelationMetadata{
					"member": directlyRelated(typeRef("user")),
					"admin":  directlyRelated(typeRef("user")),
					"owner":  directlyRelated(typeRef("user")),
				},
			},
		},
		{
			Type: "project",
			Relations: map[string]*openfgav1.Userset{
				"organization": direct,

				"admin": union(
					direct,
					ttu("organization", "owner"),
					ttu("organization", "admin"),
				),
				"editor":  union(direct, comp("admin")),
				"client":  union(direct, comp("editor")),
				"support": union(direct, comp("editor")),
			},
			Metadata: &openfgav1.Metadata{
				Relations: map[string]*openfgav1.RelationMetadata{
					"organization": directlyRelated(typeRef("organization")),
					"admin":        directlyRelated(typeRef("user")),
					"editor":       directlyRelated(typeRef("user")),
					"client":       directlyRelated(typeRef("user")),
					"support":      directlyRelated(typeRef("user")),
				},
			},
		},
	}

	type resource struct {
		name   string
		read   string // project role required for read
		create string // project role required for create
		update string // project role required for update
		delete string // project role required for delete
	}

	resources := []resource{
		{name: "users", read: "support", create: "client", update: "client", delete: "client"},
		{name: "events", read: "support", create: "client", update: "client", delete: "editor"},
		{name: "campaigns", read: "support", create: "editor", update: "editor", delete: "editor"},
		{name: "templates", read: "support", create: "editor", update: "editor", delete: "editor"},
		{name: "journeys", read: "support", create: "editor", update: "editor", delete: "editor"},
		{name: "lists", read: "support", create: "editor", update: "editor", delete: "editor"},
		{name: "tags", read: "support", create: "editor", update: "editor", delete: "editor"},
		{name: "documents", read: "support", create: "editor", update: "editor", delete: "editor"},
		{name: "locales", read: "support", create: "editor", update: "editor", delete: "editor"},
		{name: "providers", read: "support", create: "admin", update: "admin", delete: "admin"},
		{name: "push_providers", read: "support", create: "admin", update: "admin", delete: "admin"},
		{name: "subscriptions", read: "support", create: "editor", update: "editor", delete: "admin"},
		{name: "actions", read: "support", create: "editor", update: "editor", delete: "admin"},
		{name: "organizations", read: "support", create: "client", update: "client", delete: "client"},
		{name: "sender_identities", read: "support", create: "admin", update: "admin", delete: "admin"},
		{name: "broadcasts", read: "support", create: "editor", update: "editor", delete: "editor"},
		{name: "scheduled", read: "support", create: "client", update: "client", delete: "editor"},
		{name: "devices", read: "support", create: "client", update: "client", delete: "client"},
		{name: "invites", read: "admin", create: "admin", update: "admin", delete: "admin"},
		{name: "inbox", read: "support", create: "client", update: "client", delete: "admin"},
	}

	for _, r := range resources {
		types = append(types, &openfgav1.TypeDefinition{
			Type: r.name,
			Relations: map[string]*openfgav1.Userset{
				"project": direct,

				// Each CRUD relation is the union of a direct grant (a custom
				// per-policy scope written straight onto the resource) and the
				// project-role preset path.
				"read":   union(direct, ttu("project", r.read)),
				"create": union(direct, ttu("project", r.create)),
				"update": union(direct, ttu("project", r.update)),
				"delete": union(direct, ttu("project", r.delete)),
			},
			Metadata: &openfgav1.Metadata{
				Relations: map[string]*openfgav1.RelationMetadata{
					"project": directlyRelated(typeRef("project")),
					"read":    directlyRelated(typeRef("user")),
					"create":  directlyRelated(typeRef("user")),
					"update":  directlyRelated(typeRef("user")),
					"delete":  directlyRelated(typeRef("user")),
				},
			},
		})
	}

	return types
}

// Resources returns the names of all resource types defined in the
// authorization model (i.e. types that have a "project" parent relation).
// This is the single source of truth for resource names; test helpers should
// use this instead of maintaining a separate list.
func Resources() []string {
	var names []string
	for _, td := range Model() {
		if _, ok := td.GetRelations()["project"]; ok && td.GetType() != "project" {
			names = append(names, td.GetType())
		}
	}
	return names
}

// Resource is a typed resource name from the authorization model. Its values are
// the strings returned by [Resources]; the constants below cover the resources
// whose create grants can be narrowed by a per-grant constraint.
type Resource string

const (
	ResourceEvents        Resource = "events"
	ResourceSubscriptions Resource = "subscriptions"
	ResourceScheduled     Resource = "scheduled"
)

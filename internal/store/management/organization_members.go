package management

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
)

func NewOrganizationMembersStore(db store.DB) *OrganizationMembersStore {
	return &OrganizationMembersStore{db: db}
}

type OrganizationMembersStore struct {
	db store.DB
}

type OrganizationMember struct {
	ID             uuid.UUID  `db:"id"`
	OrganizationID uuid.UUID  `db:"organization_id"`
	AdminID        uuid.UUID  `db:"admin_id"`
	Role           string     `db:"role"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

// AdminOrganization is an organization an admin belongs to, with the admin's
// role in it. Used to render the organization switcher.
type AdminOrganization struct {
	ID   uuid.UUID `db:"id"`
	Name string    `db:"name"`
	Role string    `db:"role"`
}

// AddMember adds the admin to the organization, or revives/updates the role of
// a previously removed membership. Idempotent.
func (s *OrganizationMembersStore) AddMember(ctx context.Context, organizationID, adminID uuid.UUID, role string) error {
	stmt := `
	INSERT INTO organization_members (organization_id, admin_id, role)
	VALUES ($1, $2, $3)
	ON CONFLICT (organization_id, admin_id) DO UPDATE SET
		role = EXCLUDED.role,
		deleted_at = NULL,
		updated_at = NOW()`

	_, err := s.db.ExecContext(ctx, stmt, organizationID, adminID, role)
	return err
}

// RemoveMember soft-deletes a membership.
func (s *OrganizationMembersStore) RemoveMember(ctx context.Context, organizationID, adminID uuid.UUID) error {
	stmt := `
	UPDATE organization_members
	SET deleted_at = NOW()
	WHERE organization_id = $1 AND admin_id = $2 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, organizationID, adminID)
	return err
}

// ReconcileAdminOrganizations repoints an admin's home organization_id and
// active_organization_id away from organizationID, which the admin no longer
// belongs to. It is meant to run after RemoveMember (in the same transaction)
// so correctness does not rely solely on resolveActiveOrganization's read-time
// fallback: a stale active_organization_id is cleared, and a home organization
// that points at the removed org is re-pointed to a remaining membership.
//
// active_organization_id is set NULL when it referenced the removed org (the
// column is nullable and the session falls back to the home org). The home
// organization_id is NOT NULL, so it is only re-pointed when another active
// membership exists; if the removed org was the admin's only membership the
// home pointer is left as-is (the admin has no org to belong to and read-time
// fallback still applies).
func (s *OrganizationMembersStore) ReconcileAdminOrganizations(ctx context.Context, organizationID, adminID uuid.UUID) error {
	// Clear a dangling active organization first; this never fails for lack of a
	// replacement because the column is nullable.
	clearActive := `
	UPDATE admins
	SET active_organization_id = NULL
	WHERE id = $1 AND active_organization_id = $2 AND deleted_at IS NULL`
	if _, err := s.db.ExecContext(ctx, clearActive, adminID, organizationID); err != nil {
		return err
	}

	// Re-point the home organization only when the removed org was the home org
	// and the admin still has at least one other active membership to fall back
	// to. organization_id is NOT NULL, so we never null it out.
	repointHome := `
	UPDATE admins
	SET organization_id = (
		SELECT om.organization_id
		FROM organization_members om
		JOIN organizations o ON o.id = om.organization_id AND o.deleted_at IS NULL
		WHERE om.admin_id = $1 AND om.organization_id <> $2 AND om.deleted_at IS NULL
		ORDER BY om.created_at ASC
		LIMIT 1
	)
	WHERE id = $1
	AND organization_id = $2
	AND deleted_at IS NULL
	AND EXISTS (
		SELECT 1
		FROM organization_members om
		WHERE om.admin_id = $1 AND om.organization_id <> $2 AND om.deleted_at IS NULL
	)`
	if _, err := s.db.ExecContext(ctx, repointHome, adminID, organizationID); err != nil {
		return err
	}

	return nil
}

// GetMember returns the admin's active membership in the organization, or
// sql.ErrNoRows if there is none.
func (s *OrganizationMembersStore) GetMember(ctx context.Context, organizationID, adminID uuid.UUID) (*OrganizationMember, error) {
	stmt := `
	SELECT id, organization_id, admin_id, role, created_at, updated_at, deleted_at
	FROM organization_members
	WHERE organization_id = $1 AND admin_id = $2 AND deleted_at IS NULL`

	var member OrganizationMember
	err := s.db.GetContext(ctx, &member, stmt, organizationID, adminID)
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// IsMember reports whether the admin is an active member of the organization.
func (s *OrganizationMembersStore) IsMember(ctx context.Context, organizationID, adminID uuid.UUID) (bool, error) {
	stmt := `
	SELECT EXISTS (
		SELECT 1 FROM organization_members
		WHERE organization_id = $1 AND admin_id = $2 AND deleted_at IS NULL
	)`

	var exists bool
	err := s.db.GetContext(ctx, &exists, stmt, organizationID, adminID)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// ListOrganizationsForAdmin returns the organizations the admin is a member of,
// ordered by name.
func (s *OrganizationMembersStore) ListOrganizationsForAdmin(ctx context.Context, adminID uuid.UUID) ([]AdminOrganization, error) {
	stmt := `
	SELECT o.id, o.name, om.role
	FROM organization_members om
	JOIN organizations o ON o.id = om.organization_id AND o.deleted_at IS NULL
	WHERE om.admin_id = $1 AND om.deleted_at IS NULL
	ORDER BY o.name ASC`

	var orgs []AdminOrganization
	err := s.db.SelectContext(ctx, &orgs, stmt, adminID)
	if err != nil {
		return nil, err
	}
	return orgs, nil
}

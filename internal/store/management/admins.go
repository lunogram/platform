package management

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

// ErrAmbiguousEmail is returned when an email address does not resolve to a
// single, unambiguous admin. The duplicate-email migration quarantines the
// surplus rows rather than merging them, because two rows sharing an address may
// be two real people; an address touched by that reconciliation stays unresolvable
// until an operator says which account it belongs to. Every path that would act
// on "the admin with this email" fails closed here instead of guessing.
var ErrAmbiguousEmail = errors.New("management: email does not resolve to a single admin")

// AmbiguousEmailError carries every live admin on the address so the caller can
// log exactly which accounts need reconciling. It matches [ErrAmbiguousEmail]
// under errors.Is.
type AmbiguousEmailError struct {
	Email    string
	AdminIDs []uuid.UUID
}

func (e *AmbiguousEmailError) Error() string {
	if len(e.AdminIDs) == 1 {
		// The other side of the collision has since been deleted, but the
		// survivor is still quarantined and so is still not the address's
		// confirmed owner.
		return fmt.Sprintf("management: email %q belongs to an admin quarantined by duplicate-email reconciliation", e.Email)
	}
	return fmt.Sprintf("management: email %q resolves to %d live admins", e.Email, len(e.AdminIDs))
}

func (e *AmbiguousEmailError) Unwrap() error { return ErrAmbiguousEmail }

// NewAdminsStore builds the admin store. sessions may be nil, in which case
// deleting an admin does not end the sessions they hold.
func NewAdminsStore(db store.DB, sessions *AdminSessionsStore) *AdminsStore {
	return &AdminsStore{db: db, sessions: sessions}
}

type AdminsStore struct {
	db store.DB
	// sessions is held so a write that must end a session can do it from inside
	// the store. Leaving that to callers makes it something a future path can
	// forget, and forgetting means a deleted admin keeps a working console.
	sessions *AdminSessionsStore
}

type Admin struct {
	ID                   uuid.UUID  `db:"id"`
	OrganizationID       uuid.UUID  `db:"organization_id"`
	ActiveOrganizationID *uuid.UUID `db:"active_organization_id"`
	Email                string     `db:"email"`
	FirstName            *string    `db:"first_name"`
	LastName             *string    `db:"last_name"`
	ImageURL             *string    `db:"image_url"`
	Role                 string     `db:"role"`
	CreatedAt            time.Time  `db:"created_at"`
	UpdatedAt            time.Time  `db:"updated_at"`
}

func (admin *Admin) OAPI() oapi.Admin {
	return oapi.Admin{
		Id:             admin.ID,
		OrganizationId: admin.OrganizationID,
		Email:          admin.Email,
		FirstName:      admin.FirstName,
		LastName:       admin.LastName,
		ImageUrl:       admin.ImageURL,
		Role:           oapi.OrganizationRole(admin.Role),
		CreatedAt:      admin.CreatedAt,
		UpdatedAt:      admin.UpdatedAt,
	}
}

func (s *AdminsStore) GetAdmin(ctx context.Context, id uuid.UUID) (*Admin, error) {
	stmt := `
	SELECT id, organization_id, active_organization_id, email, first_name, last_name, image_url, role, created_at, updated_at
	FROM admins
	WHERE id = $1
	AND deleted_at IS NULL`

	var admin Admin
	err := s.db.GetContext(ctx, &admin, stmt, id)
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

func (s *AdminsStore) CreateAdmin(ctx context.Context, admin Admin) (uuid.UUID, error) {
	// A newly created admin's active organization defaults to its home
	// organization; the switcher can change it later.
	stmt := `
	INSERT INTO admins (organization_id, active_organization_id, email, first_name, last_name, image_url, role)
	VALUES ($1, $1, $2, $3, $4, $5, $6)
	RETURNING id
	`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		admin.OrganizationID,
		admin.Email,
		admin.FirstName,
		admin.LastName,
		admin.ImageURL,
		admin.Role,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *AdminsStore) ListAdmins(ctx context.Context, organizationID uuid.UUID, pagination store.Pagination, search string) ([]Admin, int, error) {
	var admins []Admin
	var total int

	// Lists the members of the organization. Membership is the source of truth
	// (an admin can belong to several organizations), so this joins
	// organization_members rather than filtering admins.organization_id.
	query := `
	SELECT
		a.id, a.organization_id, a.active_organization_id, a.email, a.first_name, a.last_name, a.image_url, a.role, a.created_at, a.updated_at,
		COUNT(*) OVER () AS total_count
	FROM admins a
	JOIN organization_members om ON om.admin_id = a.id AND om.organization_id = $1 AND om.deleted_at IS NULL
	WHERE a.deleted_at IS NULL
	AND (
		$2 = '' OR
		a.first_name ILIKE '%' || $2 || '%' OR
		a.last_name ILIKE '%' || $2 || '%' OR
		a.email ILIKE '%' || $2 || '%'
	)
	ORDER BY a.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		Admin
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, organizationID, search, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []Admin{}, 0, nil
	}

	total = results[0].TotalCount
	for _, r := range results {
		admins = append(admins, r.Admin)
	}

	return admins, total, nil
}

// GetAdminByEmail looks up an admin by email, case-insensitively. Email
// matching must be case-insensitive everywhere it affects authorization (e.g.
// the cross-organization invite guard and login provisioning); an exact match
// would let an uppercase-stored email silently bypass those checks.
//
// Quarantined admins (email_conflict_at set) are excluded. They keep their real
// address and keep authenticating through admin_identities; it is only the email
// JOIN -- the part that would have to guess which of several accounts an address
// means -- that skips them. Callers that link an identity onto an address must
// use [AdminsStore.ResolveAdminByEmail], which reports the ambiguity instead of
// silently returning the surviving row.
func (s *AdminsStore) GetAdminByEmail(ctx context.Context, email string) (*Admin, error) {
	stmt := `
	SELECT id, organization_id, active_organization_id, email, first_name, last_name, image_url, role, created_at, updated_at
	FROM admins
	WHERE lower(email) = lower($1)
	AND deleted_at IS NULL
	AND email_conflict_at IS NULL`

	var admin Admin
	err := s.db.GetContext(ctx, &admin, stmt, email)
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

// ResolveAdminByEmail resolves the single admin that owns an address, for the
// paths that use email as an identity key rather than as a display field.
//
// It returns [ErrAmbiguousEmail] (as an [AmbiguousEmailError] naming every
// colliding admin) when the address is shared by more than one live admin, even
// if exactly one of them is canonical. That is the whole point: a quarantined
// duplicate means nobody can tell which of the two accounts the address
// identifies, so binding a new identity to either is a guess -- and guessing
// wrong hands one person's console to another. Failing closed leaves an operator
// to reconcile the accounts. sql.ErrNoRows means the address is unclaimed.
func (s *AdminsStore) ResolveAdminByEmail(ctx context.Context, email string) (*Admin, error) {
	stmt := `
	SELECT id, organization_id, active_organization_id, email, first_name, last_name, image_url, role, created_at, updated_at,
	       email_conflict_at IS NOT NULL AS quarantined
	FROM admins
	WHERE lower(email) = lower($1)
	AND deleted_at IS NULL
	ORDER BY email_conflict_at NULLS FIRST, created_at ASC, id ASC`

	type row struct {
		Admin
		Quarantined bool `db:"quarantined"`
	}

	var rows []row
	if err := s.db.SelectContext(ctx, &rows, stmt, email); err != nil {
		return nil, err
	}

	switch {
	case len(rows) == 0:
		return nil, store.ErrNoRows
	case len(rows) == 1 && !rows[0].Quarantined:
		admin := rows[0].Admin
		return &admin, nil
	}

	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return nil, &AmbiguousEmailError{Email: email, AdminIDs: ids}
}

type AdminUpdate struct {
	Email     *string
	FirstName *string
	LastName  *string
	Role      *string
}

func (s *AdminsStore) UpdateAdmin(ctx context.Context, id uuid.UUID, update AdminUpdate) error {
	stmt := `
	UPDATE admins
	SET
		email = COALESCE($2, email),
		first_name = COALESCE($3, first_name),
		last_name = COALESCE($4, last_name),
		role = COALESCE($5, role)
	WHERE id = $1
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, id, update.Email, update.FirstName, update.LastName, update.Role)
	return err
}

// DeleteAdmin soft-deletes an admin and ends every session they hold. Revoking
// the sessions is not housekeeping: without it a deleted admin keeps a working
// console until their token expires, because the middleware resolves a session
// by id and never re-checks that the admin still exists.
func (s *AdminsStore) DeleteAdmin(ctx context.Context, id uuid.UUID) error {
	stmt := `UPDATE admins SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	if _, err := s.db.ExecContext(ctx, stmt, id); err != nil {
		return err
	}

	if s.sessions != nil {
		_, err := s.sessions.RevokeAdminSessionsForAdmin(ctx, id)
		return err
	}
	return nil
}

// SetActiveOrganization updates the admin's active organization, the one that
// scopes their session. Callers must verify the admin is a member of the
// organization first.
//
// The active organization is deliberately NOT part of the cached session: it
// lives here, on the admin, and is re-read and re-validated against current
// membership on every request. Nothing cached changes here, so there is nothing
// to invalidate.
func (s *AdminsStore) SetActiveOrganization(ctx context.Context, adminID, organizationID uuid.UUID) error {
	stmt := `UPDATE admins SET active_organization_id = $2 WHERE id = $1 AND deleted_at IS NULL`
	_, err := s.db.ExecContext(ctx, stmt, adminID, organizationID)
	return err
}

// Project Admin methods

type ProjectAdmin struct {
	ID        uuid.UUID  `db:"id"`
	ProjectID uuid.UUID  `db:"project_id"`
	AdminID   uuid.UUID  `db:"admin_id"`
	Role      string     `db:"role"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
	Email     *string    `db:"email"`
	FirstName *string    `db:"first_name"`
	LastName  *string    `db:"last_name"`
}

func (pa *ProjectAdmin) OAPI() oapi.ProjectAdmin {
	result := oapi.ProjectAdmin{
		Id:        pa.ID,
		ProjectId: pa.ProjectID,
		AdminId:   pa.AdminID,
		Role:      oapi.ProjectRole(pa.Role),
		CreatedAt: pa.CreatedAt,
		UpdatedAt: pa.UpdatedAt,
	}

	if pa.Email != nil {
		result.Email = pa.Email
	}
	if pa.FirstName != nil {
		result.FirstName = pa.FirstName
	}
	if pa.LastName != nil {
		result.LastName = pa.LastName
	}

	return result
}

type ProjectAdmins []ProjectAdmin

func (pas ProjectAdmins) OAPI() []oapi.ProjectAdmin {
	results := make([]oapi.ProjectAdmin, len(pas))
	for i, pa := range pas {
		results[i] = pa.OAPI()
	}
	return results
}

func (s *AdminsStore) ListProjectAdmins(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string) ([]ProjectAdmin, int, error) {
	var projectAdmins []ProjectAdmin
	var total int

	query := `
	SELECT
		project_admins.id,
		project_admins.project_id,
		project_admins.admin_id,
		project_admins.role,
		project_admins.created_at,
		project_admins.updated_at,
		admins.email,
		admins.first_name,
		admins.last_name,
		COUNT(*) OVER () AS total_count
	FROM project_admins
	JOIN admins ON project_admins.admin_id = admins.id
	WHERE project_admins.project_id = $1
	AND project_admins.deleted_at IS NULL
	AND (
		$2 = '' OR
		admins.first_name ILIKE '%' || $2 || '%' OR
		admins.last_name ILIKE '%' || $2 || '%' OR
		admins.email ILIKE '%' || $2 || '%'
	)
	ORDER BY project_admins.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		ProjectAdmin
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, search, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []ProjectAdmin{}, 0, nil
	}

	total = results[0].TotalCount
	for _, r := range results {
		projectAdmins = append(projectAdmins, r.ProjectAdmin)
	}

	return projectAdmins, total, nil
}

func (s *AdminsStore) GetProjectAdmin(ctx context.Context, projectID, adminID uuid.UUID) (*ProjectAdmin, error) {
	query := `
	SELECT
		project_admins.id,
		project_admins.project_id,
		project_admins.admin_id,
		project_admins.role,
		project_admins.created_at,
		project_admins.updated_at,
		admins.email,
		admins.first_name,
		admins.last_name
	FROM project_admins
	JOIN admins ON project_admins.admin_id = admins.id
	WHERE project_admins.project_id = $1
	AND project_admins.admin_id = $2
	AND project_admins.deleted_at IS NULL`

	var projectAdmin ProjectAdmin
	err := s.db.GetContext(ctx, &projectAdmin, query, projectID, adminID)
	if err != nil {
		return nil, err
	}

	return &projectAdmin, nil
}

// AddAdminToProject grants an admin a role on a project, reviving the row of a
// membership that was removed earlier.
//
// Removal is a soft delete, and the table carries a plain UNIQUE on
// (project_id, admin_id) alongside the partial index that ignores deleted rows.
// A removed member therefore leaves a row that every read filters out but that
// an INSERT still collides with, which is what made re-inviting somebody you had
// removed fail on a duplicate key. Reviving is also the right answer on its own
// terms: the membership is identified by the pair, so re-adding is the same
// membership again rather than a second one.
func (s *AdminsStore) AddAdminToProject(ctx context.Context, projectID, adminID uuid.UUID, role string) error {
	query := `
	INSERT INTO project_admins (project_id, admin_id, role)
	VALUES ($1, $2, $3)
	ON CONFLICT (project_id, admin_id)
	DO UPDATE SET role = EXCLUDED.role, deleted_at = NULL`

	_, err := s.db.ExecContext(ctx, query, projectID, adminID, role)
	return err
}

func (s *AdminsStore) UpdateProjectAdminRole(ctx context.Context, projectID, adminID uuid.UUID, role string) error {
	query := `
	UPDATE project_admins
	SET role = $1
	WHERE project_id = $2
	AND admin_id = $3
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, role, projectID, adminID)
	return err
}

func (s *AdminsStore) DeleteProjectAdmin(ctx context.Context, projectID, adminID uuid.UUID) error {
	query := `
	UPDATE project_admins
	SET deleted_at = NOW()
	WHERE project_id = $1
	AND admin_id = $2
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, projectID, adminID)
	return err
}

func (s *AdminsStore) HardDeleteProjectAdmin(ctx context.Context, projectID, adminID uuid.UUID) error {
	query := `
	DELETE FROM project_admins
	WHERE project_id = $1
	AND admin_id = $2`

	_, err := s.db.ExecContext(ctx, query, projectID, adminID)
	return err
}

// registrationBootstrapLock is the advisory-lock key that serialises the "is
// this the first admin?" decision. The value is arbitrary and only has to be
// stable and unique within this database.
const registrationBootstrapLock int64 = 0x6C756E6F67720001

// LockRegistrationBootstrap serialises the first-admin decision against other
// registrations.
//
// It MUST be called on a transaction-scoped store, and it must be called before
// [AdminsStore.AdminsExist] in the same transaction as the admin insert.
// "No admin exists yet" is otherwise a read that two concurrent registrations
// both pass, and both would then create an owner of their own organization on a
// deployment meant to admit exactly one.
//
// pg_advisory_xact_lock rather than a session lock: it is released by the
// commit or rollback, so it cannot leak onto a pooled connection.
func (s *AdminsStore) LockRegistrationBootstrap(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, registrationBootstrapLock)
	return err
}

// AdminsExist reports whether this deployment has any admin at all.
//
// It is what makes an invite-only deployment bootstrappable: the very first
// account cannot have been invited by anybody, so registration is open until
// exactly one admin exists and closed from then on. Callers deciding whether to
// CREATE that first admin must hold [AdminsStore.LockRegistrationBootstrap] and
// be inside the transaction that inserts it.
func (s *AdminsStore) AdminsExist(ctx context.Context) (bool, error) {
	stmt := `SELECT EXISTS (SELECT 1 FROM admins WHERE deleted_at IS NULL)`

	var exists bool
	if err := s.db.GetContext(ctx, &exists, stmt); err != nil {
		return false, err
	}
	return exists, nil
}

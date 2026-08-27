package management

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
	"github.com/oapi-codegen/runtime/types"
)

func NewInvitesStore(db store.DB) *InvitesStore {
	return &InvitesStore{db: db}
}

type InvitesStore struct {
	db store.DB
}

type Invite struct {
	ID                uuid.UUID  `db:"id"`
	ProjectID         uuid.UUID  `db:"project_id"`
	ProjectName       *string    `db:"project_name"`
	InviterAdminID    *uuid.UUID `db:"inviter_admin_id"`
	InviterAdminEmail *string    `db:"inviter_admin_email"`
	InviteeEmail      string     `db:"invitee_email"`
	InviteeAdminID    *uuid.UUID `db:"invitee_admin_id"`
	Role              string     `db:"role"`
	ExpiresAt         time.Time  `db:"expires_at"`
	AcceptedAt        *time.Time `db:"accepted_at"`
	RevokedAt         *time.Time `db:"revoked_at"`
	CreatedAt         time.Time  `db:"created_at"`
}

func (invite *Invite) OAPI() oapi.ProjectInvite {
	role := oapi.ProjectInviteRole(invite.Role)
	inviteeEmail := types.Email(invite.InviteeEmail)

	return oapi.ProjectInvite{
		Id:                &invite.ID,
		ProjectId:         &invite.ProjectID,
		ProjectName:       invite.ProjectName,
		InviterAdminId:    invite.InviterAdminID,
		InviterAdminEmail: invite.InviterAdminEmail,
		InviteeEmail:      &inviteeEmail,
		InviteeAdminId:    invite.InviteeAdminID,
		Role:              &role,
		ExpiresAt:         &invite.ExpiresAt,
		AcceptedAt:        invite.AcceptedAt,
		RevokedAt:         invite.RevokedAt,
		CreatedAt:         &invite.CreatedAt,
	}
}

// inviteColumns is the canonical column list returned by the invite queries,
// joined with the inviter admin and project so the API can render display
// fields. inviter_admin_id is nullable (admins are soft-deleted but the FK is
// ON DELETE SET NULL) so the join is a LEFT JOIN.
const inviteColumns = `
	pi.id, pi.project_id, p.name AS project_name,
	pi.inviter_admin_id, a.email AS inviter_admin_email,
	pi.invitee_email, pi.invitee_admin_id, pi.role,
	pi.expires_at, pi.created_at, pi.revoked_at, pi.accepted_at`

// CreateProjectInvite creates (or refreshes) the single pending invite for a
// project + email. The partial unique index guarantees at most one pending
// invite per (project, lower(email)); re-inviting the same address upserts the
// existing pending row with the new role, inviter and expiry. inviteeAdminID is
// the denormalized id of the admin that currently owns the email, if any.
func (s *InvitesStore) CreateProjectInvite(ctx context.Context, projectID, inviterAdminID uuid.UUID, inviteeEmail string, inviteeAdminID *uuid.UUID, role oapi.CreateProjectInviteRole, ttl time.Duration) (*Invite, error) {
	// The TTL is bound as an integer number of seconds and multiplied by a
	// fixed '1 second'::interval. This keeps the (validated, clamped) duration
	// fully parameterized and avoids casting any free-form user string to an
	// interval, which could otherwise fail at the database layer.
	stmt := `
	INSERT INTO project_invites (project_id, inviter_admin_id, invitee_email, invitee_admin_id, role, expires_at)
	VALUES ($1, $2, $3, $4, $5, NOW() + ($6 * INTERVAL '1 second'))
	ON CONFLICT (project_id, lower(invitee_email)) WHERE accepted_at IS NULL AND revoked_at IS NULL
	DO UPDATE SET
		inviter_admin_id = EXCLUDED.inviter_admin_id,
		invitee_admin_id = EXCLUDED.invitee_admin_id,
		role = EXCLUDED.role,
		expires_at = EXCLUDED.expires_at,
		created_at = NOW()
	RETURNING id, project_id, inviter_admin_id, invitee_email, invitee_admin_id, role, expires_at, created_at, revoked_at, accepted_at`

	var invite Invite
	err := s.db.GetContext(ctx, &invite, stmt, projectID, inviterAdminID, inviteeEmail, inviteeAdminID, role, int64(ttl.Seconds()))
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

// GetInviteByID returns an invite regardless of its status so the caller can
// produce precise errors (expired / revoked / already accepted / wrong account).
func (s *InvitesStore) GetInviteByID(ctx context.Context, id uuid.UUID) (*Invite, error) {
	stmt := `
	SELECT` + inviteColumns + `
	FROM project_invites pi
	LEFT JOIN admins a ON pi.inviter_admin_id = a.id
	LEFT JOIN projects p ON pi.project_id = p.id
	WHERE pi.id = $1`

	var invite Invite
	err := s.db.GetContext(ctx, &invite, stmt, id)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

// AcceptProjectInvite marks a pending invite accepted. It is a no-op (no rows,
// sql.ErrNoRows) if the invite is already accepted, revoked or expired, which
// makes accept idempotent and race-safe under the partial unique index.
func (s *InvitesStore) AcceptProjectInvite(ctx context.Context, id uuid.UUID) (*Invite, error) {
	stmt := `
	UPDATE project_invites
	SET accepted_at = NOW()
	WHERE id = $1 AND revoked_at IS NULL AND accepted_at IS NULL AND expires_at > NOW()
	RETURNING id, project_id, inviter_admin_id, invitee_email, invitee_admin_id, role, expires_at, created_at, revoked_at, accepted_at`

	var invite Invite
	err := s.db.GetContext(ctx, &invite, stmt, id)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

// RevokeProjectInvite marks a pending invite revoked.
func (s *InvitesStore) RevokeProjectInvite(ctx context.Context, id uuid.UUID) (*Invite, error) {
	stmt := `
	UPDATE project_invites
	SET revoked_at = NOW()
	WHERE id = $1 AND revoked_at IS NULL AND accepted_at IS NULL
	RETURNING id, project_id, inviter_admin_id, invitee_email, invitee_admin_id, role, expires_at, created_at, revoked_at, accepted_at`

	var invite Invite
	err := s.db.GetContext(ctx, &invite, stmt, id)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

// ListInvitesForEmail returns the pending invites addressed to the given email,
// used to surface "my invites" to the logged-in admin. The email is matched
// case-insensitively.
func (s *InvitesStore) ListInvitesForEmail(ctx context.Context, email string) ([]Invite, error) {
	stmt := `
	SELECT` + inviteColumns + `
	FROM project_invites pi
	LEFT JOIN admins a ON pi.inviter_admin_id = a.id
	LEFT JOIN projects p ON pi.project_id = p.id
	WHERE lower(pi.invitee_email) = lower($1)
	  AND pi.accepted_at IS NULL AND pi.revoked_at IS NULL AND pi.expires_at > NOW()
	ORDER BY pi.created_at DESC`

	var invites []Invite
	err := s.db.SelectContext(ctx, &invites, stmt, email)
	if err != nil {
		return nil, err
	}
	return invites, nil
}

func (s *InvitesStore) ListProjectInvites(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string, role *oapi.ListProjectInvitesParamsRole, status *oapi.ListProjectInvitesParamsStatus, expiresBefore *string, expiresAfter *string, inviterAdminID *uuid.UUID) ([]Invite, int, error) {
	countStmt := `
	SELECT COUNT(*)
	FROM project_invites
	WHERE project_id = $1
	AND ($2::text IS NULL OR $2::text = '' OR invitee_email ILIKE '%' || $2 || '%')
	AND ($3::text IS NULL OR role = $3)
	AND ($4::text IS NULL OR (
		($4 = 'pending'  AND revoked_at IS NULL AND accepted_at IS NULL AND expires_at > NOW()) OR
		($4 = 'accepted' AND accepted_at IS NOT NULL) OR
		($4 = 'revoked'  AND revoked_at IS NOT NULL) OR
		($4 = 'expired'  AND expires_at <= NOW() AND accepted_at IS NULL AND revoked_at IS NULL)
	))
	AND ($5::timestamptz IS NULL OR expires_at <= $5::timestamptz)
	AND ($6::timestamptz IS NULL OR expires_at >= $6::timestamptz)
	AND ($7::uuid IS NULL OR inviter_admin_id = $7::uuid)`

	var total int
	err := s.db.GetContext(ctx, &total, countStmt, projectID, search, role, status, expiresBefore, expiresAfter, inviterAdminID)
	if err != nil {
		return nil, 0, err
	}

	stmt := `
	SELECT` + inviteColumns + `
	FROM project_invites pi
	LEFT JOIN admins a ON pi.inviter_admin_id = a.id
	LEFT JOIN projects p ON pi.project_id = p.id
	WHERE pi.project_id = $1
	AND ($2::text IS NULL OR $2::text = '' OR pi.invitee_email ILIKE '%' || $2 || '%')
	AND ($3::text IS NULL OR pi.role = $3)
	AND ($4::text IS NULL OR (
		($4 = 'pending'  AND pi.revoked_at IS NULL AND pi.accepted_at IS NULL AND pi.expires_at > NOW()) OR
		($4 = 'accepted' AND pi.accepted_at IS NOT NULL) OR
		($4 = 'revoked'  AND pi.revoked_at IS NOT NULL) OR
		($4 = 'expired'  AND pi.expires_at <= NOW() AND pi.accepted_at IS NULL AND pi.revoked_at IS NULL)
	))
	AND ($5::timestamptz IS NULL OR pi.expires_at <= $5::timestamptz)
	AND ($6::timestamptz IS NULL OR pi.expires_at >= $6::timestamptz)
	AND ($7::uuid IS NULL OR pi.inviter_admin_id = $7::uuid)
	ORDER BY pi.created_at DESC
	LIMIT $8 OFFSET $9`

	var invites []Invite
	err = s.db.SelectContext(ctx, &invites, stmt, projectID, search, role, status, expiresBefore, expiresAfter, inviterAdminID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}
	return invites, total, nil
}

// GetPendingInviteOrganization returns the organization behind the newest
// pending invite addressed to an email, or sql.ErrNoRows when there is none.
//
// It is how somebody who was invited before they had an account lands in the
// organization that invited them, instead of getting an organization of their
// own that nobody meant to create. The invite itself stays pending: accepting
// it is what grants the project role, and that flow already exists.
func (s *InvitesStore) GetPendingInviteOrganization(ctx context.Context, email string) (uuid.UUID, error) {
	stmt := `
	SELECT p.organization_id
	FROM project_invites pi
	JOIN projects p ON p.id = pi.project_id
	WHERE lower(pi.invitee_email) = lower($1)
	AND pi.accepted_at IS NULL AND pi.revoked_at IS NULL AND pi.expires_at > NOW()
	AND p.organization_id IS NOT NULL AND p.deleted_at IS NULL
	ORDER BY pi.created_at DESC
	LIMIT 1`

	var organizationID uuid.UUID
	if err := s.db.GetContext(ctx, &organizationID, stmt, email); err != nil {
		return uuid.Nil, err
	}
	return organizationID, nil
}

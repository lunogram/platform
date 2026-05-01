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
	InviterAdminID    uuid.UUID  `db:"inviter_admin_id"`
	InviterAdminEmail *string    `db:"inviter_admin_email"`
	InviteeEmail      string     `db:"invitee_email"`
	Token             string     `db:"token"`
	Nonce             *string    `db:"nonce"`
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
		InviterAdminId:    &invite.InviterAdminID,
		InviteeEmail:      &inviteeEmail,
		InviterAdminEmail: invite.InviterAdminEmail,
		Token:             &invite.Token,
		Nonce:             invite.Nonce,
		Role:              &role,
		ExpiresAt:         &invite.ExpiresAt,
		AcceptedAt:        invite.AcceptedAt,
		RevokedAt:         invite.RevokedAt,
	}
}

func (s *InvitesStore) CreateProjectInvite(ctx context.Context, projectID uuid.UUID, inviterAdminID string, inviteeEmail string, role oapi.CreateProjectInviteRole, token string, nonce string, expiresIn string) (*Invite, error) {
	stmt := `
	WITH revoked AS (
		UPDATE project_invites
		SET revoked_at = NOW()
		WHERE project_id = $1
		  AND invitee_email = $3
		  AND revoked_at IS NULL
		  AND accepted_at IS NULL
		  AND expires_at > NOW()
	)
	INSERT INTO project_invites (project_id, inviter_admin_id, invitee_email, role, token, nonce, expires_at)
	VALUES ($1, $2, $3, $4, $5, $6, NOW() + $7::interval)
	RETURNING id, project_id, inviter_admin_id, invitee_email, role, token, nonce, expires_at, created_at, revoked_at, accepted_at`

	var invite Invite
	err := s.db.GetContext(ctx, &invite, stmt, projectID, inviterAdminID, inviteeEmail, role, token, nonce, expiresIn)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (s *InvitesStore) GetInviteByToken(ctx context.Context, token string) (*Invite, error) {
	stmt := `
	SELECT id, project_id, inviter_admin_id, invitee_email, role, token, nonce, expires_at, created_at, revoked_at, accepted_at
	FROM project_invites
	WHERE token = $1 AND revoked_at IS NULL AND accepted_at IS NULL AND expires_at > NOW()`

	var invite Invite
	err := s.db.GetContext(ctx, &invite, stmt, token)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (s *InvitesStore) AcceptProjectInvite(ctx context.Context, token string) (*Invite, error) {
	stmt := `
	UPDATE project_invites
	SET accepted_at = NOW()
	WHERE token = $1 AND revoked_at IS NULL AND accepted_at IS NULL AND expires_at > NOW()
	RETURNING id, project_id, inviter_admin_id, invitee_email, role, token, nonce, expires_at, created_at, revoked_at, accepted_at`

	var invite Invite
	err := s.db.GetContext(ctx, &invite, stmt, token)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (s *InvitesStore) RevokeProjectInvite(ctx context.Context, token string) (*Invite, error) {
	stmt := `
	UPDATE project_invites
	SET revoked_at = NOW()
	WHERE token = $1 AND revoked_at IS NULL AND accepted_at IS NULL
	RETURNING id, project_id, inviter_admin_id, invitee_email, role, token, nonce, expires_at, created_at, revoked_at, accepted_at`

	var invite Invite
	err := s.db.GetContext(ctx, &invite, stmt, token)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (s *InvitesStore) ListProjectInvites(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string, role *oapi.ListProjectInvitesParamsRole, status *oapi.ListProjectInvitesParamsStatus, expiresBefore *string, expiresAfter *string, inviterAdminID *uuid.UUID) ([]Invite, int, error) {
	countStmt := `
	SELECT COUNT(*)
	FROM project_invites
	WHERE project_id = $1
	AND ($2::text IS NULL OR $2::text = '' OR invitee_email ILIKE '%' || $2 || '%')
	AND ($3::text IS NULL OR role = $3)
	AND ($4::text IS NULL OR (
		$4 = 'pending'   AND revoked_at IS NULL AND accepted_at IS NULL AND expires_at > NOW() OR
		$4 = 'accepted'  AND accepted_at IS NOT NULL OR
		$4 = 'revoked'   AND revoked_at IS NOT NULL OR
		$4 = 'expired'   AND expires_at <= NOW() AND accepted_at IS NULL AND revoked_at IS NULL
	))
	AND ($5::date IS NULL OR expires_at >= $5::date)
	AND ($6::date IS NULL OR expires_at <= $6::date)
	AND ($7::uuid IS NULL OR inviter_admin_id = $7::uuid)`

	var total int
	err := s.db.GetContext(ctx, &total, countStmt, projectID, search, role, status, expiresBefore, expiresAfter, inviterAdminID)
	if err != nil {
		return nil, 0, err
	}

	stmt := `
	SELECT pi.id AS id, pi.project_id AS project_id, pi.inviter_admin_id AS inviter_admin_id, a.email AS inviter_admin_email, pi.invitee_email AS invitee_email, pi.role AS role, pi.token AS token, pi.nonce AS nonce, pi.expires_at AS expires_at, pi.created_at AS created_at, pi.revoked_at AS revoked_at, pi.accepted_at AS accepted_at
	FROM project_invites as pi
	INNER JOIN admins as a ON pi.inviter_admin_id = a.id
	WHERE pi.project_id = $1
	AND ($2::text IS NULL OR $2::text = '' OR pi.invitee_email ILIKE '%' || $2 || '%')
	AND ($3::text IS NULL OR pi.role = $3)
	AND ($4::text IS NULL OR (
		$4 = 'pending'   AND pi.revoked_at IS NULL AND pi.accepted_at IS NULL AND pi.expires_at > NOW() OR
		$4 = 'accepted'  AND pi.accepted_at IS NOT NULL OR
		$4 = 'revoked'   AND pi.revoked_at IS NOT NULL OR
		$4 = 'expired'   AND pi.expires_at <= NOW() AND pi.accepted_at IS NULL AND pi.revoked_at IS NULL
	))
	AND ($5::date IS NULL OR pi.expires_at >= $5::date)
	AND ($6::date IS NULL OR pi.expires_at <= $6::date)
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

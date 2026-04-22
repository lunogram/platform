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
	ID             uuid.UUID  `db:"id"`
	ProjectID      uuid.UUID  `db:"project_id"`
	InviterAdminID uuid.UUID  `db:"inviter_admin_id"`
	InviteeEmail   string     `db:"invitee_email"`
	Token          string     `db:"token"`
	Role           string     `db:"role"`
	ExpiresAt      time.Time  `db:"expires_at"`
	AcceptedAt     *time.Time `db:"accepted_at"`
	RevokedAt      *time.Time `db:"revoked_at"`
	CreatedAt      time.Time  `db:"created_at"`
}

func (invite *Invite) OAPI() oapi.ProjectInvite {
	role := oapi.ProjectInviteRole(invite.Role)
	inviteeEmail := types.Email(invite.InviteeEmail)

	return oapi.ProjectInvite{
		Id:             &invite.ID,
		ProjectId:      &invite.ProjectID,
		InviterAdminId: &invite.InviterAdminID,
		InviteeEmail:   &inviteeEmail,
		Token:          &invite.Token,
		Role:           &role,
		ExpiresAt:      &invite.ExpiresAt,
		AcceptedAt:     invite.AcceptedAt,
		RevokedAt:      invite.RevokedAt,
	}
}

func (s *InvitesStore) CreateProjectInvite(ctx context.Context, projectID uuid.UUID, inviterAdminID string, inviteeEmail string, role oapi.CreateProjectInviteRole, token string, expiresIn string) (*Invite, error) {
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
	INSERT INTO project_invites (project_id, inviter_admin_id, invitee_email, role, token, expires_at)
	VALUES ($1, $2, $3, $4, $5, NOW() + $6::interval)
	RETURNING id, project_id, inviter_admin_id, invitee_email, role, token, expires_at, created_at, revoked_at, accepted_at`

	var invite Invite
	err := s.db.GetContext(ctx, &invite, stmt, projectID, inviterAdminID, inviteeEmail, role, token, expiresIn)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (s *InvitesStore) GetInviteByToken(ctx context.Context, token string) (*Invite, error) {
	stmt := `
	SELECT id, project_id, inviter_admin_id, invitee_email, role, token, expires_at, created_at, revoked_at, accepted_at
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
	RETURNING id, project_id, inviter_admin_id, invitee_email, role, token, expires_at, created_at, revoked_at, accepted_at`

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
	RETURNING id, project_id, inviter_admin_id, invitee_email, role, token, expires_at, created_at, revoked_at, accepted_at`

	var invite Invite
	err := s.db.GetContext(ctx, &invite, stmt, token)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (s *InvitesStore) ListProjectInvites(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string, role *oapi.ListProjectInvitesParamsRole, status *oapi.ListProjectInvitesParamsStatus, expiresBefore *string, expiresAfter *string) ([]Invite, int, error) {
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
	AND ($6::date IS NULL OR expires_at <= $6::date)`

	var total int
	err := s.db.GetContext(ctx, &total, countStmt, projectID, search, role, status, expiresBefore, expiresAfter)
	if err != nil {
		return nil, 0, err
	}

	stmt := `
	SELECT id, project_id, inviter_admin_id, invitee_email, role, token, expires_at, created_at, revoked_at, accepted_at
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
	ORDER BY created_at DESC
	LIMIT $7 OFFSET $8`

	var invites []Invite
	err = s.db.SelectContext(ctx, &invites, stmt, projectID, search, role, status, expiresBefore, expiresAfter, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}
	return invites, total, nil
}

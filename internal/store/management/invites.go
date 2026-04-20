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
	invitee_email  string     `db:"invitee_email"`
	token          string     `db:"token"`
	Role           string     `db:"role"`
	ExpiresAt      time.Time  `db:"expires_at"`
	AcceptedAt     *time.Time `db:"accepted_at"`
	RevokedAt      *time.Time `db:"revoked_at"`
	CreatedAt      time.Time  `db:"created_at"`
}

func (invite *Invite) OAPI() oapi.ProjectInvite {
	role := oapi.ProjectInviteRole(invite.Role)
	inviteeEmail := types.Email(invite.invitee_email)

	return oapi.ProjectInvite{
		Id:             &invite.ID,
		ProjectId:      &invite.ProjectID,
		InviterAdminId: &invite.InviterAdminID,
		InviteeEmail:   &inviteeEmail,
		Token:          &invite.token,
		Role:           &role,
		ExpiresAt:      &invite.ExpiresAt,
		AcceptedAt:     invite.AcceptedAt,
		RevokedAt:      invite.RevokedAt,
	}
}

func (s *InvitesStore) CreateProjectInvite(ctx context.Context, projectID uuid.UUID, inviterAdminID string, inviteeEmail string, role oapi.CreateProjectInviteRole, token string, expiresIn string) (*Invite, error) {
	stmt := `
	INSERT INTO invites (project_id, inviter_admin_id, invitee_email, role, token, expires_at)
	VALUES ($1, $2, $3, $4, $5, NOW() + $6::interval)
	RETURNING id, project_id, inviter_admin_id, invitee_email, role, token, expires_at, created_at, revoked_at, accepted_at`

	var invite Invite
	err := s.db.GetContext(ctx, &invite, stmt, projectID, inviterAdminID, inviteeEmail, role, token, expiresIn)
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

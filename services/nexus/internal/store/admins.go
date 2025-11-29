package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/claim"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/oapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func NewAdminsStore(db DB) *AdminsStore {
	return &AdminsStore{db: db}
}

type AdminsStore struct {
	db DB
}

type Admin struct {
	ID             uuid.UUID `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	ExternalID     *string   `db:"external_id"`
	Email          string    `db:"email"`
	FirstName      *string   `db:"first_name"`
	LastName       *string   `db:"last_name"`
	ImageURL       *string   `db:"image_url"`
	Role           string    `db:"role"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

func (admin *Admin) OAPI() oapi.Admin {
	return oapi.Admin{
		Id:             admin.ID,
		OrganizationId: admin.OrganizationID,
		ExternalId:     admin.ExternalID,
		Email:          openapi_types.Email(admin.Email),
		FirstName:      admin.FirstName,
		LastName:       admin.LastName,
		ImageUrl:       admin.ImageURL,
		Role:           oapi.AdminRole(admin.Role),
		CreatedAt:      admin.CreatedAt,
		UpdatedAt:      admin.UpdatedAt,
	}
}

func (s *AdminsStore) GetAdmin(ctx context.Context, id uuid.UUID) (*Admin, error) {
	stmt := `
	SELECT id, organization_id, external_id, email, first_name, last_name, image_url, role, created_at, updated_at
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

func (s *AdminsStore) GetAdminByExternalID(ctx context.Context, externalID string) (*Admin, error) {
	stmt := `
	SELECT id, organization_id, external_id, email, first_name, last_name, image_url, role, created_at, updated_at
	FROM admins
	WHERE external_id = $1
	AND deleted_at IS NULL`

	var admin Admin
	err := s.db.GetContext(ctx, &admin, stmt, externalID)
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

func (s *AdminsStore) GetAdminBySubject(ctx context.Context, session claim.Session) (*Admin, error) {
	if session.Issuer != "" {
		admin, err := s.GetAdminByExternalID(ctx, session.Subject)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if admin != nil {
			return admin, nil
		}
	}

	adminID, err := uuid.Parse(session.Subject)
	if err != nil {
		return nil, problem.ErrUnauthorized(problem.Describe("invalid token"))
	}

	return s.GetAdmin(ctx, adminID)
}

func (s *AdminsStore) CreateAdmin(ctx context.Context, admin Admin) (uuid.UUID, error) {
	stmt := `
	INSERT INTO admins (organization_id, external_id, email, first_name, last_name, image_url, role)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING id
	`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		admin.OrganizationID,
		admin.ExternalID,
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

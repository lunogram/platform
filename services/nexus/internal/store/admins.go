package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
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

func (s *AdminsStore) ListAdmins(ctx context.Context, organizationID uuid.UUID, limit, offset int, search string) ([]Admin, int, error) {
	var admins []Admin
	var total int

	query := `
	SELECT 
		id, organization_id, external_id, email, first_name, last_name, image_url, role, created_at, updated_at,
		COUNT(*) OVER () AS total_count
	FROM admins
	WHERE organization_id = $1 
	AND deleted_at IS NULL`

	args := []interface{}{organizationID}
	argCount := 1

	if search != "" {
		argCount++
		query += ` AND (
			first_name ILIKE $` + strconv.Itoa(argCount) + ` OR 
			last_name ILIKE $` + strconv.Itoa(argCount) + ` OR 
			email ILIKE $` + strconv.Itoa(argCount) + `
		)`
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern)
	}

	argCount++
	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(argCount)
	args = append(args, limit)

	argCount++
	query += ` OFFSET $` + strconv.Itoa(argCount)
	args = append(args, offset)

	type result struct {
		Admin
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, args...)
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

func (s *AdminsStore) GetAdminByEmail(ctx context.Context, email string, organizationID uuid.UUID) (*Admin, error) {
	stmt := `
	SELECT id, organization_id, external_id, email, first_name, last_name, image_url, role, created_at, updated_at
	FROM admins
	WHERE email = $1
	AND organization_id = $2
	AND deleted_at IS NULL`

	var admin Admin
	err := s.db.GetContext(ctx, &admin, stmt, email, organizationID)
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

func (s *AdminsStore) UpdateAdmin(ctx context.Context, id uuid.UUID, email *string, firstName, lastName *string, role *string) error {
	updates := []string{}
	args := []interface{}{id}
	argCount := 1

	if email != nil {
		argCount++
		updates = append(updates, "email = $"+strconv.Itoa(argCount))
		args = append(args, *email)
	}

	if firstName != nil {
		argCount++
		updates = append(updates, "first_name = $"+strconv.Itoa(argCount))
		args = append(args, *firstName)
	}

	if lastName != nil {
		argCount++
		updates = append(updates, "last_name = $"+strconv.Itoa(argCount))
		args = append(args, *lastName)
	}

	if role != nil {
		argCount++
		updates = append(updates, "role = $"+strconv.Itoa(argCount))
		args = append(args, *role)
	}

	if len(updates) == 0 {
		return nil
	}

	stmt := `UPDATE admins SET ` + strings.Join(updates, ", ") + ` WHERE id = $1 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, args...)
	return err
}

func (s *AdminsStore) DeleteAdmin(ctx context.Context, id uuid.UUID) error {
	stmt := `UPDATE admins SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1 AND deleted_at IS NULL`
	_, err := s.db.ExecContext(ctx, stmt, id)
	return err
}

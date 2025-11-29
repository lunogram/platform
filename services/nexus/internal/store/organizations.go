package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Organization struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func NewOrganizationsStore(db DB) *OrganizationsStore {
	return &OrganizationsStore{db: db}
}

type OrganizationsStore struct {
	db DB
}

func (s *OrganizationsStore) CreateOrganization(ctx context.Context, name string) (uuid.UUID, error) {
	stmt := `
	INSERT INTO organizations (name)
	VALUES ($1)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, name)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *OrganizationsStore) GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error) {
	stmt := `
	SELECT id, name, created_at, updated_at
	FROM organizations
	WHERE id = $1
	AND deleted_at IS NULL`

	var org Organization
	err := s.db.GetContext(ctx, &org, stmt, id)
	if err != nil {
		return nil, err
	}

	return &org, nil
}

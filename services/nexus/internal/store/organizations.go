package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/oapi"
)

type Organization struct {
	ID                        uuid.UUID       `db:"id"`
	Name                      string          `db:"name"`
	Username                  sql.NullString  `db:"username"`
	Domain                    sql.NullString  `db:"domain"`
	Auth                      json.RawMessage `db:"auth"`
	NotificationProviderID    *uuid.UUID      `db:"notification_provider_id"`
	TrackingDeeplinkMirrorURL sql.NullString  `db:"tracking_deeplink_mirror_url"`
	CreatedAt                 time.Time       `db:"created_at"`
	UpdatedAt                 time.Time       `db:"updated_at"`
	DeletedAt                 sql.NullTime    `db:"deleted_at"`
}

func (o *Organization) OAPI() oapi.Organization {
	org := oapi.Organization{
		Id:        o.ID,
		Name:      o.Name,
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}

	if o.Username.Valid {
		org.Username = &o.Username.String
	}

	if o.Domain.Valid {
		org.Domain = &o.Domain.String
	}

	if o.TrackingDeeplinkMirrorURL.Valid {
		org.TrackingDeeplinkMirrorUrl = &o.TrackingDeeplinkMirrorURL.String
	}

	if o.NotificationProviderID != nil {
		org.NotificationProviderId = o.NotificationProviderID
	}

	return org
}

type OrganizationUpdate struct {
	Username                  *string
	Domain                    *string
	TrackingDeeplinkMirrorURL *string
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
	SELECT id, name, username, domain, COALESCE(auth, '{}'::jsonb) as auth, notification_provider_id, 
		   tracking_deeplink_mirror_url, created_at, updated_at, deleted_at
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

func (s *OrganizationsStore) UpdateOrganization(ctx context.Context, id uuid.UUID, update OrganizationUpdate) error {
	stmt := `
	UPDATE organizations
	SET username = COALESCE($2, username),
		domain = COALESCE($3, domain),
		tracking_deeplink_mirror_url = COALESCE($4, tracking_deeplink_mirror_url),
		updated_at = CURRENT_TIMESTAMP
	WHERE id = $1
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, id, update.Username, update.Domain, update.TrackingDeeplinkMirrorURL)
	return err
}

func (s *OrganizationsStore) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	stmt := `
	UPDATE organizations
	SET deleted_at = CURRENT_TIMESTAMP
	WHERE id = $1
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, id)
	return err
}

func (s *OrganizationsStore) GetOrganizationIntegrations(ctx context.Context, orgID uuid.UUID) ([]Provider, error) {
	stmt := `
	SELECT p.id, p.project_id, p.type, p.group, p.data, p.is_default, 
		   p.rate_limit, p.rate_interval, p.name, p.created_at, p.updated_at
	FROM providers p
	LEFT JOIN projects pr ON pr.id = p.project_id
	WHERE pr.organization_id = $1
	AND p.deleted_at IS NULL`

	var providers []Provider
	err := s.db.SelectContext(ctx, &providers, stmt, orgID)
	if err != nil {
		return nil, err
	}

	return providers, nil
}

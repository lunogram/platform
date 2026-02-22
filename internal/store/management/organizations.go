package management

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

type Organization struct {
	ID                        uuid.UUID  `db:"id"`
	Name                      string     `db:"name"`
	NotificationProviderID    *uuid.UUID `db:"notification_provider_id"`
	TrackingDeeplinkMirrorURL *string    `db:"tracking_deeplink_mirror_url"`
	CreatedAt                 time.Time  `db:"created_at"`
	UpdatedAt                 time.Time  `db:"updated_at"`
}

func (o *Organization) OAPI() oapi.Tenant {
	return oapi.Tenant{
		Id:                        o.ID,
		Name:                      o.Name,
		TrackingDeeplinkMirrorUrl: o.TrackingDeeplinkMirrorURL,
		NotificationProviderId:    o.NotificationProviderID,
		CreatedAt:                 o.CreatedAt,
		UpdatedAt:                 o.UpdatedAt,
	}
}

type OrganizationUpdate struct {
	TrackingDeeplinkMirrorURL *string
}

func NewOrganizationsStore(db store.DB) *OrganizationsStore {
	return &OrganizationsStore{db: db}
}

type OrganizationsStore struct {
	db store.DB
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
	SELECT id, name, notification_provider_id,
		   tracking_deeplink_mirror_url, created_at, updated_at
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
	SET tracking_deeplink_mirror_url = COALESCE($2, tracking_deeplink_mirror_url)
	WHERE id = $1
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, id, update.TrackingDeeplinkMirrorURL)
	return err
}

func (s *OrganizationsStore) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	stmt := `
	UPDATE organizations
	SET deleted_at = NOW()
	WHERE id = $1
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, id)
	return err
}

func (s *OrganizationsStore) GetOrganizationIntegrations(ctx context.Context, orgID uuid.UUID) (Providers, error) {
	stmt := `
	SELECT p.id, p.project_id, p.module, p.channel, p.data, p.is_default,
		   p.rate_limit, p.rate_interval, p.name, p.created_at, p.updated_at
	FROM providers p
	INNER JOIN projects pr ON pr.id = p.project_id
	WHERE pr.organization_id = $1
	AND p.deleted_at IS NULL
	AND pr.deleted_at IS NULL`

	var providers []Provider
	err := s.db.SelectContext(ctx, &providers, stmt, orgID)
	if err != nil {
		return nil, err
	}

	return providers, nil
}

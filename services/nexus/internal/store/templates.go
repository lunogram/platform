package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
)

type Templates []Template

func (templates Templates) OAPI() []oapi.Template {
	result := make([]oapi.Template, len(templates))
	for index, template := range templates {
		result[index] = template.OAPI()
	}
	return result
}

type Template struct {
	ID         uuid.UUID       `db:"id"`
	CampaignID uuid.UUID       `db:"campaign_id"`
	ProjectID  uuid.UUID       `db:"project_id"`
	Type       string          `db:"type"`
	Data       json.RawMessage `db:"data"`
	Locale     string          `db:"locale"`
	CreatedAt  time.Time       `db:"created_at"`
	UpdatedAt  time.Time       `db:"updated_at"`
}

func (template Template) OAPI() oapi.Template {
	return oapi.Template{
		Id:         template.ID,
		CampaignId: template.CampaignID,
		Type:       oapi.Channel(template.Type),
		Data:       template.Data,
		Locale:     template.Locale,
		ProjectId:  template.ProjectID,
		UpdatedAt:  template.UpdatedAt,
		CreatedAt:  template.CreatedAt,
	}
}

func NewTemplatesStore(db DB) *TemplatesStore {
	return &TemplatesStore{db: db}
}

type TemplatesStore struct {
	db DB
}

func (s *TemplatesStore) CreateTemplate(ctx context.Context, projectID, campaignID uuid.UUID, channel string, locale string) (uuid.UUID, error) {
	// TODO: remove channel type, this type is needed within the "legacy" NodeJS back-end
	stmt := `
	INSERT INTO templates (project_id, campaign_id, type, locale)
	VALUES ($1, $2, $3, $4)
	RETURNING id`

	var id uuid.UUID
	err := s.db.QueryRowContext(ctx, stmt, projectID, campaignID, channel, locale).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *TemplatesStore) GetTemplate(ctx context.Context, projectID, templateID uuid.UUID) (*Template, error) {
	query := `
	SELECT templates.id, templates.project_id, templates.campaign_id, campaigns.channel AS type, templates.data, templates.locale, templates.created_at, templates.updated_at
	FROM templates
	JOIN campaigns ON templates.campaign_id = campaigns.id
	WHERE templates.project_id = $1
	AND templates.id = $2`

	var template Template
	err := s.db.GetContext(ctx, &template, query, projectID, templateID)
	if err != nil {
		return nil, err
	}

	return &template, nil
}

func (s *TemplatesStore) ListTemplates(ctx context.Context, projectID, campaignID uuid.UUID) ([]Template, error) {
	query := `
	SELECT templates.id, templates.project_id, templates.campaign_id, campaigns.channel AS type, templates.data, templates.locale, templates.created_at, templates.updated_at
	FROM templates
	JOIN campaigns ON templates.campaign_id = campaigns.id
	WHERE templates.project_id = $1
	AND templates.campaign_id = $2`

	var templates []Template
	err := s.db.SelectContext(ctx, &templates, query, projectID, campaignID)
	if err != nil {
		return nil, err
	}

	return templates, nil
}

type TemplateUpdate struct {
	Data *json.RawMessage
}

func (s *TemplatesStore) UpdateTemplate(ctx context.Context, projectID, templateID uuid.UUID, update TemplateUpdate) error {
	query := `
	UPDATE templates
	SET data = COALESCE($3, data)
	WHERE project_id = $1
	AND id = $2`

	_, err := s.db.ExecContext(ctx, query, projectID, templateID, update.Data)
	return err
}

func (s *TemplatesStore) DeleteTemplate(ctx context.Context, projectID, templateID uuid.UUID) error {
	query := `
	DELETE FROM templates
	WHERE project_id = $1
	AND id = $2`

	_, err := s.db.ExecContext(ctx, query, projectID, templateID)
	return err
}

func (s *TemplatesStore) DuplicateTemplate(ctx context.Context, projectID, templateID, newCampaignID uuid.UUID) error {
	query := `
	INSERT INTO templates (project_id, campaign_id, type, data, locale)
	SELECT project_id, $1, type, data, locale
	FROM templates
	WHERE project_id = $2
	AND id = $3`

	_, err := s.db.ExecContext(ctx, query, newCampaignID, projectID, templateID)
	return err
}

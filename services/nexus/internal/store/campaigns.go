package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
)

type Campaigns []Campaign

func (campaigns Campaigns) OAPI() []oapi.Campaign {
	result := make([]oapi.Campaign, len(campaigns))
	for index, campaign := range campaigns {
		result[index] = campaign.OAPI()
	}
	return result
}

type Campaign struct {
	ID             uuid.UUID       `db:"id"`
	ProjectID      uuid.UUID       `db:"project_id"`
	Name           string          `db:"name"`
	Channel        string          `db:"channel"`
	ProviderID     *uuid.UUID      `db:"provider_id"`
	SubscriptionID *uuid.UUID      `db:"subscription_id"`
	Delivery       JSONB[Delivery] `db:"delivery"`
	Templates      Templates       `db:"-"`
	Provider       *Provider       `db:"-"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
	DeletedAt      *time.Time      `db:"deleted_at"`
}

func (campaign Campaign) OAPI() oapi.Campaign {
	result := oapi.Campaign{
		Id:             campaign.ID,
		ProjectId:      campaign.ProjectID,
		Name:           campaign.Name,
		Channel:        oapi.Channel(campaign.Channel),
		SubscriptionId: campaign.SubscriptionID,
		Delivery:       campaign.Delivery.Data.OAPI(),
		Templates:      campaign.Templates.OAPI(),
		CreatedAt:      campaign.CreatedAt,
		UpdatedAt:      campaign.UpdatedAt,
	}

	if campaign.Provider != nil {
		provider := campaign.Provider.OAPI()
		result.Provider = &provider
	}

	return result
}

type Delivery struct {
	Sent   int `db:"sent"`
	Opens  int `db:"opens"`
	Total  int `db:"total"`
	Clicks int `db:"clicks"`
}

func (delivery Delivery) OAPI() oapi.Delivery {
	return oapi.Delivery{
		Sent:   delivery.Sent,
		Opens:  delivery.Opens,
		Total:  delivery.Total,
		Clicks: delivery.Clicks,
	}
}

func NewCampaignsStore(db DB) *CampaignsStore {
	return &CampaignsStore{
		db:        db,
		templates: NewTemplatesStore(db),
		providers: NewProvidersStore(db),
	}
}

type CampaignsStore struct {
	db        DB
	templates *TemplatesStore
	providers *ProvidersStore
}

func (s *CampaignsStore) CreateCampaign(ctx context.Context, campaign Campaign) (uuid.UUID, error) {
	stmt := `
	INSERT INTO campaigns (project_id, name, channel, provider_id, subscription_id)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, campaign.ProjectID, campaign.Name, campaign.Channel, campaign.ProviderID, campaign.SubscriptionID)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *CampaignsStore) ListCampaigns(ctx context.Context, project uuid.UUID, pagination Pagination) (Campaigns, int, error) {
	query := `
	SELECT id, project_id, name, channel, provider_id, subscription_id, delivery, created_at, updated_at, deleted_at,
		COUNT(*) OVER () AS total_count
	FROM campaigns
	WHERE project_id = $1
	AND deleted_at IS NULL
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	var results []struct {
		Campaign
		TotalCount int `db:"total_count"`
	}
	err := s.db.SelectContext(ctx, &results, query, project, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	campaigns := make(Campaigns, len(results))
	total := 0

	for i, r := range results {
		campaigns[i] = r.Campaign
		if i == 0 {
			total = r.TotalCount
		}
	}

	return campaigns, total, nil
}

func (s *CampaignsStore) GetCampaign(ctx context.Context, projectID, campaignID uuid.UUID) (*Campaign, error) {
	query := `
	SELECT id, project_id, name, channel, provider_id, subscription_id, delivery, created_at, updated_at, deleted_at
	FROM campaigns
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NULL`

	var campaign Campaign
	err := s.db.GetContext(ctx, &campaign, query, projectID, campaignID)
	if err != nil {
		return nil, err
	}

	campaign.Templates, err = s.templates.ListTemplates(ctx, projectID, campaignID)
	if err != nil {
		return nil, err
	}

	if campaign.ProviderID != nil {
		campaign.Provider, err = s.providers.GetProvider(ctx, *campaign.ProviderID)
		if err != nil {
			return nil, err
		}
	}

	return &campaign, nil
}

func (s *CampaignsStore) GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (*Campaign, error) {
	query := `
	SELECT id, project_id, name, channel, provider_id, subscription_id, delivery, created_at, updated_at, deleted_at
	FROM campaigns
	WHERE id = $1
	AND deleted_at IS NULL`

	var campaign Campaign
	err := s.db.GetContext(ctx, &campaign, query, campaignID)
	if err != nil {
		return nil, err
	}

	return &campaign, nil
}

type CampaignUpdate struct {
	Name       *string
	ProviderID *uuid.UUID
}

func (s *CampaignsStore) UpdateCampaign(ctx context.Context, projectID, campaignID uuid.UUID, update CampaignUpdate) error {
	query := `
	UPDATE campaigns
	SET
		name = COALESCE($1, name),
		provider_id = COALESCE($2, provider_id)
	WHERE project_id = $3
	AND id = $4
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, update.Name, update.ProviderID, projectID, campaignID)
	return err
}

func (s *CampaignsStore) DeleteCampaign(ctx context.Context, projectID, campaignID uuid.UUID) error {
	query := `
	UPDATE campaigns
	SET deleted_at = NOW()
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, projectID, campaignID)
	return err
}

type CampaignUsers []CampaignUser

func (users CampaignUsers) OAPI() []oapi.CampaignUser {
	result := make([]oapi.CampaignUser, len(users))
	for index, user := range users {
		result[index] = user.OAPI()
	}
	return result
}

type CampaignUser struct {
	ID         uuid.UUID  `db:"id"`
	CampaignID uuid.UUID  `db:"campaign_id"`
	UserID     uuid.UUID  `db:"user_id"`
	State      string     `db:"state"`
	SendAt     *time.Time `db:"sent_at"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
}

func (user CampaignUser) OAPI() oapi.CampaignUser {
	return oapi.CampaignUser{
		Id:         user.ID,
		CampaignId: user.CampaignID,
		UserId:     user.UserID,
		Status:     oapi.CampaignUserStatus(user.State),
		SentAt:     user.SendAt,
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	}
}

func (s *CampaignsStore) GetCampaignUsers(ctx context.Context, projectID, campaignID uuid.UUID, pagination Pagination) (CampaignUsers, int, error) {
	query := `
	SELECT cs.id, cs.campaign_id, cs.user_id, cs.state, cs.sent_at, cs.created_at, cs.updated_at,
		COUNT(*) OVER () AS total_count
	FROM campaign_sends cs
	JOIN campaigns c ON c.id = cs.campaign_id
	WHERE c.project_id = $1
	AND cs.campaign_id = $2
	ORDER BY cs.created_at DESC
	LIMIT $3 OFFSET $4`

	var results []struct {
		CampaignUser
		TotalCount int `db:"total_count"`
	}
	err := s.db.SelectContext(ctx, &results, query, projectID, campaignID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	users := make(CampaignUsers, len(results))
	total := 0

	for i, r := range results {
		users[i] = r.CampaignUser
		if i == 0 {
			total = r.TotalCount
		}
	}

	return users, total, nil
}

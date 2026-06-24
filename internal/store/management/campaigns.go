package management

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

type Campaigns []Campaign

func (campaigns Campaigns) OAPI() []oapi.Campaign {
	result := make([]oapi.Campaign, len(campaigns))
	for index, campaign := range campaigns {
		result[index] = campaign.OAPI()
	}
	return result
}

type CampaignVariable struct {
	Name    string  `json:"name"`
	Default *string `json:"default,omitempty"`
}

type CampaignVariables []CampaignVariable

type Campaign struct {
	ID             uuid.UUID                      `db:"id"`
	ProjectID      uuid.UUID                      `db:"project_id"`
	Name           string                         `db:"name"`
	Channel        string                         `db:"channel"`
	SubscriptionID *uuid.UUID                     `db:"subscription_id"`
	Transactional  bool                           `db:"transactional"`
	Delivery       store.JSONB[Delivery]          `db:"delivery"`
	Variables      store.JSONB[CampaignVariables] `db:"variables"`
	Templates      Templates                      `db:"-"`
	CreatedAt      time.Time                      `db:"created_at"`
	UpdatedAt      time.Time                      `db:"updated_at"`
	DeletedAt      *time.Time                     `db:"deleted_at"`
}

func (campaign Campaign) OAPI() oapi.Campaign {
	variables := make([]oapi.CampaignVariable, len(campaign.Variables.Data))
	for i, v := range campaign.Variables.Data {
		variables[i] = oapi.CampaignVariable{
			Name:    v.Name,
			Default: v.Default,
		}
	}

	archived := campaign.DeletedAt != nil
	result := oapi.Campaign{
		Id:             campaign.ID,
		ProjectId:      campaign.ProjectID,
		Name:           campaign.Name,
		Channel:        oapi.Channel(campaign.Channel),
		SubscriptionId: campaign.SubscriptionID,
		Transactional:  campaign.Transactional,
		Delivery:       campaign.Delivery.Data.OAPI(),
		Variables:      &variables,
		Templates:      campaign.Templates.OAPI(),
		CreatedAt:      campaign.CreatedAt,
		UpdatedAt:      campaign.UpdatedAt,
		Archived:       &archived,
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

func NewCampaignsStore(db store.DB) *CampaignsStore {
	return &CampaignsStore{
		db:        db,
		templates: NewTemplatesStore(db),
	}
}

type CampaignsStore struct {
	db        store.DB
	templates *TemplatesStore
}

func (s *CampaignsStore) CreateCampaign(ctx context.Context, campaign Campaign) (uuid.UUID, error) {
	stmt := `
	INSERT INTO campaigns (project_id, name, channel, subscription_id, transactional)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, campaign.ProjectID, campaign.Name, campaign.Channel, campaign.SubscriptionID, campaign.Transactional)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *CampaignsStore) ListCampaigns(ctx context.Context, project uuid.UUID, pagination store.Pagination, search string, archivedOnly bool) (Campaigns, int, error) {
	query := `
	SELECT id, project_id, COALESCE(name, '') AS name, channel, subscription_id, transactional, delivery, variables, created_at, updated_at, deleted_at,
		COUNT(*) OVER () AS total_count
	FROM campaigns
	WHERE project_id = $1
	AND (($5 = false AND deleted_at IS NULL) OR ($5 = true AND deleted_at IS NOT NULL))
	AND ($4 = '' OR COALESCE(name, '') ILIKE '%' || $4 || '%')
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	var results []struct {
		Campaign
		TotalCount int `db:"total_count"`
	}
	err := s.db.SelectContext(ctx, &results, query, project, pagination.Limit, pagination.Offset, search, archivedOnly)
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
	SELECT id, project_id, COALESCE(name, '') AS name, channel, subscription_id, transactional, delivery, variables, created_at, updated_at, deleted_at
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

	return &campaign, nil
}

type CampaignUpdate struct {
	Name           *string
	SubscriptionID *uuid.UUID
	Transactional  *bool
	Variables      *store.JSONB[CampaignVariables]
}

func (s *CampaignsStore) UpdateCampaign(ctx context.Context, projectID, campaignID uuid.UUID, update CampaignUpdate) error {
	query := `
	UPDATE campaigns
	SET
		name = COALESCE($1, name),
		variables = COALESCE($2, variables),
		transactional = COALESCE($3, transactional),
		subscription_id = CASE
			WHEN COALESCE($3, transactional) THEN NULL
			ELSE COALESCE($4, subscription_id)
		END
	WHERE project_id = $5
	AND id = $6
	AND deleted_at IS NULL`

	var variablesVal any
	if update.Variables != nil {
		v, err := update.Variables.Value()
		if err != nil {
			return err
		}
		variablesVal = v
	}

	_, err := s.db.ExecContext(ctx, query, update.Name, variablesVal, update.Transactional, update.SubscriptionID, projectID, campaignID)
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

func (s *CampaignsStore) UnarchiveCampaign(ctx context.Context, projectID, campaignID uuid.UUID) error {
	query := `
	UPDATE campaigns
	SET deleted_at = NULL
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NOT NULL`

	result, err := s.db.ExecContext(ctx, query, projectID, campaignID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return store.ErrNoRows
	}
	return nil
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

func (s *CampaignsStore) GetCampaignUsers(ctx context.Context, usersDB store.DB, projectID, campaignID uuid.UUID, pagination store.Pagination) (CampaignUsers, int, error) {
	// First verify the campaign belongs to this project (management DB)
	var exists bool
	err := s.db.GetContext(ctx, &exists, `
		SELECT EXISTS(
			SELECT 1 FROM campaigns
			WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL
		)`, campaignID, projectID)
	if err != nil {
		return nil, 0, err
	}
	if !exists {
		return []CampaignUser{}, 0, nil
	}

	// Query user_inbox_messages from users DB
	query := `
	SELECT id, campaign_id, user_id,
		CASE WHEN sent_at IS NOT NULL THEN 'sent' ELSE 'pending' END AS state,
		sent_at, created_at, updated_at,
		COUNT(*) OVER () AS total_count
	FROM user_inbox_messages
	WHERE campaign_id = $1 AND deleted_at IS NULL
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	var results []struct {
		CampaignUser
		TotalCount int `db:"total_count"`
	}
	err = usersDB.SelectContext(ctx, &results, query, campaignID, pagination.Limit, pagination.Offset)
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

// CampaignLookup contains minimal campaign data for unsubscribe flows
type CampaignLookup struct {
	ID             uuid.UUID  `db:"id"`
	ProjectID      uuid.UUID  `db:"project_id"`
	SubscriptionID *uuid.UUID `db:"subscription_id"`
	Transactional  bool       `db:"transactional"`
}

// GetCampaignByID retrieves a campaign by ID without project filter.
// This is used for unsubscribe flows where the project_id is not known.
func (s *CampaignsStore) GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (*CampaignLookup, error) {
	query := `
	SELECT id, project_id, subscription_id, transactional
	FROM campaigns
	WHERE id = $1
	AND deleted_at IS NULL`

	var campaign CampaignLookup
	err := s.db.GetContext(ctx, &campaign, query, campaignID)
	if err != nil {
		return nil, err
	}

	return &campaign, nil
}

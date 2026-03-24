package subjects

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
)

// CampaignSend represents a row in the campaign_sends table.
type CampaignSend struct {
	ID            uuid.UUID  `db:"id"`
	CampaignID    uuid.UUID  `db:"campaign_id"`
	UserID        uuid.UUID  `db:"user_id"`
	State         *string    `db:"state"`
	SentAt        *time.Time `db:"sent_at"`
	OpenedAt      *time.Time `db:"opened_at"`
	Clicks        int        `db:"clicks"`
	ReferenceType *string    `db:"reference_type"`
	ReferenceID   string     `db:"reference_id"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

func NewCampaignSendsStore(db store.DB) *CampaignSendsStore {
	return &CampaignSendsStore{db: db}
}

// CampaignSendsStore provides database operations for the campaign_sends table.
type CampaignSendsStore struct {
	db store.DB
}

// LookupCampaignSendByReference finds a campaign send by its reference_type and reference_id.
// This is used by the webhook consumer to resolve the user_id and campaign_id
// from a provider's message ID.
func (s *CampaignSendsStore) LookupCampaignSendByReference(ctx context.Context, referenceType, referenceID string) (*CampaignSend, error) {
	query := `
	SELECT id, campaign_id, user_id, state, sent_at, opened_at, clicks,
	       reference_type, reference_id, created_at, updated_at
	FROM campaign_sends
	WHERE reference_type = $1
	  AND reference_id = $2
	LIMIT 1`

	var send CampaignSend
	err := s.db.GetContext(ctx, &send, query, referenceType, referenceID)
	if err != nil {
		return nil, err
	}

	return &send, nil
}

// UpdateCampaignSendDelivered sets the state to "delivered" and records the delivery timestamp.
func (s *CampaignSendsStore) UpdateCampaignSendDelivered(ctx context.Context, referenceType, referenceID string) error {
	stmt := `
	UPDATE campaign_sends
	SET state = 'delivered'
	WHERE reference_type = $1
	  AND reference_id = $2`

	_, err := s.db.ExecContext(ctx, stmt, referenceType, referenceID)
	return err
}

// UpdateCampaignSendBounced sets the state to "bounced".
func (s *CampaignSendsStore) UpdateCampaignSendBounced(ctx context.Context, referenceType, referenceID string) error {
	stmt := `
	UPDATE campaign_sends
	SET state = 'bounced'
	WHERE reference_type = $1
	  AND reference_id = $2`

	_, err := s.db.ExecContext(ctx, stmt, referenceType, referenceID)
	return err
}

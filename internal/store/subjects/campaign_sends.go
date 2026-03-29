package subjects

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/store"
)

// CampaignSend represents a row in the campaign_sends table.
type CampaignSend struct {
	ID            uuid.UUID  `db:"id"`
	CampaignID    uuid.UUID  `db:"campaign_id"`
	UserID        uuid.UUID  `db:"user_id"`
	BroadcastID   *uuid.UUID `db:"broadcast_id"`
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

// CountSendsByBroadcastIDs returns the number of campaign_sends rows for each
// of the provided broadcast IDs. Rows with an empty state are excluded so the
// result reflects messages that were actually processed by the provider.
func (s *CampaignSendsStore) CountSendsByBroadcastIDs(ctx context.Context, broadcastIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	if len(broadcastIDs) == 0 {
		return map[uuid.UUID]int{}, nil
	}

	query := `
	SELECT broadcast_id, COUNT(*) AS cnt
	FROM campaign_sends
	WHERE broadcast_id = ANY($1)
	AND state != ''
	GROUP BY broadcast_id`

	var rows []struct {
		BroadcastID uuid.UUID `db:"broadcast_id"`
		Count       int       `db:"cnt"`
	}
	err := s.db.SelectContext(ctx, &rows, query, pq.Array(broadcastIDs))
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]int, len(rows))
	for _, r := range rows {
		result[r.BroadcastID] = r.Count
	}

	return result, nil
}

// CountSendsByBroadcastID returns the number of campaign_sends rows for a
// single broadcast. Rows with an empty state are excluded.
func (s *CampaignSendsStore) CountSendsByBroadcastID(ctx context.Context, broadcastID uuid.UUID) (int, error) {
	query := `
	SELECT COUNT(*)
	FROM campaign_sends
	WHERE broadcast_id = $1
	AND state != ''`

	var count int
	err := s.db.GetContext(ctx, &count, query, broadcastID)
	return count, err
}

// InsertCampaignSend creates a new campaign_sends row. The reference_id is used
// as part of the composite primary key — pass the provider response ID when
// available, or a generated UUID otherwise.
func (s *CampaignSendsStore) InsertCampaignSend(ctx context.Context, send CampaignSend) error {
	stmt := `
	INSERT INTO campaign_sends (campaign_id, user_id, broadcast_id, state, sent_at, reference_type, reference_id)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT (campaign_id, user_id, reference_id) DO NOTHING`

	_, err := s.db.ExecContext(ctx, stmt, send.CampaignID, send.UserID, send.BroadcastID, send.State, send.SentAt, send.ReferenceType, send.ReferenceID)
	return err
}

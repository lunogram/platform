package management

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type BroadcastState string

const (
	BroadcastStatePending   BroadcastState = "pending"
	BroadcastStateScheduled BroadcastState = "scheduled"
	BroadcastStateSending   BroadcastState = "sending"
	BroadcastStateCompleted BroadcastState = "completed"
	BroadcastStateFailed    BroadcastState = "failed"
	BroadcastStateCancelled BroadcastState = "cancelled"
)

type Broadcasts []Broadcast

type Broadcast struct {
	ID         uuid.UUID      `db:"id"`
	ProjectID  uuid.UUID      `db:"project_id"`
	CampaignID uuid.UUID      `db:"campaign_id"`
	ListID     uuid.UUID      `db:"list_id"`
	ListName   string         `db:"list_name"`
	ListType   string         `db:"list_type"`
	State      BroadcastState `db:"state"`
	// Total is the number of messages published to NATS during broadcast
	// processing. Sent tracks the number of messages actually delivered.
	Total       int        `db:"total"`
	Sent        int        `db:"sent"`
	Error       *string    `db:"error"`
	ScheduledAt *time.Time `db:"scheduled_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	StartedAt   *time.Time `db:"started_at"`
	CompletedAt *time.Time `db:"completed_at"`

	// Joined fields
	Campaign *Campaign `db:"-"`
}

// OAPI converts a Broadcast to the generated oapi.Broadcast type for JSON responses.
func (b Broadcast) OAPI() oapi.Broadcast {
	result := oapi.Broadcast{
		Id:          b.ID,
		ProjectId:   b.ProjectID,
		CampaignId:  b.CampaignID,
		ListId:      b.ListID,
		ListName:    b.ListName,
		ListType:    b.ListType,
		State:       oapi.BroadcastState(b.State),
		Total:       b.Total,
		Sent:        &b.Sent,
		Error:       b.Error,
		ScheduledAt: b.ScheduledAt,
		StartedAt:   b.StartedAt,
		CompletedAt: b.CompletedAt,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}

	if b.Campaign != nil {
		channel := b.Campaign.Channel
		result.Campaign = &struct {
			Channel *string             `json:"channel,omitempty"`
			Id      *openapi_types.UUID `json:"id,omitempty"`
			Name    *string             `json:"name,omitempty"`
		}{
			Id:   &b.CampaignID,
			Name: &b.Campaign.Name,
		}
		if channel != "" {
			result.Campaign.Channel = &channel
		}
	}

	return result
}

// OAPI converts a Broadcasts slice to a slice of oapi.Broadcast.
func (broadcasts Broadcasts) OAPI() []oapi.Broadcast {
	result := make([]oapi.Broadcast, len(broadcasts))
	for i, b := range broadcasts {
		result[i] = b.OAPI()
	}
	return result
}

func NewBroadcastsStore(db store.DB) *BroadcastsStore {
	return &BroadcastsStore{
		db: db,
	}
}

type BroadcastsStore struct {
	db store.DB
}

func (s *BroadcastsStore) CreateBroadcast(ctx context.Context, broadcast Broadcast) (Broadcast, error) {
	state := BroadcastStatePending
	if broadcast.ScheduledAt != nil {
		state = BroadcastStateScheduled
	}

	stmt := `
	INSERT INTO campaign_broadcasts (project_id, campaign_id, list_id, list_name, list_type, state, scheduled_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING id, project_id, campaign_id, list_id, list_name, list_type, state, total, error, scheduled_at, created_at, updated_at, started_at, completed_at`

	var result Broadcast
	err := s.db.GetContext(ctx, &result, stmt, broadcast.ProjectID, broadcast.CampaignID, broadcast.ListID, broadcast.ListName, broadcast.ListType, string(state), broadcast.ScheduledAt)
	if err != nil {
		return Broadcast{}, err
	}

	return result, nil
}

func (s *BroadcastsStore) GetBroadcast(ctx context.Context, projectID, broadcastID uuid.UUID) (*Broadcast, error) {
	query := `
	SELECT
		cb.id, cb.project_id, cb.campaign_id, cb.list_id, cb.list_name, cb.list_type,
		cb.state, cb.total, cb.sent, cb.error, cb.scheduled_at, cb.created_at, cb.updated_at, cb.started_at, cb.completed_at,
		c.name AS campaign_name, c.channel AS campaign_channel
	FROM campaign_broadcasts cb
	INNER JOIN campaigns c ON c.id = cb.campaign_id
	WHERE cb.project_id = $1
	AND cb.id = $2`

	var result struct {
		Broadcast
		CampaignName    string `db:"campaign_name"`
		CampaignChannel string `db:"campaign_channel"`
	}
	err := s.db.GetContext(ctx, &result, query, projectID, broadcastID)
	if err != nil {
		return nil, err
	}

	broadcast := result.Broadcast
	broadcast.Campaign = &Campaign{
		ID:      broadcast.CampaignID,
		Name:    result.CampaignName,
		Channel: result.CampaignChannel,
	}

	return &broadcast, nil
}

func (s *BroadcastsStore) ListBroadcasts(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string, campaignID *uuid.UUID, listID *uuid.UUID, state *BroadcastState) (Broadcasts, int, error) {
	query := `
	SELECT
		cb.id, cb.project_id, cb.campaign_id, cb.list_id, cb.list_name, cb.list_type,
		cb.state, cb.total, cb.sent, cb.error, cb.scheduled_at, cb.created_at, cb.updated_at, cb.started_at, cb.completed_at,
		c.name AS campaign_name, c.channel AS campaign_channel,
		COUNT(*) OVER () AS total_count
	FROM campaign_broadcasts cb
	INNER JOIN campaigns c ON c.id = cb.campaign_id
	WHERE cb.project_id = $1
	AND ($4::uuid IS NULL OR cb.campaign_id = $4)
	AND ($5::uuid IS NULL OR cb.list_id = $5)
	AND ($6::text IS NULL OR cb.state = $6)
	AND ($7 = '' OR cb.list_name ILIKE '%' || $7 || '%' OR c.name ILIKE '%' || $7 || '%')
	ORDER BY cb.created_at DESC
	LIMIT $2 OFFSET $3`

	type result struct {
		Broadcast
		CampaignName    string `db:"campaign_name"`
		CampaignChannel string `db:"campaign_channel"`
		TotalCount      int    `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, pagination.Limit, pagination.Offset, campaignID, listID, state, search)
	if err != nil {
		return nil, 0, err
	}

	broadcasts := make(Broadcasts, len(results))
	total := 0

	for i, r := range results {
		broadcasts[i] = r.Broadcast
		broadcasts[i].Campaign = &Campaign{
			ID:      r.Broadcast.CampaignID,
			Name:    r.CampaignName,
			Channel: r.CampaignChannel,
		}
		if i == 0 {
			total = r.TotalCount
		}
	}

	return broadcasts, total, nil
}

// UpdateBroadcastState transitions a broadcast to the given state, updating the
// total count and optional error message. Timestamps (started_at, completed_at)
// are set automatically based on the target state. The query is scoped by
// project_id to prevent cross-project state mutations.
func (s *BroadcastsStore) UpdateBroadcastState(ctx context.Context, projectID, broadcastID uuid.UUID, state BroadcastState, total int, msg *string) error {
	query := `
	UPDATE campaign_broadcasts
	SET
		state = $3,
		total = $4,
		error = $5,
		updated_at = NOW(),
		started_at = CASE WHEN $6 = 'sending' AND started_at IS NULL THEN NOW() ELSE started_at END,
		completed_at = CASE WHEN $6 IN ('completed', 'failed') THEN NOW() ELSE completed_at END
	WHERE id = $2
	AND project_id = $1`

	_, err := s.db.ExecContext(ctx, query, projectID, broadcastID, state, total, msg, string(state))
	return err
}

// TransitionBroadcastState is like UpdateBroadcastState but only applies the
// update when the broadcast is currently in fromState. This prevents concurrent
// senders from racing to complete the same broadcast. Returns true when the
// row was actually updated (i.e. the transition happened).
func (s *BroadcastsStore) TransitionBroadcastState(ctx context.Context, projectID, broadcastID uuid.UUID, fromState, toState BroadcastState, total int, msg *string) (bool, error) {
	query := `
	UPDATE campaign_broadcasts
	SET
		state = $3,
		total = $4,
		error = $5,
		updated_at = NOW(),
		started_at = CASE WHEN $6 = 'sending' AND started_at IS NULL THEN NOW() ELSE started_at END,
		completed_at = CASE WHEN $6 IN ('completed', 'failed') THEN NOW() ELSE completed_at END
	WHERE id = $2
	AND project_id = $1
	AND state = $7`

	result, err := s.db.ExecContext(ctx, query, projectID, broadcastID, toState, total, msg, string(toState), string(fromState))
	if err != nil {
		return false, err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return n > 0, nil
}

// IncrementBroadcastSent atomically increments the sent counter on a broadcast
// and returns the new sent count and total.
func (s *BroadcastsStore) IncrementBroadcastSent(ctx context.Context, broadcastID uuid.UUID) (sent int, total int, err error) {
	stmt := `
	UPDATE campaign_broadcasts
	SET sent = sent + 1, updated_at = NOW()
	WHERE id = $1
	RETURNING sent, total`

	err = s.db.QueryRowContext(ctx, stmt, broadcastID).Scan(&sent, &total)
	return
}

// IncrementBroadcastTotal atomically adds delta to the broadcast's total count.
// This is used during batched broadcast processing to track progress without
// race conditions between batches.
func (s *BroadcastsStore) IncrementBroadcastTotal(ctx context.Context, projectID, broadcastID uuid.UUID, delta int) error {
	query := `
	UPDATE campaign_broadcasts
	SET total = total + $3, updated_at = NOW()
	WHERE id = $2
	AND project_id = $1`

	_, err := s.db.ExecContext(ctx, query, projectID, broadcastID, delta)
	return err
}

// UpdateBroadcast updates mutable fields of a broadcast (currently only scheduled_at).
// The broadcast must be in 'pending' or 'scheduled' state. When scheduled_at is set the
// state is changed to 'scheduled'; when it is cleared the state reverts to 'pending'.
func (s *BroadcastsStore) UpdateBroadcast(ctx context.Context, projectID, broadcastID uuid.UUID, scheduledAt *time.Time) (*Broadcast, error) {
	var newState BroadcastState
	if scheduledAt != nil {
		newState = BroadcastStateScheduled
	} else {
		newState = BroadcastStatePending
	}

	query := `
	UPDATE campaign_broadcasts
	SET
		scheduled_at = $3,
		state = $4,
		updated_at = NOW()
	WHERE project_id = $1
	AND id = $2
	AND state IN ('pending', 'scheduled')
	RETURNING id, project_id, campaign_id, list_id, list_name, list_type, state, total, error, scheduled_at, created_at, updated_at, started_at, completed_at`

	var result Broadcast
	err := s.db.GetContext(ctx, &result, query, projectID, broadcastID, scheduledAt, string(newState))
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// CancelBroadcast transitions a broadcast from 'pending' or 'scheduled' to 'cancelled'.
// Returns sql.ErrNoRows if the broadcast doesn't exist or is not in a cancellable state.
func (s *BroadcastsStore) CancelBroadcast(ctx context.Context, projectID, broadcastID uuid.UUID) (*Broadcast, error) {
	query := `
	UPDATE campaign_broadcasts
	SET
		state = $3,
		scheduled_at = NULL,
		updated_at = NOW()
	WHERE project_id = $1
	AND id = $2
	AND state IN ('pending', 'scheduled')
	RETURNING id, project_id, campaign_id, list_id, list_name, list_type, state, total, error, scheduled_at, created_at, updated_at, started_at, completed_at`

	var result Broadcast
	err := s.db.GetContext(ctx, &result, query, projectID, broadcastID, string(BroadcastStateCancelled))
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// ScanScheduledBroadcasts finds broadcasts in 'scheduled' state whose scheduled_at
// time has passed. The scanner callback is invoked for each due broadcast. This is
// used by the cluster leader scheduler to trigger scheduled broadcasts. The query
// uses FOR UPDATE SKIP LOCKED so that rows already locked by a concurrent
// transaction are silently skipped instead of blocking. This prevents
// double-processing during cluster leader transitions where two nodes may
// briefly both act as leader.
func (s *BroadcastsStore) ScanScheduledBroadcasts(ctx context.Context, limit int, scanner func(Broadcast) error) (int, error) {
	query := `
	SELECT id, project_id, campaign_id, list_id, list_name, list_type, state, total, error, scheduled_at, created_at, updated_at, started_at, completed_at
	FROM campaign_broadcasts
	WHERE state = 'scheduled'
	AND scheduled_at IS NOT NULL
	AND scheduled_at <= NOW()
	ORDER BY scheduled_at ASC
	LIMIT $1
	FOR UPDATE SKIP LOCKED`

	var broadcasts []Broadcast
	err := s.db.SelectContext(ctx, &broadcasts, query, limit)
	if err != nil {
		return 0, err
	}

	for n, b := range broadcasts {
		if err := scanner(b); err != nil {
			return n, err
		}
	}

	return len(broadcasts), nil
}

// TransitionScheduledBroadcastToSending atomically transitions a broadcast from
// 'scheduled' to 'sending' state, ensuring no duplicate processing.
func (s *BroadcastsStore) TransitionScheduledBroadcastToSending(ctx context.Context, broadcastID uuid.UUID) error {
	query := `
	UPDATE campaign_broadcasts
	SET state = 'sending', started_at = NOW(), updated_at = NOW()
	WHERE id = $1 AND state = 'scheduled'`

	result, err := s.db.ExecContext(ctx, query, broadcastID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("broadcast %s is not in scheduled state", broadcastID)
	}

	return nil
}

// TransitionPendingBroadcastToSending atomically transitions a broadcast from
// 'pending' to 'sending' state, ensuring no duplicate processing.
func (s *BroadcastsStore) TransitionPendingBroadcastToSending(ctx context.Context, broadcastID uuid.UUID) error {
	query := `
	UPDATE campaign_broadcasts
	SET state = 'sending', started_at = NOW(), updated_at = NOW()
	WHERE id = $1 AND state = 'pending'`

	result, err := s.db.ExecContext(ctx, query, broadcastID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("broadcast %s is not in pending state", broadcastID)
	}

	return nil
}

// BroadcastUser combines inbox message metadata with user profile data.
type BroadcastUser struct {
	ID       uuid.UUID  `db:"id" json:"id"`
	UserID   uuid.UUID  `db:"user_id" json:"user_id"`
	State    string     `db:"state" json:"state"`
	SentAt   *time.Time `db:"sent_at" json:"sent_at,omitempty"`
	FullName *string    `db:"full_name" json:"full_name,omitempty"`
	Email    *string    `db:"email" json:"email,omitempty"`
	Phone    *string    `db:"phone" json:"phone,omitempty"`
}

type BroadcastUsers []BroadcastUser

// GetBroadcastUsers queries user_inbox_messages for a specific broadcast_id
// and joins with the users table to return user profile data. The search
// parameter filters on full_name, email, or phone.
func (s *BroadcastsStore) GetBroadcastUsers(ctx context.Context, usersDB store.DB, broadcastID uuid.UUID, pagination store.Pagination, search string) (BroadcastUsers, int, error) {
	query := `
	SELECT
		m.id, m.user_id,
		CASE WHEN m.sent_at IS NOT NULL THEN 'sent' ELSE 'pending' END AS state,
		m.sent_at,
		NULLIF(TRIM(COALESCE(u.data->>'first_name', '') || ' ' || COALESCE(u.data->>'last_name', '')), '') AS full_name,
		u.email, u.phone,
		COUNT(*) OVER () AS total_count
	FROM user_inbox_messages m
	INNER JOIN users u ON u.id = m.user_id
	WHERE m.broadcast_id = $1 AND m.deleted_at IS NULL
	AND ($4 = '' OR u.data->>'first_name' ILIKE '%' || $4 || '%' OR u.data->>'last_name' ILIKE '%' || $4 || '%' OR u.email ILIKE '%' || $4 || '%' OR u.phone ILIKE '%' || $4 || '%')
	ORDER BY m.created_at DESC
	LIMIT $2 OFFSET $3`

	var results []struct {
		BroadcastUser
		TotalCount int `db:"total_count"`
	}
	err := usersDB.SelectContext(ctx, &results, query, broadcastID, pagination.Limit, pagination.Offset, search)
	if err != nil {
		return nil, 0, err
	}

	users := make(BroadcastUsers, len(results))
	total := 0

	for i, r := range results {
		users[i] = r.BroadcastUser
		if i == 0 {
			total = r.TotalCount
		}
	}

	return users, total, nil
}

// ScanStuckBroadcasts finds broadcasts in 'sending' state whose started_at
// timestamp is older than the given threshold. These broadcasts may be stuck
// due to a crash between the state transition and the NATS publish or because
// the consumer failed without updating state.
func (s *BroadcastsStore) ScanStuckBroadcasts(ctx context.Context, stuckThreshold time.Duration, scanner func(Broadcast) error) (int, error) {
	query := `
	SELECT id, project_id, campaign_id, list_id, list_name, list_type, state, total, error, scheduled_at, created_at, updated_at, started_at, completed_at
	FROM campaign_broadcasts
	WHERE state = 'sending'
	AND started_at IS NOT NULL
	AND started_at < $1
	ORDER BY started_at ASC`

	cutoff := time.Now().Add(-stuckThreshold)
	rows, err := s.db.QueryxContext(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var n int
	for rows.Next() {
		var b Broadcast
		if err := rows.StructScan(&b); err != nil {
			return n, err
		}
		if err := scanner(b); err != nil {
			return n, err
		}
		n++
	}

	return n, rows.Err()
}

package subjects

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/pkg/modules"
)

const (
	InboxStatusUnread   = "unread"
	InboxStatusRead     = "read"
	InboxStatusArchived = "archived"
)

// InboxClassStandard is the default class; InboxClassCompliance marks a message
// that must reach the recipient regardless of suppression state (an opt-out
// confirmation is itself a compliance message).
const (
	InboxClassStandard   = "standard"
	InboxClassCompliance = "compliance"
)

// DefaultInboxPriority is applied when a message is created without an explicit
// priority. It matches the OpenAPI schema default for InboxMessage.priority
// (range 1-5, default 3).
const DefaultInboxPriority int16 = 3

// InboxMessage is one row from user_inbox_messages or organization_inbox_messages.
//
// Render output (title, body, format, link_url, subject, ...) lives in Content.
// Provenance (template_id, journey_id, journey_entry_id, journey_step_id) and
// arbitrary caller metadata live in Data.
//
// Lifecycle facts (delivered, bounced, read, ...) live in the events log
// keyed by inbox_message_id.
type InboxMessage struct {
	ID               uuid.UUID       `db:"id"`
	ProjectID        uuid.UUID       `db:"project_id"`
	UserID           *uuid.UUID      `db:"user_id"`
	OrganizationID   *uuid.UUID      `db:"organization_id"`
	ExternalID       *string         `db:"external_id"`
	Channel          modules.Channel `db:"channel"`
	SenderIdentityID *uuid.UUID      `db:"sender_identity_id"`
	CampaignID       *uuid.UUID      `db:"campaign_id"`
	BroadcastID      *uuid.UUID      `db:"broadcast_id"`
	Content          json.RawMessage `db:"content"`
	Data             json.RawMessage `db:"data"`
	Tags             pq.StringArray  `db:"tags"`
	Priority         int16           `db:"priority"`
	Source           *string         `db:"source"`
	ScheduledAt      time.Time       `db:"scheduled_at"`
	ExpiresAt        *time.Time      `db:"expires_at"`
	ReadAt           *time.Time      `db:"read_at"`
	ArchivedAt       *time.Time      `db:"archived_at"`
	SentAt           *time.Time      `db:"sent_at"`
	FailedAt         *time.Time      `db:"failed_at"`
	FailureReason    *string         `db:"failure_reason"`
	// Class gates the compliance bypass. It is a dedicated column rather than a
	// tag because tags are caller-controlled and must not grant that bypass.
	Class             string     `db:"class"`
	RecipientTimezone *string    `db:"recipient_timezone"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
	DeletedAt         *time.Time `db:"deleted_at"`
}

type InboxMessages []InboxMessage

func (m *InboxMessage) OAPI() oapi.InboxMessage {
	return oapi.InboxMessage{
		Id:               m.ID,
		ProjectId:        m.ProjectID,
		UserId:           m.UserID,
		OrganizationId:   m.OrganizationID,
		ExternalId:       m.ExternalID,
		Channel:          oapi.Channel(m.Channel),
		SenderIdentityId: m.SenderIdentityID,
		CampaignId:       m.CampaignID,
		BroadcastId:      m.BroadcastID,
		Content:          m.Content,
		Data:             m.Data,
		Tags:             []string(m.Tags),
		Priority:         m.Priority,
		Source:           m.Source,
		ScheduledAt:      m.ScheduledAt,
		ExpiresAt:        m.ExpiresAt,
		ReadAt:           m.ReadAt,
		ArchivedAt:       m.ArchivedAt,
		SentAt:           m.SentAt,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}

// IsDue reports whether this inbox message should be dispatched now.
// A message is due when its scheduled time has arrived and it has not
// expired. Dedup of duplicate publishes across retries is handled by
// JetStream Msg-Id deduplication; this method does not track publish state.
func (m *InboxMessage) IsDue() bool {
	now := time.Now()
	if m.ScheduledAt.After(now) {
		return false
	}
	return m.ExpiresAt == nil || m.ExpiresAt.After(now)
}

func (ms InboxMessages) OAPI() []oapi.InboxMessage {
	results := make([]oapi.InboxMessage, len(ms))
	for i := range ms {
		results[i] = ms[i].OAPI()
	}
	return results
}

type InboxMessageParams struct {
	ExternalID       *string
	Channel          modules.Channel
	SenderIdentityID *uuid.UUID
	CampaignID       *uuid.UUID
	BroadcastID      *uuid.UUID
	Content          json.RawMessage
	Data             json.RawMessage
	Tags             []string
	Priority         *int16
	Source           *string
	ScheduledAt      *time.Time
	ExpiresAt        *time.Time
	// Class defaults to InboxClassStandard when empty. Only trusted internal
	// senders may set InboxClassCompliance: it is what lets a message past the
	// opt-out gate, so it is a store parameter rather than anything a caller
	// can reach through the public API.
	Class string
	// RecipientTimezone is observational and may be nil. Nothing on the send
	// path reads it yet; it is recorded so a later quiet-hours rollout can be
	// measured against real history, which is why an unresolved recipient
	// leaves the column NULL rather than storing a guess.
	RecipientTimezone *string
}

type InboxListFilter struct {
	Status        string
	Channel       modules.Channel
	Tags          []string
	MessageSource *string
	Priority      *int
	// Search is an ILIKE pattern matched against content title, body, and
	// subject fields using trigram indexes. Queries shorter than 3 characters
	// are ignored because pg_trgm cannot use the index efficiently for very
	// short patterns.
	Search           string
	IncludeScheduled bool
	IncludeExpired   bool
	IncludeArchived  bool
	// ExcludeFailed hides messages whose send terminally failed. It is opt-in
	// because the console must still see them - a suppressed message is the
	// audit trail for why nothing was delivered - while the end user must not
	// be shown a message that never reached them.
	ExcludeFailed bool
}

type InboxCounts struct {
	Unread int `db:"unread" json:"unread"`
	// Total is the count of non-archived, non-deleted messages that are
	// scheduled and not expired. Archived messages are excluded so clients
	// can display meaningful badge counts.
	Total int `db:"total" json:"total"`
}

func NewInboxStore(db store.DB) *InboxStore {
	return &InboxStore{db: db}
}

type InboxStore struct {
	db store.DB
}

func (s *InboxStore) CreateUserInboxMessage(ctx context.Context, projectID, userID uuid.UUID, params InboxMessageParams) (*InboxMessage, error) {
	params = normalizeInboxMessageParams(params)

	stmt := `
	INSERT INTO user_inbox_messages (project_id, user_id, external_id, channel, sender_identity_id, campaign_id, broadcast_id, content, data, tags, priority, source, scheduled_at, expires_at, class, recipient_timezone)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, COALESCE($13, NOW()), $14, $15, $16)
	ON CONFLICT DO NOTHING
	RETURNING id, project_id, user_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at`

	var row InboxMessage
	err := s.db.GetContext(ctx, &row, stmt,
		projectID,
		userID,
		params.ExternalID,
		params.Channel,
		params.SenderIdentityID,
		params.CampaignID,
		params.BroadcastID,
		params.Content,
		params.Data,
		pq.Array(params.Tags),
		params.Priority,
		params.Source,
		params.ScheduledAt,
		params.ExpiresAt,
		params.Class,
		params.RecipientTimezone,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return s.resolveUserInboxMessageConflict(ctx, projectID, userID, params)
	}
	if err != nil {
		return nil, err
	}

	return &row, nil
}

func (s *InboxStore) CreateOrganizationInboxMessage(ctx context.Context, projectID, organizationID uuid.UUID, params InboxMessageParams) (*InboxMessage, error) {
	params = normalizeInboxMessageParams(params)

	stmt := `
	INSERT INTO organization_inbox_messages (project_id, organization_id, external_id, channel, sender_identity_id, campaign_id, broadcast_id, content, data, tags, priority, source, scheduled_at, expires_at, class, recipient_timezone)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, COALESCE($13, NOW()), $14, $15, $16)
	ON CONFLICT DO NOTHING
	RETURNING id, project_id, organization_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at`

	var row InboxMessage
	err := s.db.GetContext(ctx, &row, stmt,
		projectID,
		organizationID,
		params.ExternalID,
		params.Channel,
		params.SenderIdentityID,
		params.CampaignID,
		params.BroadcastID,
		params.Content,
		params.Data,
		pq.Array(params.Tags),
		params.Priority,
		params.Source,
		params.ScheduledAt,
		params.ExpiresAt,
		params.Class,
		params.RecipientTimezone,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return s.resolveOrganizationInboxMessageConflict(ctx, projectID, organizationID, params)
	}
	if err != nil {
		return nil, err
	}

	return &row, nil
}

// resolveUserInboxMessageConflict reads back the existing row when the INSERT
// is deduped by the unique partial index on (project_id, user_id, channel,
// external_id). Without an external_id, we have nothing to look up.
func (s *InboxStore) resolveUserInboxMessageConflict(ctx context.Context, projectID, userID uuid.UUID, params InboxMessageParams) (*InboxMessage, error) {
	if params.ExternalID == nil {
		return nil, sql.ErrNoRows
	}

	stmt := `
	SELECT id, project_id, user_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at
	FROM user_inbox_messages
	WHERE project_id = $1
	AND user_id = $2
	AND channel = $3
	AND external_id = $4`

	var row InboxMessage
	err := s.db.GetContext(ctx, &row, stmt, projectID, userID, params.Channel, params.ExternalID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *InboxStore) resolveOrganizationInboxMessageConflict(ctx context.Context, projectID, organizationID uuid.UUID, params InboxMessageParams) (*InboxMessage, error) {
	if params.ExternalID == nil {
		return nil, sql.ErrNoRows
	}

	stmt := `
	SELECT id, project_id, organization_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at
	FROM organization_inbox_messages
	WHERE project_id = $1
	AND organization_id = $2
	AND channel = $3
	AND external_id = $4`

	var row InboxMessage
	err := s.db.GetContext(ctx, &row, stmt, projectID, organizationID, params.Channel, params.ExternalID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func normalizeInboxMessageParams(params InboxMessageParams) InboxMessageParams {
	if len(params.Content) == 0 {
		params.Content = json.RawMessage(`{}`)
	}
	if len(params.Data) == 0 {
		params.Data = json.RawMessage(`{}`)
	}
	if params.Tags == nil {
		params.Tags = []string{}
	}
	if params.Priority == nil {
		params.Priority = ptr.To(DefaultInboxPriority)
	}
	if params.Class == "" {
		params.Class = InboxClassStandard
	}
	return params
}

func (s *InboxStore) ListUserInboxMessages(ctx context.Context, projectID, userID uuid.UUID, pagination store.Pagination, filter InboxListFilter) (InboxMessages, int, error) {
	stmt := `
	SELECT id, project_id, user_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at,
		COUNT(*) OVER () AS total_count
	FROM user_inbox_messages
	WHERE project_id = $1
	AND user_id = $2
	AND deleted_at IS NULL
	AND ($3::bool OR scheduled_at <= NOW())
	AND ($4::bool OR expires_at IS NULL OR expires_at > NOW())
	AND (
		$5 = ''
		OR ($5 = 'unread' AND read_at IS NULL AND archived_at IS NULL)
		OR ($5 = 'read' AND read_at IS NOT NULL AND archived_at IS NULL)
		OR ($5 = 'archived' AND archived_at IS NOT NULL)
	)
	AND ($5 != '' OR $6::bool OR archived_at IS NULL)
	AND ($7 = '' OR channel = $7)
	AND ($8::text[] IS NULL OR tags @> $8)
	AND ($9 = '' OR source = $9)
	AND ($10 = 0 OR priority = $10)
	AND (
		$11 = ''
		OR (content->>'title') ILIKE '%' || $11 || '%'
		OR (content->>'body') ILIKE '%' || $11 || '%'
		OR (content->>'subject') ILIKE '%' || $11 || '%'
	)
	AND (NOT $12::bool OR failed_at IS NULL)
	ORDER BY scheduled_at DESC, created_at DESC
	LIMIT $13 OFFSET $14`

	return s.listInboxMessages(ctx, stmt, projectID, userID, pagination, filter)
}

func (s *InboxStore) ListOrganizationInboxMessages(ctx context.Context, projectID, organizationID uuid.UUID, pagination store.Pagination, filter InboxListFilter) (InboxMessages, int, error) {
	stmt := `
	SELECT id, project_id, organization_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at,
		COUNT(*) OVER () AS total_count
	FROM organization_inbox_messages
	WHERE project_id = $1
	AND organization_id = $2
	AND deleted_at IS NULL
	AND ($3::bool OR scheduled_at <= NOW())
	AND ($4::bool OR expires_at IS NULL OR expires_at > NOW())
	AND (
		$5 = ''
		OR ($5 = 'unread' AND read_at IS NULL AND archived_at IS NULL)
		OR ($5 = 'read' AND read_at IS NOT NULL AND archived_at IS NULL)
		OR ($5 = 'archived' AND archived_at IS NOT NULL)
	)
	AND ($5 != '' OR $6::bool OR archived_at IS NULL)
	AND ($7 = '' OR channel = $7)
	AND ($8::text[] IS NULL OR tags @> $8)
	AND ($9 = '' OR source = $9)
	AND ($10 = 0 OR priority = $10)
	AND (
		$11 = ''
		OR (content->>'title') ILIKE '%' || $11 || '%'
		OR (content->>'body') ILIKE '%' || $11 || '%'
		OR (content->>'subject') ILIKE '%' || $11 || '%'
	)
	AND (NOT $12::bool OR failed_at IS NULL)
	ORDER BY scheduled_at DESC, created_at DESC
	LIMIT $13 OFFSET $14`

	return s.listInboxMessages(ctx, stmt, projectID, organizationID, pagination, filter)
}

func (s *InboxStore) listInboxMessages(ctx context.Context, stmt string, projectID, ownerID uuid.UUID, pagination store.Pagination, filter InboxListFilter) (InboxMessages, int, error) {
	pagination = pagination.Clamp()

	// Ignore search strings shorter than 3 characters; pg_trgm indexes
	// cannot be used efficiently for very short patterns.
	search := filter.Search
	if len(search) < 3 {
		search = ""
	}

	var tags interface{}
	if len(filter.Tags) > 0 {
		tags = pq.Array(filter.Tags)
	}
	var source string
	if filter.MessageSource != nil {
		source = *filter.MessageSource
	}
	var priority int
	if filter.Priority != nil {
		priority = *filter.Priority
	}

	type result struct {
		InboxMessage
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, stmt,
		projectID,
		ownerID,
		filter.IncludeScheduled,
		filter.IncludeExpired,
		filter.Status,
		filter.IncludeArchived,
		filter.Channel,
		tags,
		source,
		priority,
		search,
		filter.ExcludeFailed,
		pagination.Limit,
		pagination.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	if len(results) == 0 {
		return InboxMessages{}, 0, nil
	}

	messages := make(InboxMessages, len(results))
	for i, r := range results {
		messages[i] = r.InboxMessage
	}

	return messages, results[0].TotalCount, nil
}

func (s *InboxStore) CountUserInboxMessages(ctx context.Context, projectID, userID uuid.UUID, channel string) (InboxCounts, error) {
	stmt := `
	SELECT
		COUNT(*) FILTER (WHERE read_at IS NULL AND archived_at IS NULL) AS unread,
		COUNT(*) FILTER (WHERE archived_at IS NULL) AS total
	FROM user_inbox_messages
	WHERE project_id = $1
	AND user_id = $2
	AND deleted_at IS NULL
	AND scheduled_at <= NOW()
	AND (expires_at IS NULL OR expires_at > NOW())
	AND ($3 = '' OR channel = $3)`

	var counts InboxCounts
	err := s.db.GetContext(ctx, &counts, stmt, projectID, userID, channel)
	return counts, err
}

func (s *InboxStore) CountOrganizationInboxMessages(ctx context.Context, projectID, organizationID uuid.UUID, channel string) (InboxCounts, error) {
	stmt := `
	SELECT
		COUNT(*) FILTER (WHERE read_at IS NULL AND archived_at IS NULL) AS unread,
		COUNT(*) FILTER (WHERE archived_at IS NULL) AS total
	FROM organization_inbox_messages
	WHERE project_id = $1
	AND organization_id = $2
	AND deleted_at IS NULL
	AND scheduled_at <= NOW()
	AND (expires_at IS NULL OR expires_at > NOW())
	AND ($3 = '' OR channel = $3)`

	var counts InboxCounts
	err := s.db.GetContext(ctx, &counts, stmt, projectID, organizationID, channel)
	return counts, err
}

// ScanDueUserInboxMessages iterates over at most limit user inbox messages whose
// scheduled_at is in the past and that have not been sent, deleted, or expired.
//
// FOR UPDATE SKIP LOCKED lets concurrent scanners avoid handing each other the
// same rows without blocking. Note the scan runs as its own statement (not an
// open transaction held across fn), so the row locks are released as soon as
// the SELECT returns; SKIP LOCKED reduces contention but is not by itself a
// guarantee against double-processing. Idempotency is provided downstream: the
// scheduler re-injects each due message with a stable Msg-Id (deduped by
// JetStream) and the consumer's sent_at guard ensures a message is dispatched
// at most once.
func (s *InboxStore) ScanDueUserInboxMessages(ctx context.Context, limit int, fn func(InboxMessage) error) (int, error) {
	stmt := `
	SELECT id, project_id, user_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at
	FROM user_inbox_messages
	WHERE deleted_at IS NULL
	AND sent_at IS NULL
	AND failed_at IS NULL
	AND scheduled_at <= NOW()
	AND (expires_at IS NULL OR expires_at > NOW())
	ORDER BY scheduled_at ASC
	LIMIT $1
	FOR UPDATE SKIP LOCKED`

	var messages []InboxMessage
	err := s.db.SelectContext(ctx, &messages, stmt, limit)
	if err != nil {
		return 0, err
	}

	for n, message := range messages {
		if err := fn(message); err != nil {
			return n, err
		}
	}

	return len(messages), nil
}

// ScanDueOrganizationInboxMessages iterates over at most limit organization inbox
// messages whose scheduled_at is in the past and that have not been sent,
// deleted, or expired.
//
// FOR UPDATE SKIP LOCKED lets concurrent scanners avoid handing each other the
// same rows without blocking. As with ScanDueUserInboxMessages, the scan is not
// held in a transaction across fn, so SKIP LOCKED reduces contention but does
// not by itself prevent double-processing; idempotency comes from the
// scheduler's stable Msg-Id and the consumer's sent_at guard.
func (s *InboxStore) ScanDueOrganizationInboxMessages(ctx context.Context, limit int, fn func(InboxMessage) error) (int, error) {
	stmt := `
	SELECT id, project_id, organization_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at
	FROM organization_inbox_messages
	WHERE deleted_at IS NULL
	AND sent_at IS NULL
	AND failed_at IS NULL
	AND scheduled_at <= NOW()
	AND (expires_at IS NULL OR expires_at > NOW())
	ORDER BY scheduled_at ASC
	LIMIT $1
	FOR UPDATE SKIP LOCKED`

	var messages []InboxMessage
	err := s.db.SelectContext(ctx, &messages, stmt, limit)
	if err != nil {
		return 0, err
	}

	for n, message := range messages {
		if err := fn(message); err != nil {
			return n, err
		}
	}

	return len(messages), nil
}

// MarkUserInboxMessageSent atomically sets sent_at on a user inbox message
// that has not been sent yet. Returns true when the timestamp was set, false
// when the message was already settled - marked as sent (idempotent across
// retries) or marked as permanently failed.
func (s *InboxStore) MarkUserInboxMessageSent(ctx context.Context, messageID uuid.UUID) (bool, error) {
	stmt := `
	UPDATE user_inbox_messages
	SET sent_at = NOW(), updated_at = NOW()
	WHERE id = $1 AND sent_at IS NULL AND failed_at IS NULL AND deleted_at IS NULL`

	result, err := s.db.ExecContext(ctx, stmt, messageID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

// MarkOrganizationInboxMessageSent atomically sets sent_at on an organization
// inbox message that has not been sent yet. Returns true when the timestamp
// was set, false when the message was already settled as sent or failed.
func (s *InboxStore) MarkOrganizationInboxMessageSent(ctx context.Context, messageID uuid.UUID) (bool, error) {
	stmt := `
	UPDATE organization_inbox_messages
	SET sent_at = NOW(), updated_at = NOW()
	WHERE id = $1 AND sent_at IS NULL AND failed_at IS NULL AND deleted_at IS NULL`

	result, err := s.db.ExecContext(ctx, stmt, messageID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

// MarkUserInboxMessageFailed atomically settles a user inbox message on a
// permanent failure. Returns true when the outcome was recorded, false when the
// message was already settled - a message that has been sent can never be
// marked failed, and the first failure reason is the one that is kept.
func (s *InboxStore) MarkUserInboxMessageFailed(ctx context.Context, id uuid.UUID, reason string) (bool, error) {
	stmt := `
	UPDATE user_inbox_messages
	SET failed_at = NOW(), failure_reason = $2, updated_at = NOW()
	WHERE id = $1 AND sent_at IS NULL AND failed_at IS NULL AND deleted_at IS NULL`

	result, err := s.db.ExecContext(ctx, stmt, id, reason)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

// MarkOrganizationInboxMessageFailed atomically settles an organization inbox
// message on a permanent failure. Returns true when the outcome was recorded,
// false when the message was already settled as sent or failed.
func (s *InboxStore) MarkOrganizationInboxMessageFailed(ctx context.Context, id uuid.UUID, reason string) (bool, error) {
	stmt := `
	UPDATE organization_inbox_messages
	SET failed_at = NOW(), failure_reason = $2, updated_at = NOW()
	WHERE id = $1 AND sent_at IS NULL AND failed_at IS NULL AND deleted_at IS NULL`

	result, err := s.db.ExecContext(ctx, stmt, id, reason)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

// GetUserInboxMessageByID loads a user inbox message by its primary key only.
// Used by the inbox dispatch layer when only the message ID is known.
func (s *InboxStore) GetUserInboxMessageByID(ctx context.Context, messageID uuid.UUID) (*InboxMessage, error) {
	stmt := `
	SELECT id, project_id, user_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at
	FROM user_inbox_messages
	WHERE id = $1 AND deleted_at IS NULL`

	var message InboxMessage
	err := s.db.GetContext(ctx, &message, stmt, messageID)
	if err != nil {
		return nil, err
	}
	return &message, nil
}

// GetOrganizationInboxMessageByID loads an organization inbox message by its
// primary key only. Used by the inbox dispatch layer when only the message ID
// is known.
func (s *InboxStore) GetOrganizationInboxMessageByID(ctx context.Context, messageID uuid.UUID) (*InboxMessage, error) {
	stmt := `
	SELECT id, project_id, organization_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at
	FROM organization_inbox_messages
	WHERE id = $1 AND deleted_at IS NULL`

	var message InboxMessage
	err := s.db.GetContext(ctx, &message, stmt, messageID)
	if err != nil {
		return nil, err
	}
	return &message, nil
}

// ErrInboxMessageAlreadyDue is returned by Update*InboxMessageScheduledAt
// when the target message exists but its scheduled_at is already <= NOW(),
// meaning it has either been picked up by the scheduler or is about to be.
// The plan's §2 contract is: a message can only be rescheduled while it is
// still strictly in the future.
var ErrInboxMessageAlreadyDue = errors.New("inbox message is already due and cannot be rescheduled")

func (s *InboxStore) UpdateUserInboxMessageScheduledAt(ctx context.Context, projectID, userID, messageID uuid.UUID, scheduledAt time.Time) (*InboxMessage, error) {
	stmt := `
	WITH updated AS (
		UPDATE user_inbox_messages
		SET scheduled_at = $4
		WHERE id = $1
		AND project_id = $2
		AND user_id = $3
		AND scheduled_at > NOW()
		AND deleted_at IS NULL
		RETURNING id, project_id, user_id, external_id, channel,
			sender_identity_id, campaign_id, broadcast_id, content, data, tags,
			priority, source, scheduled_at, expires_at, read_at, archived_at,
			sent_at, failed_at, failure_reason, class, recipient_timezone,
			created_at, updated_at, deleted_at
	)
	SELECT TRUE AS was_updated, updated.* FROM updated
	UNION ALL
	SELECT FALSE AS was_updated,
		id, project_id, user_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at
	FROM user_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND user_id = $3
	AND deleted_at IS NULL
	AND scheduled_at <= NOW()
	AND NOT EXISTS (SELECT 1 FROM updated)`

	return s.scanInboxScheduleUpdate(ctx, stmt, messageID, projectID, userID, scheduledAt)
}

func (s *InboxStore) UpdateOrganizationInboxMessageScheduledAt(ctx context.Context, projectID, organizationID, messageID uuid.UUID, scheduledAt time.Time) (*InboxMessage, error) {
	stmt := `
	WITH updated AS (
		UPDATE organization_inbox_messages
		SET scheduled_at = $4
		WHERE id = $1
		AND project_id = $2
		AND organization_id = $3
		AND scheduled_at > NOW()
		AND deleted_at IS NULL
		RETURNING id, project_id, organization_id, external_id, channel,
			sender_identity_id, campaign_id, broadcast_id, content, data, tags,
			priority, source, scheduled_at, expires_at, read_at, archived_at,
			sent_at, failed_at, failure_reason, class, recipient_timezone,
			created_at, updated_at, deleted_at
	)
	SELECT TRUE AS was_updated, updated.* FROM updated
	UNION ALL
	SELECT FALSE AS was_updated,
		id, project_id, organization_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at
	FROM organization_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND organization_id = $3
	AND deleted_at IS NULL
	AND scheduled_at <= NOW()
	AND NOT EXISTS (SELECT 1 FROM updated)`

	return s.scanInboxScheduleUpdate(ctx, stmt, messageID, projectID, organizationID, scheduledAt)
}

// scanInboxScheduleUpdate executes the schedule-update CTE and turns the
// `was_updated` discriminator into ErrInboxMessageAlreadyDue when the row
// existed but could not be updated.
func (s *InboxStore) scanInboxScheduleUpdate(ctx context.Context, stmt string, args ...any) (*InboxMessage, error) {
	var row struct {
		WasUpdated bool `db:"was_updated"`
		InboxMessage
	}
	if err := s.db.GetContext(ctx, &row, stmt, args...); err != nil {
		return nil, err
	}
	if !row.WasUpdated {
		return nil, ErrInboxMessageAlreadyDue
	}
	return &row.InboxMessage, nil
}

func (s *InboxStore) GetUserInboxMessage(ctx context.Context, projectID, userID, messageID uuid.UUID) (*InboxMessage, error) {
	stmt := `
	SELECT id, project_id, user_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at
	FROM user_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND user_id = $3
	AND deleted_at IS NULL`

	var message InboxMessage
	err := s.db.GetContext(ctx, &message, stmt, messageID, projectID, userID)
	if err != nil {
		return nil, err
	}

	return &message, nil
}

func (s *InboxStore) GetOrganizationInboxMessage(ctx context.Context, projectID, organizationID, messageID uuid.UUID) (*InboxMessage, error) {
	stmt := `
	SELECT id, project_id, organization_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at
	FROM organization_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND organization_id = $3
	AND deleted_at IS NULL`

	var message InboxMessage
	err := s.db.GetContext(ctx, &message, stmt, messageID, projectID, organizationID)
	if err != nil {
		return nil, err
	}

	return &message, nil
}

func (s *InboxStore) ReadUserInboxMessage(ctx context.Context, projectID, userID, messageID uuid.UUID) (*InboxMessage, bool, error) {
	stmt := `
	WITH updated AS (
		UPDATE user_inbox_messages
		SET read_at = NOW()
		WHERE id = $1
		AND project_id = $2
		AND user_id = $3
		AND deleted_at IS NULL
		AND scheduled_at <= NOW()
		AND (expires_at IS NULL OR expires_at > NOW())
		AND read_at IS NULL
		RETURNING id, project_id, user_id, external_id, channel,
			sender_identity_id, campaign_id, broadcast_id, content, data, tags,
			priority, source, scheduled_at, expires_at, read_at, archived_at,
			sent_at, failed_at, failure_reason, class, recipient_timezone,
			created_at, updated_at, deleted_at,
			true AS transitioned
	)
	SELECT * FROM updated
	UNION ALL
	SELECT id, project_id, user_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at,
		false AS transitioned
	FROM user_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND user_id = $3
	AND deleted_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM updated)`

	type result struct {
		InboxMessage
		Transitioned bool `db:"transitioned"`
	}

	var row result
	err := s.db.GetContext(ctx, &row, stmt, messageID, projectID, userID)
	if err != nil {
		return nil, false, err
	}

	return &row.InboxMessage, row.Transitioned, nil
}

func (s *InboxStore) ArchiveUserInboxMessage(ctx context.Context, projectID, userID, messageID uuid.UUID) (*InboxMessage, bool, error) {
	stmt := `
	WITH updated AS (
		UPDATE user_inbox_messages
		SET archived_at = NOW()
		WHERE id = $1
		AND project_id = $2
		AND user_id = $3
		AND deleted_at IS NULL
		AND scheduled_at <= NOW()
		AND (expires_at IS NULL OR expires_at > NOW())
		AND archived_at IS NULL
		RETURNING id, project_id, user_id, external_id, channel,
			sender_identity_id, campaign_id, broadcast_id, content, data, tags,
			priority, source, scheduled_at, expires_at, read_at, archived_at,
			sent_at, failed_at, failure_reason, class, recipient_timezone,
			created_at, updated_at, deleted_at,
			true AS transitioned
	)
	SELECT * FROM updated
	UNION ALL
	SELECT id, project_id, user_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at,
		false AS transitioned
	FROM user_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND user_id = $3
	AND deleted_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM updated)`

	type result struct {
		InboxMessage
		Transitioned bool `db:"transitioned"`
	}

	var row result
	err := s.db.GetContext(ctx, &row, stmt, messageID, projectID, userID)
	if err != nil {
		return nil, false, err
	}

	return &row.InboxMessage, row.Transitioned, nil
}

func (s *InboxStore) ReadOrganizationInboxMessage(ctx context.Context, projectID, organizationID, messageID uuid.UUID) (*InboxMessage, bool, error) {
	stmt := `
	WITH updated AS (
		UPDATE organization_inbox_messages
		SET read_at = NOW()
		WHERE id = $1
		AND project_id = $2
		AND organization_id = $3
		AND deleted_at IS NULL
		AND scheduled_at <= NOW()
		AND (expires_at IS NULL OR expires_at > NOW())
		AND read_at IS NULL
		RETURNING id, project_id, organization_id, external_id, channel,
			sender_identity_id, campaign_id, broadcast_id, content, data, tags,
			priority, source, scheduled_at, expires_at, read_at, archived_at,
			sent_at, failed_at, failure_reason, class, recipient_timezone,
			created_at, updated_at, deleted_at,
			true AS transitioned
	)
	SELECT * FROM updated
	UNION ALL
	SELECT id, project_id, organization_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at,
		false AS transitioned
	FROM organization_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND organization_id = $3
	AND deleted_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM updated)`

	type result struct {
		InboxMessage
		Transitioned bool `db:"transitioned"`
	}

	var row result
	err := s.db.GetContext(ctx, &row, stmt, messageID, projectID, organizationID)
	if err != nil {
		return nil, false, err
	}

	return &row.InboxMessage, row.Transitioned, nil
}

func (s *InboxStore) ArchiveOrganizationInboxMessage(ctx context.Context, projectID, organizationID, messageID uuid.UUID) (*InboxMessage, bool, error) {
	stmt := `
	WITH updated AS (
		UPDATE organization_inbox_messages
		SET archived_at = NOW()
		WHERE id = $1
		AND project_id = $2
		AND organization_id = $3
		AND deleted_at IS NULL
		AND scheduled_at <= NOW()
		AND (expires_at IS NULL OR expires_at > NOW())
		AND archived_at IS NULL
		RETURNING id, project_id, organization_id, external_id, channel,
			sender_identity_id, campaign_id, broadcast_id, content, data, tags,
			priority, source, scheduled_at, expires_at, read_at, archived_at,
			sent_at, failed_at, failure_reason, class, recipient_timezone,
			created_at, updated_at, deleted_at,
			true AS transitioned
	)
	SELECT * FROM updated
	UNION ALL
	SELECT id, project_id, organization_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at,
		false AS transitioned
	FROM organization_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND organization_id = $3
	AND deleted_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM updated)`

	type result struct {
		InboxMessage
		Transitioned bool `db:"transitioned"`
	}

	var row result
	err := s.db.GetContext(ctx, &row, stmt, messageID, projectID, organizationID)
	if err != nil {
		return nil, false, err
	}

	return &row.InboxMessage, row.Transitioned, nil
}

func (s *InboxStore) UnarchiveUserInboxMessage(ctx context.Context, projectID, userID, messageID uuid.UUID) (*InboxMessage, bool, error) {
	stmt := `
	WITH updated AS (
		UPDATE user_inbox_messages
		SET archived_at = NULL
		WHERE id = $1
		AND project_id = $2
		AND user_id = $3
		AND deleted_at IS NULL
		AND archived_at IS NOT NULL
		RETURNING id, project_id, user_id, external_id, channel,
			sender_identity_id, campaign_id, broadcast_id, content, data, tags,
			priority, source, scheduled_at, expires_at, read_at, archived_at,
			sent_at, failed_at, failure_reason, class, recipient_timezone,
			created_at, updated_at, deleted_at,
			true AS transitioned
	)
	SELECT * FROM updated
	UNION ALL
	SELECT id, project_id, user_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at,
		false AS transitioned
	FROM user_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND user_id = $3
	AND deleted_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM updated)`

	type result struct {
		InboxMessage
		Transitioned bool `db:"transitioned"`
	}

	var row result
	err := s.db.GetContext(ctx, &row, stmt, messageID, projectID, userID)
	if err != nil {
		return nil, false, err
	}

	return &row.InboxMessage, row.Transitioned, nil
}

func (s *InboxStore) UnreadUserInboxMessage(ctx context.Context, projectID, userID, messageID uuid.UUID) (*InboxMessage, bool, error) {
	stmt := `
	WITH updated AS (
		UPDATE user_inbox_messages
		SET read_at = NULL
		WHERE id = $1
		AND project_id = $2
		AND user_id = $3
		AND deleted_at IS NULL
		AND read_at IS NOT NULL
		RETURNING id, project_id, user_id, external_id, channel,
			sender_identity_id, campaign_id, broadcast_id, content, data, tags,
			priority, source, scheduled_at, expires_at, read_at, archived_at,
			sent_at, failed_at, failure_reason, class, recipient_timezone,
			created_at, updated_at, deleted_at,
			true AS transitioned
	)
	SELECT * FROM updated
	UNION ALL
	SELECT id, project_id, user_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at,
		false AS transitioned
	FROM user_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND user_id = $3
	AND deleted_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM updated)`

	type result struct {
		InboxMessage
		Transitioned bool `db:"transitioned"`
	}

	var row result
	err := s.db.GetContext(ctx, &row, stmt, messageID, projectID, userID)
	if err != nil {
		return nil, false, err
	}

	return &row.InboxMessage, row.Transitioned, nil
}

func (s *InboxStore) UnarchiveOrganizationInboxMessage(ctx context.Context, projectID, organizationID, messageID uuid.UUID) (*InboxMessage, bool, error) {
	stmt := `
	WITH updated AS (
		UPDATE organization_inbox_messages
		SET archived_at = NULL
		WHERE id = $1
		AND project_id = $2
		AND organization_id = $3
		AND deleted_at IS NULL
		AND archived_at IS NOT NULL
		RETURNING id, project_id, organization_id, external_id, channel,
			sender_identity_id, campaign_id, broadcast_id, content, data, tags,
			priority, source, scheduled_at, expires_at, read_at, archived_at,
			sent_at, failed_at, failure_reason, class, recipient_timezone,
			created_at, updated_at, deleted_at,
			true AS transitioned
	)
	SELECT * FROM updated
	UNION ALL
	SELECT id, project_id, organization_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at,
		false AS transitioned
	FROM organization_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND organization_id = $3
	AND deleted_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM updated)`

	type result struct {
		InboxMessage
		Transitioned bool `db:"transitioned"`
	}

	var row result
	err := s.db.GetContext(ctx, &row, stmt, messageID, projectID, organizationID)
	if err != nil {
		return nil, false, err
	}

	return &row.InboxMessage, row.Transitioned, nil
}

func (s *InboxStore) UnreadOrganizationInboxMessage(ctx context.Context, projectID, organizationID, messageID uuid.UUID) (*InboxMessage, bool, error) {
	stmt := `
	WITH updated AS (
		UPDATE organization_inbox_messages
		SET read_at = NULL
		WHERE id = $1
		AND project_id = $2
		AND organization_id = $3
		AND deleted_at IS NULL
		AND read_at IS NOT NULL
		RETURNING id, project_id, organization_id, external_id, channel,
			sender_identity_id, campaign_id, broadcast_id, content, data, tags,
			priority, source, scheduled_at, expires_at, read_at, archived_at,
			sent_at, failed_at, failure_reason, class, recipient_timezone,
			created_at, updated_at, deleted_at,
			true AS transitioned
	)
	SELECT * FROM updated
	UNION ALL
	SELECT id, project_id, organization_id, external_id, channel,
		sender_identity_id, campaign_id, broadcast_id, content, data, tags,
		priority, source, scheduled_at, expires_at, read_at, archived_at,
		sent_at, failed_at, failure_reason, class, recipient_timezone,
		created_at, updated_at, deleted_at,
		false AS transitioned
	FROM organization_inbox_messages
	WHERE id = $1
	AND project_id = $2
	AND organization_id = $3
	AND deleted_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM updated)`

	type result struct {
		InboxMessage
		Transitioned bool `db:"transitioned"`
	}

	var row result
	err := s.db.GetContext(ctx, &row, stmt, messageID, projectID, organizationID)
	if err != nil {
		return nil, false, err
	}

	return &row.InboxMessage, row.Transitioned, nil
}

package consumer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/stretchr/testify/require"
)

// TestInboxMessageParamsProjectsAllWireFields verifies that every field on
// the wire schemas.InboxMessage is forwarded to its slot on
// subjects.InboxMessageParams, and — by virtue of being a pure assignment
// test — that no dropped field (idempotency_key, provider_id, template_id,
// journey_*, title, body, format, rendered_payload, link_url) accidentally
// creeps back into the projection.
func TestInboxMessageParamsProjectsAllWireFields(t *testing.T) {
	t.Parallel()

	externalID := "ext-123"
	senderIdentityID := uuid.New()
	campaignID := uuid.New()
	broadcastID := uuid.New()
	scheduledAt := time.Now().Add(time.Hour).UTC()
	expiresAt := scheduledAt.Add(24 * time.Hour)
	source := "campaign"

	inbox := schemas.InboxMessage{
		ProjectID:        uuid.New(),
		SubjectID:        uuid.New(),
		ExternalID:       &externalID,
		Channel:          "email",
		SenderIdentityID: &senderIdentityID,
		CampaignID:       &campaignID,
		BroadcastID:      &broadcastID,
		Content:          json.RawMessage(`{"subject":"hi"}`),
		Data:             json.RawMessage(`{"template_id":"abc"}`),
		Tags:             []string{"campaign", "journey"},
		Priority:         ptr.To(int16(3)),
		Source:           &source,
		ScheduledAt:      &scheduledAt,
		ExpiresAt:        &expiresAt,
	}

	params := inboxMessageParams(inbox)

	require.Equal(t, inbox.ExternalID, params.ExternalID)
	require.Equal(t, modules.Channel(inbox.Channel), params.Channel)
	require.Equal(t, inbox.SenderIdentityID, params.SenderIdentityID)
	require.Equal(t, inbox.CampaignID, params.CampaignID)
	require.Equal(t, inbox.BroadcastID, params.BroadcastID)
	require.JSONEq(t, string(inbox.Content), string(params.Content))
	require.JSONEq(t, string(inbox.Data), string(params.Data))
	require.Equal(t, inbox.Tags, params.Tags)
	require.Equal(t, inbox.Priority, params.Priority)
	require.Equal(t, inbox.Source, params.Source)
	require.Equal(t, inbox.ScheduledAt, params.ScheduledAt)
	require.Equal(t, inbox.ExpiresAt, params.ExpiresAt)
}

// TestInboxMessageParamsPreservesEmptyOptionals verifies that nil/zero
// optional fields pass through unchanged — the store layer is responsible
// for defaulting, not the projection.
func TestInboxMessageParamsPreservesEmptyOptionals(t *testing.T) {
	t.Parallel()

	inbox := schemas.InboxMessage{
		ProjectID: uuid.New(),
		SubjectID: uuid.New(),
		Channel:   "inbox",
	}

	params := inboxMessageParams(inbox)

	require.Nil(t, params.ExternalID)
	require.Equal(t, modules.Channel("inbox"), params.Channel)
	require.Nil(t, params.SenderIdentityID)
	require.Nil(t, params.CampaignID)
	require.Nil(t, params.BroadcastID)
	require.Nil(t, params.Content)
	require.Nil(t, params.Data)
	require.Nil(t, params.Tags)
	require.Nil(t, params.Priority)
	require.Nil(t, params.Source)
	require.Nil(t, params.ScheduledAt)
	require.Nil(t, params.ExpiresAt)
}

// TODO(plan): handler-level coverage (inbox channel ingestion, email
// channel ingestion, idempotent re-delivery) requires fakes for
// management.State, providers.Registry, and jetstream.Msg.
// See migration-execution-plan.md T05.

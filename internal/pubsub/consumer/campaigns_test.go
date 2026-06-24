package consumer

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules"
	providers "github.com/lunogram/platform/pkg/modules/providers"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestBuildRenderDataIncludesUnsubscribeURLWhenSubscriptionLinked(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	campaignID := uuid.New()
	subscriptionID := uuid.New()
	userID := uuid.New()

	campaign := &management.Campaign{
		ID:             campaignID,
		ProjectID:      projectID,
		Transactional:  false,
		SubscriptionID: &subscriptionID,
	}
	user := &subjects.User{ID: userID}

	data := buildRenderData("https://example.com/", user, campaign, nil)

	rawUnsubscribeURL, ok := data["unsubscribe_url"]
	require.True(t, ok)

	unsubscribeURL, ok := rawUnsubscribeURL.(string)
	require.True(t, ok)

	parsedURL, err := url.Parse(unsubscribeURL)
	require.NoError(t, err)
	require.Equal(t, "/unsubscribe/email", parsedURL.Path)

	link := parsedURL.Query().Get("link")
	require.NotEmpty(t, link)

	parsedLink, err := url.Parse(link)
	require.NoError(t, err)
	require.Equal(t, userID.String(), parsedLink.Query().Get("u"))
	require.Equal(t, campaignID.String(), parsedLink.Query().Get("c"))
}

func TestBuildRenderDataOmitsUnsubscribeURLWhenNotEligible(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	userID := uuid.New()
	user := &subjects.User{ID: userID}

	tests := map[string]*management.Campaign{
		"transactional campaign": {
			ID:             uuid.New(),
			ProjectID:      projectID,
			Transactional:  true,
			SubscriptionID: func() *uuid.UUID { id := uuid.New(); return &id }(),
		},
		"no linked subscription": {
			ID:             uuid.New(),
			ProjectID:      projectID,
			Transactional:  false,
			SubscriptionID: nil,
		},
	}

	for name, campaign := range tests {
		campaign := campaign
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data := buildRenderData("https://example.com", user, campaign, nil)
			_, ok := data["unsubscribe_url"]
			require.False(t, ok)
		})
	}
}

func TestInboxSummaryFromPayload(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		channel providers.Channel
		payload json.RawMessage
		title   string
		body    *string
	}{
		"email text body": {
			channel: providers.ChannelEmail,
			payload: json.RawMessage(`{"subject":"Hello","text":"Plain","html":"<p>HTML</p>"}`),
			title:   "Hello",
			body:    stringPtr("Plain"),
		},
		"email html fallback": {
			channel: providers.ChannelEmail,
			payload: json.RawMessage(`{"subject":"Hello","html":"<p>HTML</p>"}`),
			title:   "Hello",
			body:    stringPtr("<p>HTML</p>"),
		},
		"sms": {
			channel: providers.ChannelSMS,
			payload: json.RawMessage(`{"body":"Hi"}`),
			title:   "Hi",
			body:    stringPtr("Hi"),
		},
		"push": {
			channel: providers.ChannelPush,
			payload: json.RawMessage(`{"title":"Ping","body":"Open app"}`),
			title:   "Ping",
			body:    stringPtr("Open app"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			summary, err := inboxSummaryFromPayload(tc.channel, tc.payload)
			require.NoError(t, err)
			require.Equal(t, tc.title, summary.Title)
			require.Equal(t, tc.body, summary.Body)
		})
	}

	_, err := inboxSummaryFromPayload(providers.ChannelEmail, json.RawMessage(`{`))
	require.Error(t, err)
}

func TestCreateCampaignInboxMessagePersistsOriginAndPayload(t *testing.T) {
	t.Parallel()

	db, sqlxDB := newSubjectsTestStoreWithDB(t)
	ctx := context.Background()
	projectID := uuid.New()
	userID, err := db.UpsertUser(ctx, projectID, subjects.UpsertUserParams{
		Identifiers: []subjects.ExternalIDParam{{Source: "default", ExternalID: "campaign-user"}},
	})
	require.NoError(t, err)

	senderIdentityID := uuid.New()
	templateID := uuid.New()
	campaignID := uuid.New()
	broadcastID := uuid.New()
	journeyID := uuid.New()
	journeyEntryID := uuid.New()
	journeyStepID := "send-message"
	payload := json.RawMessage(`{"subject":"Rendered","text":"Body"}`)
	rendered := renderedCampaignInboxMessage{
		Channel:          providers.ChannelEmail,
		SenderIdentityID: &senderIdentityID,
		TemplateID:       templateID,
		RenderedPayload:  payload,
	}
	event := schemas.SendCampaign{
		ProjectID:   projectID,
		UserID:      userID,
		CampaignID:  campaignID,
		BroadcastID: &broadcastID,
		Data: &schemas.SendCampaignData{
			JourneyID:      &journeyID,
			JourneyEntryID: &journeyEntryID,
			JourneyStepID:  &journeyStepID,
		},
	}

	message, err := createCampaignInboxMessageAndPublish(ctx, sqlxDB, pubsub.NewNoopPublisher(), zaptest.NewLogger(t), event, rendered)
	require.NoError(t, err)
	require.Equal(t, schemas.InboxSourceJourney, *message.Source)
	require.Equal(t, []string{"journey", "campaign"}, []string(message.Tags))
	require.Equal(t, &senderIdentityID, message.SenderIdentityID)
	require.Equal(t, &campaignID, message.CampaignID)
	require.Equal(t, &broadcastID, message.BroadcastID)
	require.Equal(t, modules.Channel(providers.ChannelEmail), message.Channel)
	require.NotNil(t, message.ExternalID)
	require.Contains(t, *message.ExternalID, journeyEntryID.String())
	require.Contains(t, *message.ExternalID, senderIdentityID.String())
	require.JSONEq(t, string(payload), string(message.Content))

	var provenance map[string]any
	require.NoError(t, json.Unmarshal(message.Data, &provenance))
	require.Equal(t, templateID.String(), provenance["template_id"])
	require.Equal(t, campaignID.String(), provenance["campaign_id"])
	require.Equal(t, broadcastID.String(), provenance["broadcast_id"])
	require.Equal(t, journeyID.String(), provenance["journey_id"])
	require.Equal(t, journeyEntryID.String(), provenance["journey_entry_id"])
	require.Equal(t, journeyStepID, provenance["journey_step_id"])

	retried, err := createCampaignInboxMessageAndPublish(ctx, sqlxDB, pubsub.NewNoopPublisher(), zaptest.NewLogger(t), event, rendered)
	require.NoError(t, err)
	require.Equal(t, message.ID, retried.ID)

	changed := rendered
	changed.RenderedPayload = json.RawMessage(`{"subject":"Changed","text":"Body"}`)
	retried, err = createCampaignInboxMessageAndPublish(ctx, sqlxDB, pubsub.NewNoopPublisher(), zaptest.NewLogger(t), event, changed)
	require.NoError(t, err)
	require.Equal(t, message.ID, retried.ID)
}

func stringPtr(value string) *string {
	return &value
}

func newSubjectsTestStoreWithDB(t *testing.T) (*subjects.State, *sqlx.DB) {
	t.Helper()

	uri := container.RunPostgreSQL(t)
	usersURI := container.CreateSchema(t, uri, "users")
	require.NoError(t, subjects.Migrate(usersURI))

	ctx := graceful.NewContext(t.Context())
	logger := zaptest.NewLogger(t)
	usersDB, err := store.Connect(ctx, logger, usersURI)
	require.NoError(t, err)

	return subjects.NewState(usersDB, logger), usersDB
}

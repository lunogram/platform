package schemas

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestInboxOriginPrecedence pins the source/external_id precedence
// (journey w/ entry+step → broadcast → campaign) called out in
// migration-execution-plan.md T05 as a bug fix. A unit test guards against
// regression of the precedence ordering since the production effect — wrong
// source label on the inbox row, wrong dedupe key — is silent.
func TestInboxOriginPrecedence(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	userID := uuid.New()
	campaignID := uuid.New()
	broadcastID := uuid.New()
	journeyID := uuid.New()
	journeyEntryID := uuid.New()
	stepID := "step-1"
	providerPart := "multi"

	base := SendCampaign{
		ProjectID:  projectID,
		UserID:     userID,
		CampaignID: campaignID,
	}

	t.Run("journey with entry and step wins over broadcast and campaign", func(t *testing.T) {
		t.Parallel()

		event := base
		event.BroadcastID = &broadcastID
		event.Data = &SendCampaignData{
			JourneyID:      &journeyID,
			JourneyEntryID: &journeyEntryID,
			JourneyStepID:  &stepID,
		}

		source, externalID := event.InboxOrigin(providerPart)
		require.Equal(t, InboxSourceJourney, source)
		require.True(t, strings.HasPrefix(externalID, "journey:"), "expected journey-prefixed key, got %q", externalID)
		require.Contains(t, externalID, journeyEntryID.String())
		require.Contains(t, externalID, stepID)
		require.Contains(t, externalID, userID.String())
		require.Contains(t, externalID, campaignID.String())
		require.Contains(t, externalID, providerPart)
	})

	t.Run("journey without complete entry+step falls through to broadcast", func(t *testing.T) {
		t.Parallel()

		event := base
		event.BroadcastID = &broadcastID
		event.Data = &SendCampaignData{
			JourneyID: &journeyID,
			// JourneyEntryID and JourneyStepID intentionally nil — partial
			// journey context must not claim the journey precedence slot.
		}

		source, externalID := event.InboxOrigin(providerPart)
		require.Equal(t, InboxSourceBroadcast, source)
		require.True(t, strings.HasPrefix(externalID, "broadcast:"), "expected broadcast-prefixed key, got %q", externalID)
		require.Contains(t, externalID, broadcastID.String())
	})

	t.Run("broadcast alone uses broadcast source", func(t *testing.T) {
		t.Parallel()

		event := base
		event.BroadcastID = &broadcastID

		source, externalID := event.InboxOrigin(providerPart)
		require.Equal(t, InboxSourceBroadcast, source)
		require.True(t, strings.HasPrefix(externalID, "broadcast:"), "got %q", externalID)
		require.Contains(t, externalID, broadcastID.String())
		require.Contains(t, externalID, userID.String())
		require.Contains(t, externalID, providerPart)
	})

	t.Run("campaign-only falls back to campaign source", func(t *testing.T) {
		t.Parallel()

		source, externalID := base.InboxOrigin(providerPart)
		require.Equal(t, InboxSourceCampaign, source)
		require.True(t, strings.HasPrefix(externalID, "campaign:"), "got %q", externalID)
		require.Contains(t, externalID, campaignID.String())
		require.Contains(t, externalID, userID.String())
		require.Contains(t, externalID, providerPart)
	})
}

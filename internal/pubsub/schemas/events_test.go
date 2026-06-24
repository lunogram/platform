package schemas

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJourneyStepEntered(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	journeyID := uuid.New()
	entryID := uuid.New()
	userID := uuid.New()
	versionID := uuid.New()
	name := "Welcome email"

	event := JourneyStepEntered(projectID, journeyID, entryID, userID, &versionID, "step-1", "campaign", &name)

	assert.Equal(t, EventJourneyStepEntered, event.Name)
	assert.Equal(t, projectID, event.ProjectID)
	assert.Equal(t, userID, event.UserID)

	// Journey and step provenance lives under Data so gate rules can filter on it.
	assert.Equal(t, journeyID, event.Data["journey_id"])
	assert.Equal(t, entryID, event.Data["journey_entry_id"])
	assert.Equal(t, versionID, event.Data["version_id"])
	assert.Equal(t, "step-1", event.Data["step_id"])
	assert.Equal(t, "campaign", event.Data["step_type"])
	assert.Equal(t, name, event.Data["step_name"])
}

func TestJourneyStepEnteredOmitsOptionalFields(t *testing.T) {
	t.Parallel()

	event := JourneyStepEntered(uuid.New(), uuid.New(), uuid.New(), uuid.New(), nil, "step-1", "delay", nil)

	_, hasVersion := event.Data["version_id"]
	assert.False(t, hasVersion, "version_id should be omitted when nil")

	_, hasName := event.Data["step_name"]
	assert.False(t, hasName, "step_name should be omitted when nil")
}

// TestJourneyStepEnteredMsgID verifies the dedup key is stable for a given step
// entry (so redeliveries collapse) yet distinct across entries and step nodes
// (so genuine re-entries on a loop are still counted).
func TestJourneyStepEnteredMsgID(t *testing.T) {
	t.Parallel()

	entryID := uuid.New()

	// Same entry + step + source sequence -> identical (redelivery dedup).
	require.Equal(t,
		JourneyStepEnteredMsgID(entryID, "step-1", 42),
		JourneyStepEnteredMsgID(entryID, "step-1", 42),
	)

	// A distinct inbound message (new source sequence) is a distinct entry.
	require.NotEqual(t,
		JourneyStepEnteredMsgID(entryID, "step-1", 42),
		JourneyStepEnteredMsgID(entryID, "step-1", 43),
	)

	// Different step node within the same entry is distinct.
	require.NotEqual(t,
		JourneyStepEnteredMsgID(entryID, "step-1", 42),
		JourneyStepEnteredMsgID(entryID, "step-2", 42),
	)
}

package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// seedListTriggerJourney creates a published journey whose entrance is a list
// trigger configured to enter the user when they join listID and (when
// exitOnLeave is set) exit them when they leave it. It returns the created
// journey ID and the enter/exit event IDs registered as its dependencies.
func seedListTriggerJourney(t *testing.T, ctx context.Context, jrny *journey.State, usrs *subjects.State, projectID, listID uuid.UUID, exitOnLeave bool) (journeyID uuid.UUID) {
	t.Helper()

	journeyID, err := jrny.CreateJourney(ctx, journey.Journey{
		ProjectID: projectID,
		Name:      "List Trigger Journey",
	})
	require.NoError(t, err)

	versionID, err := jrny.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	entranceData, err := json.Marshal(oapi.EntranceStepData{
		Trigger: oapi.TriggerList,
		List: &oapi.ListTrigger{
			ID:          listID,
			Direction:   oapi.ListJoins,
			ExitOnLeave: exitOnLeave,
		},
	})
	require.NoError(t, err)

	stepMap := oapi.JourneyStepMap{
		"entrance-1": {
			Type: "entrance",
			Data: entranceData,
			X:    0,
			Y:    0,
			Children: []oapi.JourneyStepChild{
				{ExternalId: "delay-1"},
			},
		},
		"delay-1": {
			Type: "delay",
			X:    100,
			Y:    100,
		},
	}

	_, err = jrny.SetJourneySteps(ctx, versionID, stepMap)
	require.NoError(t, err)

	// Register the same enter/exit dependencies the publish handler would derive
	// for a list-join entrance configured to exit on leave.
	addedID, err := usrs.UpsertEvent(ctx, projectID, schemas.EventListUserAdded, subjects.SubjectTypeUser)
	require.NoError(t, err)
	removedID, err := usrs.UpsertEvent(ctx, projectID, schemas.EventListUserRemoved, subjects.SubjectTypeUser)
	require.NoError(t, err)

	deps := []journey.StepEventDependency{{EventID: addedID, Kind: journey.StepEventKindEnter}}
	if exitOnLeave {
		deps = append(deps, journey.StepEventDependency{EventID: removedID, Kind: journey.StepEventKindExit})
	}
	err = jrny.SetJourneyStepEventDependencies(ctx, versionID, "entrance-1", deps)
	require.NoError(t, err)

	err = jrny.PublishVersion(ctx, journeyID, versionID)
	require.NoError(t, err)

	return journeyID
}

// drainEntrances fetches up to a few journey-entrance messages with a short
// wait and returns them. An empty slice means nothing was published.
func drainEntrances(t *testing.T, ctx context.Context, jet jetstream.JetStream, ns Namespace) []schemas.JourneyEntrance {
	t.Helper()

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamJourneys), ns.Consumer(ConsumerJourneysEntrance))
	require.NoError(t, err)

	var entrances []schemas.JourneyEntrance
	for {
		msgs, err := consumer.Fetch(10, jetstream.FetchMaxWait(2*time.Second))
		require.NoError(t, err)

		var got int
		for msg := range msgs.Messages() {
			got++
			var entrance schemas.JourneyEntrance
			require.NoError(t, json.Unmarshal(msg.Data(), &entrance))
			entrances = append(entrances, entrance)
			require.NoError(t, msg.Ack())
		}
		require.NoError(t, msgs.Error())

		if got == 0 {
			break
		}
	}

	return entrances
}

// TestPublishUserEventJourneyDependenciesListIDFilter is the headline safety
// check: a list membership event must only enrol journeys configured for that
// exact list. A list.user.added for list A must NOT enrol a journey wired to
// list B.
func TestPublishUserEventJourneyDependenciesListIDFilter(t *testing.T) {
	t.Parallel()

	_, usrsDB, jrnyDB, projectID, jet := setupUserEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	require.NoError(t, Bootstrap(ctx, logger, jet, ns))

	usrs := subjects.NewState(usrsDB, zap.NewNop())
	jrny := journey.NewState(jrnyDB)
	pub := pubsub.NewPublisher(jet, string(ns))

	listA := uuid.New()
	listB := uuid.New()
	userID := uuid.New()

	// One journey is wired to list B only.
	journeyB := seedListTriggerJourney(t, ctx, jrny, usrs, projectID, listB, false)

	addedID, err := usrs.UpsertEvent(ctx, projectID, schemas.EventListUserAdded, subjects.SubjectTypeUser)
	require.NoError(t, err)

	t.Run("cross-list event does not enrol", func(t *testing.T) {
		event := schemas.UserEvent{
			ID:        addedID,
			Name:      schemas.EventListUserAdded,
			ProjectID: projectID,
			UserID:    userID,
			Data:      map[string]any{"list_id": listA.String()},
		}

		err := PublishUserEventJourneyDependencies(ctx, logger, jrny, pub, event)()
		require.NoError(t, err)

		entrances := drainEntrances(t, ctx, jet, ns)
		assert.Empty(t, entrances, "a list.user.added for list A must not enrol a journey configured for list B")
	})

	t.Run("matching-list event enrols", func(t *testing.T) {
		event := schemas.UserEvent{
			ID:        addedID,
			Name:      schemas.EventListUserAdded,
			ProjectID: projectID,
			UserID:    userID,
			Data:      map[string]any{"list_id": listB.String()},
		}

		err := PublishUserEventJourneyDependencies(ctx, logger, jrny, pub, event)()
		require.NoError(t, err)

		entrances := drainEntrances(t, ctx, jet, ns)
		require.Len(t, entrances, 1, "the journey configured for list B should be enrolled")
		assert.Equal(t, journeyB, entrances[0].JourneyID)
		assert.Equal(t, userID, entrances[0].UserID)
		assert.Equal(t, "entrance-1", entrances[0].ExternalStepID)
	})

	t.Run("missing list_id does not enrol", func(t *testing.T) {
		event := schemas.UserEvent{
			ID:        addedID,
			Name:      schemas.EventListUserAdded,
			ProjectID: projectID,
			UserID:    userID,
			Data:      map[string]any{},
		}

		err := PublishUserEventJourneyDependencies(ctx, logger, jrny, pub, event)()
		require.NoError(t, err)

		entrances := drainEntrances(t, ctx, jet, ns)
		assert.Empty(t, entrances, "an event without a list_id must not enrol a list-trigger journey")
	})
}

// TestCompleteUserEventJourneyExits covers the runtime exit path: a
// list.user.removed event completes the user's active runs for a journey
// configured to exit on leaving that list, and a mismatched list_id is skipped.
func TestCompleteUserEventJourneyExits(t *testing.T) {
	t.Parallel()

	_, usrsDB, jrnyDB, projectID, jet := setupUserEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	require.NoError(t, Bootstrap(ctx, logger, jet, ns))

	usrs := subjects.NewState(usrsDB, zap.NewNop())
	jrny := journey.NewState(jrnyDB)

	listID := uuid.New()
	otherListID := uuid.New()
	userID := uuid.New()

	journeyID := seedListTriggerJourney(t, ctx, jrny, usrs, projectID, listID, true)

	// Put the user into an active run started from the list-join entrance.
	entryID := uuid.New()
	_, err := jrny.CreateUserJourneyState(ctx, journey.JourneyUserState{
		JourneyID:      journeyID,
		JourneyEntryID: entryID,
		UserID:         userID,
		ExternalStepID: "entrance-1",
	})
	require.NoError(t, err)

	resumeAt := time.Now().Add(time.Hour)
	_, err = jrny.CreateUserJourneyState(ctx, journey.JourneyUserState{
		JourneyID:      journeyID,
		JourneyEntryID: entryID,
		UserID:         userID,
		ExternalStepID: "delay-1",
		ResumeAt:       &resumeAt,
	})
	require.NoError(t, err)

	removedID, err := usrs.UpsertEvent(ctx, projectID, schemas.EventListUserRemoved, subjects.SubjectTypeUser)
	require.NoError(t, err)

	t.Run("mismatched list_id is skipped", func(t *testing.T) {
		event := schemas.UserEvent{
			ID:        removedID,
			Name:      schemas.EventListUserRemoved,
			ProjectID: projectID,
			UserID:    userID,
			Data:      map[string]any{"list_id": otherListID.String()},
		}

		err := CompleteUserEventJourneyExits(ctx, logger, jrny, event)()
		require.NoError(t, err)

		state, err := jrny.GetUserJourneyState(ctx, entryID, "entrance-1")
		require.NoError(t, err)
		assert.Nil(t, state.CompletedAt, "leaving a different list must not complete the run")
	})

	t.Run("matching list leave completes the run", func(t *testing.T) {
		event := schemas.UserEvent{
			ID:        removedID,
			Name:      schemas.EventListUserRemoved,
			ProjectID: projectID,
			UserID:    userID,
			Data:      map[string]any{"list_id": listID.String()},
		}

		err := CompleteUserEventJourneyExits(ctx, logger, jrny, event)()
		require.NoError(t, err)

		entrance, err := jrny.GetUserJourneyState(ctx, entryID, "entrance-1")
		require.NoError(t, err)
		require.NotNil(t, entrance.CompletedAt, "leaving the configured list completes the entrance state")

		delay, err := jrny.GetUserJourneyState(ctx, entryID, "delay-1")
		require.NoError(t, err)
		require.NotNil(t, delay.CompletedAt, "the in-flight step in the same run is completed too")
		assert.Nil(t, delay.ResumeAt)
	})
}

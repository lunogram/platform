package journey

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJourneysStoreCreateJourney(t *testing.T) {
	t.Parallel()

	store, _ := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()

	t.Run("creates journey with initial draft version", func(t *testing.T) {
		journeyID, err := store.CreateJourney(ctx, Journey{
			ProjectID:   projectID,
			Name:        "Onboarding Journey",
			Description: ptr.To("Welcome new users"),
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, journeyID)

		journey, err := store.GetJourney(ctx, projectID, journeyID)
		require.NoError(t, err)
		assert.Equal(t, "Onboarding Journey", journey.Name)
		assert.Equal(t, "Welcome new users", *journey.Description)
		assert.Nil(t, journey.VersionID)
	})
}

func TestJourneysStoreVersionWorkflow(t *testing.T) {
	t.Parallel()

	store, db := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()

	journeyID, err := store.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	t.Run("creates initial draft version", func(t *testing.T) {
		versionID, err := store.CreateJourneyVersion(ctx, journeyID, "draft")
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, versionID)

		version, err := store.GetJourneyVersion(ctx, versionID)
		require.NoError(t, err)
		assert.Equal(t, journeyID, version.JourneyID)
		assert.Equal(t, 1, version.VersionNumber)
		assert.Equal(t, "draft", version.Status)
		assert.Nil(t, version.PublishedAt)
	})

	t.Run("publishes draft version", func(t *testing.T) {
		draftVersion, err := store.GetLatestDraftVersion(ctx, journeyID)
		require.NoError(t, err)

		err = store.PublishVersion(ctx, journeyID, draftVersion.ID)
		require.NoError(t, err)

		version, err := store.GetJourneyVersion(ctx, draftVersion.ID)
		require.NoError(t, err)
		assert.Equal(t, "published", version.Status)
		assert.NotNil(t, version.PublishedAt)

		journey, err := store.GetJourney(ctx, projectID, journeyID)
		require.NoError(t, err)
		assert.Equal(t, draftVersion.ID, *journey.VersionID)

		currentVersion, err := store.GetCurrentVersion(ctx, journeyID)
		require.NoError(t, err)
		assert.Equal(t, draftVersion.ID, currentVersion.ID)
	})

	t.Run("creates new draft version after publish", func(t *testing.T) {
		versionID, err := store.CreateJourneyVersion(ctx, journeyID, "draft")
		require.NoError(t, err)

		version, err := store.GetJourneyVersion(ctx, versionID)
		require.NoError(t, err)
		assert.Equal(t, 2, version.VersionNumber)
		assert.Equal(t, "draft", version.Status)
	})

	t.Run("archives old version when publishing new one", func(t *testing.T) {
		draftVersion, err := store.GetLatestDraftVersion(ctx, journeyID)
		require.NoError(t, err)

		err = store.PublishVersion(ctx, journeyID, draftVersion.ID)
		require.NoError(t, err)

		query := `SELECT status FROM journey_versions WHERE journey_id = $1 AND version_number = 1`
		var status string
		err = db.GetContext(ctx, &status, query, journeyID)
		require.NoError(t, err)
		assert.Equal(t, "archived", status)
	})
}

func TestJourneysStoreSetJourneySteps(t *testing.T) {
	t.Parallel()

	store, _ := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()

	journeyID, err := store.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	versionID, err := store.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	t.Run("creates new steps and children", func(t *testing.T) {
		stepMap := oapi.JourneyStepMap{
			"step-1": {
				Type: "entrance",
				Name: ptr.To("Welcome"),
				X:    100,
				Y:    200,
				Children: []oapi.JourneyStepChild{
					{ExternalId: "step-2"},
				},
			},
			"step-2": {
				Type: "exit",
				Name: ptr.To("Goodbye"),
				X:    300,
				Y:    400,
			},
		}

		stepIDs, err := store.SetJourneySteps(ctx, versionID, stepMap)
		require.NoError(t, err)
		assert.Len(t, stepIDs, 2)

		steps, err := store.GetJourneyVersionSteps(ctx, versionID)
		require.NoError(t, err)
		assert.Len(t, steps, 2)

		children, err := store.GetJourneyVersionStepsChildren(ctx, versionID)
		require.NoError(t, err)
		assert.Len(t, children, 1)
		assert.Equal(t, "step-1", children[0].ParentExternalID)
		assert.Equal(t, "step-2", children[0].ChildExternalID)
	})

	t.Run("updates existing steps", func(t *testing.T) {
		stepMap := oapi.JourneyStepMap{
			"step-1": {
				Type: "entrance",
				Name: ptr.To("Updated Welcome"),
				X:    150,
				Y:    250,
			},
			"step-2": {
				Type: "exit",
				Name: ptr.To("Updated Goodbye"),
				X:    350,
				Y:    450,
			},
		}

		_, err := store.SetJourneySteps(ctx, versionID, stepMap)
		require.NoError(t, err)

		steps, err := store.GetJourneyVersionSteps(ctx, versionID)
		require.NoError(t, err)
		assert.Len(t, steps, 2)

		for _, step := range steps {
			switch step.ExternalID {
			case "step-1":
				assert.Equal(t, "Updated Welcome", *step.Name)
				assert.Equal(t, 150.0, step.X)
				assert.Equal(t, 250.0, step.Y)
			case "step-2":
				assert.Equal(t, "Updated Goodbye", *step.Name)
				assert.Equal(t, 350.0, step.X)
				assert.Equal(t, 450.0, step.Y)
			}
		}
	})

	t.Run("removes orphaned steps", func(t *testing.T) {
		stepMap := oapi.JourneyStepMap{
			"step-1": {
				Type: "entrance",
				Name: ptr.To("Welcome"),
				X:    100,
				Y:    200,
			},
		}

		_, err := store.SetJourneySteps(ctx, versionID, stepMap)
		require.NoError(t, err)

		steps, err := store.GetJourneyVersionSteps(ctx, versionID)
		require.NoError(t, err)
		assert.Len(t, steps, 1)
		assert.Equal(t, "step-1", steps[0].ExternalID)
	})
}

func TestJourneysStoreDuplicateJourney(t *testing.T) {
	t.Parallel()

	store, _ := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()

	journeyID, err := store.CreateJourney(ctx, Journey{
		ProjectID:   projectID,
		Name:        "Original Journey",
		Description: ptr.To("Original description"),
	})
	require.NoError(t, err)

	versionID, err := store.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	stepMap := oapi.JourneyStepMap{
		"step-1": {
			Type: "entrance",
			Name: ptr.To("Welcome"),
			X:    100,
			Y:    200,
			Children: []oapi.JourneyStepChild{
				{ExternalId: "step-2"},
			},
		},
		"step-2": {
			Type: "exit",
			Name: ptr.To("Goodbye"),
			X:    300,
			Y:    400,
		},
	}

	_, err = store.SetJourneySteps(ctx, versionID, stepMap)
	require.NoError(t, err)

	err = store.PublishVersion(ctx, journeyID, versionID)
	require.NoError(t, err)

	journey, err := store.GetJourney(ctx, projectID, journeyID)
	require.NoError(t, err)

	t.Run("duplicates as new journey", func(t *testing.T) {
		newJourneyID, err := store.DuplicateJourney(ctx, journey, false)
		require.NoError(t, err)
		assert.NotEqual(t, journeyID, newJourneyID)

		newJourney, err := store.GetJourney(ctx, projectID, newJourneyID)
		require.NoError(t, err)
		assert.Equal(t, "Copy of Original Journey", newJourney.Name)
		assert.NotNil(t, newJourney.VersionID)

		steps, err := store.GetJourneyVersionSteps(ctx, *newJourney.VersionID)
		require.NoError(t, err)
		assert.Len(t, steps, 2)
	})

	t.Run("duplicates as version", func(t *testing.T) {
		newJourneyID, err := store.DuplicateJourney(ctx, journey, true)
		require.NoError(t, err)
		assert.Equal(t, journeyID, newJourneyID)

		draftVersion, err := store.GetLatestDraftVersion(ctx, journeyID)
		require.NoError(t, err)
		assert.Equal(t, 2, draftVersion.VersionNumber)

		steps, err := store.GetJourneyVersionSteps(ctx, draftVersion.ID)
		require.NoError(t, err)
		assert.Len(t, steps, 2)
	})
}

func TestJourneysStoreEventDependencies(t *testing.T) {
	t.Parallel()

	store, db := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()

	journeyID, err := store.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	versionID, err := store.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	eventID := uuid.New()

	t.Run("sets event dependencies for step", func(t *testing.T) {
		stepMap := oapi.JourneyStepMap{
			"entrance-1": {
				Type: "entrance",
				Name: ptr.To("Signup Entrance"),
				X:    100,
				Y:    200,
			},
		}

		_, err := store.SetJourneySteps(ctx, versionID, stepMap)
		require.NoError(t, err)

		err = store.SetJourneyStepEventDependencies(ctx, versionID, "entrance-1", []uuid.UUID{eventID})
		require.NoError(t, err)

		var count int
		err = db.GetContext(ctx, &count,
			`SELECT COUNT(*) FROM journey_version_step_events WHERE version_id = $1 AND external_id = $2 AND event_id = $3`,
			versionID, "entrance-1", eventID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

func TestJourneysStoreUserJourneyState(t *testing.T) {
	t.Parallel()

	store, _ := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()
	davidID := uuid.New()

	journeyID, err := store.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	versionID, err := store.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	stepMap := oapi.JourneyStepMap{
		"entrance-1": {
			Type: "entrance",
			X:    100,
			Y:    200,
			Children: []oapi.JourneyStepChild{
				{ExternalId: "step-2"},
			},
		},
		"step-2": {
			Type: "campaign",
			X:    300,
			Y:    400,
		},
	}

	_, err = store.SetJourneySteps(ctx, versionID, stepMap)
	require.NoError(t, err)

	err = store.PublishVersion(ctx, journeyID, versionID)
	require.NoError(t, err)

	t.Run("creates user journey state", func(t *testing.T) {
		davidEntryID := uuid.New()
		stateID, err := store.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: davidEntryID,
			UserID:         davidID,
			ExternalStepID: "entrance-1",
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, stateID)

		state, err := store.GetUserJourneyState(ctx, davidEntryID, "entrance-1")
		require.NoError(t, err)
		assert.Equal(t, journeyID, state.JourneyID)
		assert.Equal(t, davidID, state.UserID)
		assert.Equal(t, "entrance-1", state.ExternalStepID)
		assert.Nil(t, state.PinnedVersionID)
	})
}

func TestJourneysStoreVersionPinning(t *testing.T) {
	t.Parallel()

	store, _ := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()
	johnID := uuid.New()

	journeyID, err := store.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	version1ID, err := store.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	stepMapV1 := oapi.JourneyStepMap{
		"step-1": {
			Type: "entrance",
			X:    100,
			Y:    200,
			Children: []oapi.JourneyStepChild{
				{ExternalId: "step-2"},
			},
		},
		"step-2": {
			Type: "exit",
			X:    300,
			Y:    400,
		},
	}

	_, err = store.SetJourneySteps(ctx, version1ID, stepMapV1)
	require.NoError(t, err)

	err = store.PublishVersion(ctx, journeyID, version1ID)
	require.NoError(t, err)

	johnEntryID := uuid.New()
	_, err = store.CreateUserJourneyState(ctx, JourneyUserState{
		JourneyID:       journeyID,
		JourneyEntryID:  johnEntryID,
		UserID:          johnID,
		ExternalStepID:  "step-1",
		PinnedVersionID: &version1ID,
	})
	require.NoError(t, err)

	version2ID, err := store.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	stepMapV2 := oapi.JourneyStepMap{
		"step-1": {
			Type: "entrance",
			X:    100,
			Y:    200,
			Children: []oapi.JourneyStepChild{
				{ExternalId: "step-3"},
			},
		},
		"step-3": {
			Type: "campaign",
			X:    300,
			Y:    400,
		},
	}

	_, err = store.SetJourneySteps(ctx, version2ID, stepMapV2)
	require.NoError(t, err)

	err = store.PublishVersion(ctx, journeyID, version2ID)
	require.NoError(t, err)

	t.Run("pinned user stays on version 1", func(t *testing.T) {
		state, err := store.GetUserJourneyState(ctx, johnEntryID, "step-1")
		require.NoError(t, err)
		assert.NotNil(t, state.PinnedVersionID)
		assert.Equal(t, version1ID, *state.PinnedVersionID)
	})

	t.Run("non-pinned user uses latest version", func(t *testing.T) {
		oxanaID := uuid.New()

		oxanaEntryID := uuid.New()
		_, err := store.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:       journeyID,
			JourneyEntryID:  oxanaEntryID,
			UserID:          oxanaID,
			ExternalStepID:  "step-1",
			PinnedVersionID: nil,
		})
		require.NoError(t, err)

		state, err := store.GetUserJourneyState(ctx, oxanaEntryID, "step-1")
		require.NoError(t, err)
		assert.Nil(t, state.PinnedVersionID)
	})
}

func TestJourneysStoreEnsureDraftVersionCopiesSteps(t *testing.T) {
	t.Parallel()

	store, _ := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()

	journeyID, err := store.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	versionID, err := store.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	stepMap := oapi.JourneyStepMap{
		"step-1": {
			Type: "entrance",
			Name: ptr.To("Entrance"),
			X:    100,
			Y:    100,
			Children: []oapi.JourneyStepChild{
				{ExternalId: "step-2"},
			},
		},
		"step-2": {
			Type: "email",
			Name: ptr.To("Send Email"),
			X:    200,
			Y:    200,
		},
	}

	_, err = store.SetJourneySteps(ctx, versionID, stepMap)
	require.NoError(t, err)

	err = store.PublishVersion(ctx, journeyID, versionID)
	require.NoError(t, err)

	newDraftID, err := store.EnsureDraftVersion(ctx, journeyID)
	require.NoError(t, err)
	assert.NotEqual(t, versionID, newDraftID, "should create a new draft version")

	draftSteps, err := store.GetJourneyVersionSteps(ctx, newDraftID)
	require.NoError(t, err)
	assert.Len(t, draftSteps, 2, "new draft should have copied steps from published version")

	draftChildren, err := store.GetJourneyVersionStepsChildren(ctx, newDraftID)
	require.NoError(t, err)
	assert.Len(t, draftChildren, 1, "connections should be copied to new draft")
}

func TestCheckEntryEligibility(t *testing.T) {
	t.Parallel()

	store, _ := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New()

	journeyID, err := store.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Entry Eligibility Journey",
	})
	require.NoError(t, err)

	versionID, err := store.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	stepMap := oapi.JourneyStepMap{
		"entrance-1": {
			Type: "entrance",
			X:    0,
			Y:    0,
			Children: []oapi.JourneyStepChild{
				{ExternalId: "step-2"},
			},
		},
		"step-2": {
			Type: "exit",
			X:    100,
			Y:    100,
		},
	}

	_, err = store.SetJourneySteps(ctx, versionID, stepMap)
	require.NoError(t, err)

	err = store.PublishVersion(ctx, journeyID, versionID)
	require.NoError(t, err)

	t.Run("allows first entry when multiple=false", func(t *testing.T) {
		newUserID := uuid.New()
		eligible, err := store.CheckEntryEligibility(ctx, journeyID, newUserID, "entrance-1", false, false)
		require.NoError(t, err)
		assert.True(t, eligible)
	})

	t.Run("blocks re-entry when multiple=false", func(t *testing.T) {
		entryID := uuid.New()
		completedAt := time.Now()
		_, err := store.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: entryID,
			UserID:         userID,
			ExternalStepID: "entrance-1",
			CompletedAt:    &completedAt,
		})
		require.NoError(t, err)

		eligible, err := store.CheckEntryEligibility(ctx, journeyID, userID, "entrance-1", false, false)
		require.NoError(t, err)
		assert.False(t, eligible)
	})

	t.Run("blocks re-entry when multiple=false even with concurrent=true", func(t *testing.T) {
		eligible, err := store.CheckEntryEligibility(ctx, journeyID, userID, "entrance-1", false, true)
		require.NoError(t, err)
		assert.False(t, eligible)
	})

	t.Run("allows re-entry when multiple=true and prior entry completed", func(t *testing.T) {
		eligible, err := store.CheckEntryEligibility(ctx, journeyID, userID, "entrance-1", true, false)
		require.NoError(t, err)
		assert.True(t, eligible)
	})

	t.Run("blocks concurrent entry when multiple=true concurrent=false and entry is active", func(t *testing.T) {
		activeUserID := uuid.New()
		entryID := uuid.New()
		// Create an active entry (no completed_at)
		_, err := store.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: entryID,
			UserID:         activeUserID,
			ExternalStepID: "entrance-1",
		})
		require.NoError(t, err)

		eligible, err := store.CheckEntryEligibility(ctx, journeyID, activeUserID, "entrance-1", true, false)
		require.NoError(t, err)
		assert.False(t, eligible)
	})

	t.Run("allows concurrent entry when both multiple=true and concurrent=true", func(t *testing.T) {
		concurrentUserID := uuid.New()
		entryID := uuid.New()
		// Create an active entry
		_, err := store.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: entryID,
			UserID:         concurrentUserID,
			ExternalStepID: "entrance-1",
		})
		require.NoError(t, err)

		eligible, err := store.CheckEntryEligibility(ctx, journeyID, concurrentUserID, "entrance-1", true, true)
		require.NoError(t, err)
		assert.True(t, eligible)
	})

	t.Run("different journey does not affect eligibility", func(t *testing.T) {
		otherJourneyID, err := store.CreateJourney(ctx, Journey{
			ProjectID: projectID,
			Name:      "Other Journey",
		})
		require.NoError(t, err)

		// userID already has an entry in the first journey, but not in this one
		eligible, err := store.CheckEntryEligibility(ctx, otherJourneyID, userID, "entrance-1", false, false)
		require.NoError(t, err)
		assert.True(t, eligible)
	})
}

func TestJourneysStoreMultiExecutionSteps(t *testing.T) {
	t.Parallel()

	store, _ := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()
	emmaID := uuid.New()

	journeyID, err := store.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	journeyEntryID := uuid.New()

	t.Run("creates multiple executions of same step", func(t *testing.T) {
		id1, err := store.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: journeyEntryID,
			UserID:         emmaID,
			ExternalStepID: "gate-1",
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id1)

		id2, err := store.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: journeyEntryID,
			UserID:         emmaID,
			ExternalStepID: "gate-1",
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id2)

		assert.NotEqual(t, id1, id2)
	})

	t.Run("gets latest execution when no state_id specified", func(t *testing.T) {
		latest, err := store.GetUserJourneyState(ctx, journeyEntryID, "gate-1")
		require.NoError(t, err)
		assert.Equal(t, "gate-1", latest.ExternalStepID)
		assert.Equal(t, 2, latest.Occurrence)
	})

	t.Run("gets specific execution by state_id", func(t *testing.T) {
		id3, err := store.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: journeyEntryID,
			UserID:         emmaID,
			ExternalStepID: "gate-1",
		})
		require.NoError(t, err)

		state, err := store.GetJourneyStateByID(ctx, id3)
		require.NoError(t, err)
		assert.Equal(t, id3, state.ID)
		assert.Equal(t, 3, state.Occurrence)
	})

	t.Run("updates existing state with ON CONFLICT", func(t *testing.T) {
		id4, err := store.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: journeyEntryID,
			UserID:         emmaID,
			ExternalStepID: "delay-1",
		})
		require.NoError(t, err)

		state, err := store.GetJourneyStateByID(ctx, id4)
		require.NoError(t, err)
		assert.Equal(t, id4, state.ID)

		state.Data = []byte(`{"updated": true}`)
		updatedID, err := store.CreateUserJourneyState(ctx, *state)
		require.NoError(t, err)
		assert.Equal(t, id4, updatedID)

		reloaded, err := store.GetJourneyStateByID(ctx, id4)
		require.NoError(t, err)
		assert.JSONEq(t, `{"updated": true}`, string(reloaded.Data))
	})
}

package store

import (
	"context"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/container"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type testState struct {
	*State
	db DB
}

func newTestStore(t *testing.T) *testState {
	t.Helper()

	logger := zaptest.NewLogger(t)

	ctx := graceful.NewContext(t.Context())
	config := Config{
		URI: container.RunPostgreSQL(t),
	}

	err := Migrate(config)
	require.NoError(t, err)

	db, err := New(ctx, logger, config)
	require.NoError(t, err)

	return &testState{
		State: NewState(db),
		db:    db,
	}
}

func TestJourneysStoreCreateJourney(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	t.Run("creates journey with initial draft version", func(t *testing.T) {
		journeyID, err := db.CreateJourney(ctx, Journey{
			ProjectID:   projectID,
			Name:        "Onboarding Journey",
			Description: ptr("Welcome new users"),
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, journeyID)

		journey, err := db.GetJourney(ctx, projectID, journeyID)
		require.NoError(t, err)
		assert.Equal(t, "Onboarding Journey", journey.Name)
		assert.Equal(t, "Welcome new users", *journey.Description)
		assert.Nil(t, journey.VersionID) // No version yet
	})
}

func TestJourneysStoreVersionWorkflow(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	journeyID, err := db.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	t.Run("creates initial draft version", func(t *testing.T) {
		versionID, err := db.CreateJourneyVersion(ctx, journeyID, "draft")
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, versionID)

		version, err := db.GetJourneyVersion(ctx, versionID)
		require.NoError(t, err)
		assert.Equal(t, journeyID, version.JourneyID)
		assert.Equal(t, 1, version.VersionNumber)
		assert.Equal(t, "draft", version.Status)
		assert.Nil(t, version.PublishedAt)
	})

	t.Run("publishes draft version", func(t *testing.T) {
		draftVersion, err := db.GetLatestDraftVersion(ctx, journeyID)
		require.NoError(t, err)

		err = db.PublishVersion(ctx, journeyID, draftVersion.ID)
		require.NoError(t, err)

		version, err := db.GetJourneyVersion(ctx, draftVersion.ID)
		require.NoError(t, err)
		assert.Equal(t, "published", version.Status)
		assert.NotNil(t, version.PublishedAt)

		journey, err := db.GetJourney(ctx, projectID, journeyID)
		require.NoError(t, err)
		assert.Equal(t, draftVersion.ID, *journey.VersionID)

		currentVersion, err := db.GetCurrentVersion(ctx, journeyID)
		require.NoError(t, err)
		assert.Equal(t, draftVersion.ID, currentVersion.ID)
	})

	t.Run("creates new draft version after publish", func(t *testing.T) {
		versionID, err := db.CreateJourneyVersion(ctx, journeyID, "draft")
		require.NoError(t, err)

		version, err := db.GetJourneyVersion(ctx, versionID)
		require.NoError(t, err)
		assert.Equal(t, 2, version.VersionNumber)
		assert.Equal(t, "draft", version.Status)
	})

	t.Run("archives old version when publishing new one", func(t *testing.T) {
		draftVersion, err := db.GetLatestDraftVersion(ctx, journeyID)
		require.NoError(t, err)

		err = db.PublishVersion(ctx, journeyID, draftVersion.ID)
		require.NoError(t, err)

		// Old published version should be archived
		query := `SELECT status FROM journey_versions WHERE journey_id = $1 AND version_number = 1`
		var status string
		err = db.db.GetContext(ctx, &status, query, journeyID)
		require.NoError(t, err)
		assert.Equal(t, "archived", status)
	})
}

func TestJourneysStoreSetJourneySteps(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	journeyID, err := db.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	versionID, err := db.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	t.Run("creates new steps and children", func(t *testing.T) {
		stepMap := oapi.JourneyStepMap{
			"step-1": {
				Type: "entrance",
				Name: ptr("Welcome"),
				X:    100,
				Y:    200,
				Children: []oapi.JourneyStepChild{
					{ExternalId: "step-2"},
				},
			},
			"step-2": {
				Type: "exit",
				Name: ptr("Goodbye"),
				X:    300,
				Y:    400,
			},
		}

		stepIDs, err := db.SetJourneySteps(ctx, versionID, stepMap)
		require.NoError(t, err)
		assert.Len(t, stepIDs, 2)

		steps, err := db.GetJourneyVersionSteps(ctx, versionID)
		require.NoError(t, err)
		assert.Len(t, steps, 2)

		children, err := db.GetJourneyVersionStepsChildren(ctx, versionID)
		require.NoError(t, err)
		assert.Len(t, children, 1)
		assert.Equal(t, "step-1", children[0].ParentExternalID)
		assert.Equal(t, "step-2", children[0].ChildExternalID)
	})

	t.Run("updates existing steps", func(t *testing.T) {
		stepMap := oapi.JourneyStepMap{
			"step-1": {
				Type: "entrance",
				Name: ptr("Updated Welcome"),
				X:    150,
				Y:    250,
			},
			"step-2": {
				Type: "exit",
				Name: ptr("Updated Goodbye"),
				X:    350,
				Y:    450,
			},
		}

		_, err := db.SetJourneySteps(ctx, versionID, stepMap)
		require.NoError(t, err)

		steps, err := db.GetJourneyVersionSteps(ctx, versionID)
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
				Name: ptr("Welcome"),
				X:    100,
				Y:    200,
			},
		}

		_, err := db.SetJourneySteps(ctx, versionID, stepMap)
		require.NoError(t, err)

		steps, err := db.GetJourneyVersionSteps(ctx, versionID)
		require.NoError(t, err)
		assert.Len(t, steps, 1)
		assert.Equal(t, "step-1", steps[0].ExternalID)
	})
}

func TestJourneysStoreDuplicateJourney(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	journeyID, err := db.CreateJourney(ctx, Journey{
		ProjectID:   projectID,
		Name:        "Original Journey",
		Description: ptr("Original description"),
	})
	require.NoError(t, err)

	versionID, err := db.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	stepMap := oapi.JourneyStepMap{
		"step-1": {
			Type: "entrance",
			Name: ptr("Welcome"),
			X:    100,
			Y:    200,
			Children: []oapi.JourneyStepChild{
				{ExternalId: "step-2"},
			},
		},
		"step-2": {
			Type: "exit",
			Name: ptr("Goodbye"),
			X:    300,
			Y:    400,
		},
	}

	_, err = db.SetJourneySteps(ctx, versionID, stepMap)
	require.NoError(t, err)

	err = db.PublishVersion(ctx, journeyID, versionID)
	require.NoError(t, err)

	journey, err := db.GetJourney(ctx, projectID, journeyID)
	require.NoError(t, err)

	t.Run("duplicates as new journey", func(t *testing.T) {
		newJourneyID, err := db.DuplicateJourney(ctx, journey, false)
		require.NoError(t, err)
		assert.NotEqual(t, journeyID, newJourneyID)

		newJourney, err := db.GetJourney(ctx, projectID, newJourneyID)
		require.NoError(t, err)
		assert.Equal(t, "Copy of Original Journey", newJourney.Name)
		assert.NotNil(t, newJourney.VersionID)

		// Check steps were copied
		steps, err := db.GetJourneyVersionSteps(ctx, *newJourney.VersionID)
		require.NoError(t, err)
		assert.Len(t, steps, 2)
	})

	t.Run("duplicates as version", func(t *testing.T) {
		newJourneyID, err := db.DuplicateJourney(ctx, journey, true)
		require.NoError(t, err)
		assert.Equal(t, journeyID, newJourneyID)

		// Should create new version
		draftVersion, err := db.GetLatestDraftVersion(ctx, journeyID)
		require.NoError(t, err)
		assert.Equal(t, 2, draftVersion.VersionNumber)

		// Check steps were copied
		steps, err := db.GetJourneyVersionSteps(ctx, draftVersion.ID)
		require.NoError(t, err)
		assert.Len(t, steps, 2)
	})
}

func TestJourneysStoreEventDependencies(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	journeyID, err := db.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	versionID, err := db.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	eventID := uuid.New()
	_, err = db.db.ExecContext(ctx, `INSERT INTO events (id, project_id, name) VALUES ($1, $2, $3)`,
		eventID, projectID, "user_signup")
	require.NoError(t, err)

	t.Run("sets event dependencies for step", func(t *testing.T) {
		stepMap := oapi.JourneyStepMap{
			"entrance-1": {
				Type: "entrance",
				Name: ptr("Signup Entrance"),
				X:    100,
				Y:    200,
			},
		}

		_, err := db.SetJourneySteps(ctx, versionID, stepMap)
		require.NoError(t, err)

		err = db.SetJourneyStepEventDependencies(ctx, versionID, "entrance-1", []uuid.UUID{eventID})
		require.NoError(t, err)

		// Verify dependency was created
		var count int
		err = db.db.GetContext(ctx, &count,
			`SELECT COUNT(*) FROM journey_version_step_events WHERE version_id = $1 AND external_id = $2 AND event_id = $3`,
			versionID, "entrance-1", eventID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("publishes version and queries entrance steps", func(t *testing.T) {
		err := db.PublishVersion(ctx, journeyID, versionID)
		require.NoError(t, err)

		eventsStore := NewEventsStore(db.db)
		entrances, err := eventsStore.ListEventJourneyDependencies(ctx, eventID)
		require.NoError(t, err)
		assert.Len(t, entrances, 1)
		assert.Equal(t, journeyID, entrances[0].JourneyID)
		assert.Equal(t, "entrance-1", entrances[0].ExternalID)
		assert.Equal(t, "entrance", entrances[0].Type)
	})
}

func TestJourneysStoreUserJourneyState(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	userID := uuid.New()
	_, err = db.db.ExecContext(ctx, `INSERT INTO users (id, project_id, external_id) VALUES ($1, $2, $3)`,
		userID, projectID, "user-123")
	require.NoError(t, err)

	journeyID, err := db.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	versionID, err := db.CreateJourneyVersion(ctx, journeyID, "draft")
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
			Type: "action",
			X:    300,
			Y:    400,
		},
	}

	_, err = db.SetJourneySteps(ctx, versionID, stepMap)
	require.NoError(t, err)

	err = db.PublishVersion(ctx, journeyID, versionID)
	require.NoError(t, err)

	t.Run("creates user journey state", func(t *testing.T) {
		journeyEntryID := uuid.New()
		stateID, err := db.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: journeyEntryID,
			UserID:         userID,
			ExternalStepID: "entrance-1",
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, stateID)

		state, err := db.GetUserJourneyState(ctx, journeyEntryID, "entrance-1")
		require.NoError(t, err)
		assert.Equal(t, journeyID, state.JourneyID)
		assert.Equal(t, userID, state.UserID)
		assert.Equal(t, "entrance-1", state.ExternalStepID)
		assert.Nil(t, state.PinnedVersionID)
	})
}

func TestJourneysStoreVersionPinning(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	userID := uuid.New()
	_, err = db.db.ExecContext(ctx, `INSERT INTO users (id, project_id, external_id) VALUES ($1, $2, $3)`,
		userID, projectID, "user-123")
	require.NoError(t, err)

	journeyID, err := db.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	// Create and publish version 1
	version1ID, err := db.CreateJourneyVersion(ctx, journeyID, "draft")
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

	_, err = db.SetJourneySteps(ctx, version1ID, stepMapV1)
	require.NoError(t, err)

	err = db.PublishVersion(ctx, journeyID, version1ID)
	require.NoError(t, err)

	// User enters journey on version 1 with pinning
	journeyEntryID := uuid.New()
	_, err = db.CreateUserJourneyState(ctx, JourneyUserState{
		JourneyID:       journeyID,
		JourneyEntryID:  journeyEntryID,
		UserID:          userID,
		ExternalStepID:  "step-1",
		PinnedVersionID: &version1ID,
	})
	require.NoError(t, err)

	// Create and publish version 2
	version2ID, err := db.CreateJourneyVersion(ctx, journeyID, "draft")
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
			Type: "action",
			X:    300,
			Y:    400,
		},
	}

	_, err = db.SetJourneySteps(ctx, version2ID, stepMapV2)
	require.NoError(t, err)

	err = db.PublishVersion(ctx, journeyID, version2ID)
	require.NoError(t, err)

	t.Run("pinned user stays on version 1", func(t *testing.T) {
		state, err := db.GetUserJourneyState(ctx, journeyEntryID, "step-1")
		require.NoError(t, err)
		assert.NotNil(t, state.PinnedVersionID)
		assert.Equal(t, version1ID, *state.PinnedVersionID)
	})

	t.Run("non-pinned user uses latest version", func(t *testing.T) {
		user2ID := uuid.New()
		_, err := db.db.ExecContext(ctx, `INSERT INTO users (id, project_id, external_id) VALUES ($1, $2, $3)`,
			user2ID, projectID, "user-456")
		require.NoError(t, err)

		journeyEntry2ID := uuid.New()
		_, err = db.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:       journeyID,
			JourneyEntryID:  journeyEntry2ID,
			UserID:          user2ID,
			ExternalStepID:  "step-1",
			PinnedVersionID: nil, // No pinning
		})
		require.NoError(t, err)

		state2, err := db.GetUserJourneyState(ctx, journeyEntry2ID, "step-1")
		require.NoError(t, err)
		assert.Nil(t, state2.PinnedVersionID)
	})
}

func TestJourneysStoreEnsureDraftVersionCopiesSteps(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()
	projectID := uuid.New()

	// Create project first
	_, err := db.db.ExecContext(ctx, `INSERT INTO projects (id, name) VALUES ($1, 'Test Project')`, projectID)
	require.NoError(t, err)

	// Create journey with a published version that has steps
	journeyID, err := db.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	// Create and publish a version with steps
	versionID, err := db.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)

	// Add steps directly to database
	_, err = db.db.ExecContext(ctx, `
		INSERT INTO journey_version_steps (version_id, external_id, type, name, x, y)
		VALUES
			($1, 'step-1', 'entrance', 'Entrance', 100, 100),
			($1, 'step-2', 'email', 'Send Email', 200, 200)
	`, versionID)
	require.NoError(t, err)

	// Add a connection
	_, err = db.db.ExecContext(ctx, `
		INSERT INTO journey_version_step_children (version_id, parent_external_id, child_external_id)
		VALUES ($1, 'step-2', 'step-1')
	`, versionID)
	require.NoError(t, err)

	// Publish the version
	err = db.PublishVersion(ctx, journeyID, versionID)
	require.NoError(t, err)

	// Now ensure draft version (should create new draft with copied steps)
	newDraftID, err := db.EnsureDraftVersion(ctx, journeyID)
	require.NoError(t, err)
	assert.NotEqual(t, versionID, newDraftID, "should create a new draft version")

	// Verify the new draft has the copied steps
	draftSteps, err := db.GetJourneyVersionSteps(ctx, newDraftID)
	require.NoError(t, err)
	assert.Len(t, draftSteps, 2, "new draft should have copied steps from published version")

	// Verify children were copied
	draftChildren, err := db.GetJourneyVersionStepsChildren(ctx, newDraftID)
	require.NoError(t, err)
	assert.Len(t, draftChildren, 1, "connections should be copied to new draft")
}

func TestJourneysStoreMultiExecutionSteps(t *testing.T) {
	db := newTestStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	userID := uuid.New()
	_, err = db.db.ExecContext(ctx, `INSERT INTO users (id, project_id, external_id) VALUES ($1, $2, $3)`,
		userID, projectID, "user-123")
	require.NoError(t, err)

	journeyID, err := db.CreateJourney(ctx, Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	journeyEntryID := uuid.New()

	t.Run("creates multiple executions of same step", func(t *testing.T) {
		// First execution
		id1, err := db.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: journeyEntryID,
			UserID:         userID,
			ExternalStepID: "gate-1",
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id1)

		// Second execution of the same step
		id2, err := db.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: journeyEntryID,
			UserID:         userID,
			ExternalStepID: "gate-1",
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, id2)

		// IDs should be different (different execution instances)
		assert.NotEqual(t, id1, id2)
	})

	t.Run("gets latest execution when no state_id specified", func(t *testing.T) {
		// Get latest execution
		latest, err := db.GetUserJourneyState(ctx, journeyEntryID, "gate-1")
		require.NoError(t, err)
		assert.Equal(t, "gate-1", latest.ExternalStepID)
		assert.Equal(t, 2, latest.Occurrence) // Should be the second execution
	})

	t.Run("gets specific execution by state_id", func(t *testing.T) {
		// Create third execution
		id3, err := db.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: journeyEntryID,
			UserID:         userID,
			ExternalStepID: "gate-1",
		})
		require.NoError(t, err)

		// Get specific execution by ID
		state, err := db.GetJourneyStateByID(ctx, id3)
		require.NoError(t, err)
		assert.Equal(t, id3, state.ID)
		assert.Equal(t, 3, state.Occurrence)
	})

	t.Run("updates existing state with ON CONFLICT", func(t *testing.T) {
		// Create execution
		id4, err := db.CreateUserJourneyState(ctx, JourneyUserState{
			JourneyID:      journeyID,
			JourneyEntryID: journeyEntryID,
			UserID:         userID,
			ExternalStepID: "delay-1",
		})
		require.NoError(t, err)

		// Load state and update it (simulating delay resume)
		state, err := db.GetJourneyStateByID(ctx, id4)
		require.NoError(t, err)
		assert.Equal(t, id4, state.ID)

		// Update by passing the same ID with new data
		state.Data = []byte(`{"updated": true}`)
		updatedID, err := db.CreateUserJourneyState(ctx, *state)
		require.NoError(t, err)
		assert.Equal(t, id4, updatedID)

		// Verify state was updated, not duplicated
		reloaded, err := db.GetJourneyStateByID(ctx, id4)
		require.NoError(t, err)
		assert.JSONEq(t, `{"updated": true}`, string(reloaded.Data))
	})
}

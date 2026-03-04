package consumer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupOrganizationsTest(t *testing.T) (mgmtDB, usrsDB, jrnyDB *sqlx.DB, projectID uuid.UUID, jet jetstream.JetStream) {
	t.Helper()

	ctx := graceful.NewContext(t.Context())

	natsURL := container.RunNATS(t)
	mgmtDB, usrsDB, jrnyDB = teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Nats: config.Nats{
			URL: natsURL,
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	mgmtState := management.NewState(mgmtDB)

	orgID, err := mgmtState.OrganizationsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err = mgmtState.ProjectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	return mgmtDB, usrsDB, jrnyDB, projectID, jet
}

func TestOrganizationsHandlerSuccess(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	pub := pubsub.NewPublisher(jet)
	handler := OrganizationsHandler(logger, usersState, pub)

	orgID := uuid.New()
	org := schemas.Organization{
		ID:         orgID,
		ProjectID:  projectID,
		ExternalID: "org_123",
		Name:       strPtr("Test Organization"),
		Data: map[string]any{
			"industry": "technology",
		},
		Version: 0, // New organization
	}

	err = pub.Publish(ctx, schemas.OrganizationsProcess(projectID), org)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamOrganizations, ConsumerOrganizationsProcess)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// Verify schema message was published (since Data was provided)
	schemaConsumer, err := jet.Consumer(ctx, StreamOrganizations, ConsumerOrganizationsSchema)
	require.NoError(t, err)

	schemaMsg, err := schemaConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedOrg schemas.Organization
	err = json.Unmarshal(schemaMsg.Data(), &receivedOrg)
	require.NoError(t, err)
	assert.Equal(t, orgID, receivedOrg.ID)
	assert.Equal(t, "org_123", receivedOrg.ExternalID)

	// Verify organization.created event was published
	orgEventsConsumer, err := jet.Consumer(ctx, StreamOrganizationEvents, ConsumerOrganizationEventsProcess)
	require.NoError(t, err)

	orgEventMsg, err := orgEventsConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent schemas.OrganizationEvent
	err = json.Unmarshal(orgEventMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.Equal(t, schemas.EventOrganizationCreated, receivedEvent.Name)
	assert.Equal(t, orgID, receivedEvent.OrganizationID)
}

func TestOrganizationsHandlerUpdatedEvent(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	pub := pubsub.NewPublisher(jet)
	handler := OrganizationsHandler(logger, usersState, pub)

	orgID := uuid.New()
	org := schemas.Organization{
		ID:         orgID,
		ProjectID:  projectID,
		ExternalID: "org_456",
		Name:       strPtr("Updated Organization"),
		Data: map[string]any{
			"industry": "finance",
		},
		Version: 1, // Existing organization (updated)
	}

	err = pub.Publish(ctx, schemas.OrganizationsProcess(projectID), org)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamOrganizations, ConsumerOrganizationsProcess)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// Verify organization.updated event was published (not created)
	orgEventsConsumer, err := jet.Consumer(ctx, StreamOrganizationEvents, ConsumerOrganizationEventsProcess)
	require.NoError(t, err)

	orgEventMsg, err := orgEventsConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent schemas.OrganizationEvent
	err = json.Unmarshal(orgEventMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.Equal(t, schemas.EventOrganizationUpdated, receivedEvent.Name)
}

func TestOrganizationsHandlerWithoutData(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	pub := pubsub.NewPublisher(jet)
	handler := OrganizationsHandler(logger, usersState, pub)

	orgID := uuid.New()
	org := schemas.Organization{
		ID:         orgID,
		ProjectID:  projectID,
		ExternalID: "org_no_data",
		Name:       strPtr("No Data Org"),
		Data:       nil, // No data
		Version:    0,
	}

	err = pub.Publish(ctx, schemas.OrganizationsProcess(projectID), org)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamOrganizations, ConsumerOrganizationsProcess)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// Verify NO schema message was published (since Data is nil)
	schemaConsumer, err := jet.Consumer(ctx, StreamOrganizations, ConsumerOrganizationsSchema)
	require.NoError(t, err)

	_, err = schemaConsumer.Next(jetstream.FetchMaxWait(1 * time.Second))
	assert.Error(t, err) // Should timeout
}

func TestOrganizationSchemasHandlerSuccess(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	handler := OrganizationSchemasHandler(logger, usersState)

	orgID := uuid.New()
	org := schemas.Organization{
		ID:         orgID,
		ProjectID:  projectID,
		ExternalID: "org_schema_test",
		Data: map[string]any{
			"company": map[string]any{
				"name":     "Acme Corp",
				"industry": "manufacturing",
			},
			"employee_count": 500,
		},
	}

	err = pubsub.NewPublisher(jet).Publish(ctx, schemas.OrganizationsSchema(projectID), org)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamOrganizations, ConsumerOrganizationsSchema)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)
}

func TestOrganizationUsersHandlerSuccess(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	pub := pubsub.NewPublisher(jet)
	handler := OrganizationUsersHandler(logger, usersState, pub)

	orgID := uuid.New()
	userID := uuid.New()
	orgUser := schemas.OrganizationUser{
		OrganizationID:         orgID,
		OrganizationExternalID: "org_user_test",
		UserID:                 userID,
		ProjectID:              projectID,
		Data: map[string]any{
			"role": "admin",
		},
		Version: 0, // New membership
	}

	err = pub.Publish(ctx, schemas.OrganizationUsersProcess(projectID), orgUser)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamOrganizationUsers, ConsumerOrganizationUsersProcess)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// Verify schema message was published (since Data was provided)
	schemaConsumer, err := jet.Consumer(ctx, StreamOrganizationUsers, ConsumerOrganizationUsersSchema)
	require.NoError(t, err)

	schemaMsg, err := schemaConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedOrgUser schemas.OrganizationUser
	err = json.Unmarshal(schemaMsg.Data(), &receivedOrgUser)
	require.NoError(t, err)
	assert.Equal(t, orgID, receivedOrgUser.OrganizationID)
	assert.Equal(t, userID, receivedOrgUser.UserID)

	// Verify organization.user.added event was published
	orgEventsConsumer, err := jet.Consumer(ctx, StreamOrganizationEvents, ConsumerOrganizationEventsProcess)
	require.NoError(t, err)

	orgEventMsg, err := orgEventsConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent schemas.OrganizationEvent
	err = json.Unmarshal(orgEventMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.Equal(t, schemas.EventOrganizationUserAdded, receivedEvent.Name)
}

func TestOrganizationUsersHandlerUpdatedEvent(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	pub := pubsub.NewPublisher(jet)
	handler := OrganizationUsersHandler(logger, usersState, pub)

	orgID := uuid.New()
	userID := uuid.New()
	orgUser := schemas.OrganizationUser{
		OrganizationID:         orgID,
		OrganizationExternalID: "org_user_update",
		UserID:                 userID,
		ProjectID:              projectID,
		Data: map[string]any{
			"role": "member",
		},
		Version: 1, // Existing membership (updated)
	}

	err = pub.Publish(ctx, schemas.OrganizationUsersProcess(projectID), orgUser)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamOrganizationUsers, ConsumerOrganizationUsersProcess)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// Verify organization.user.updated event was published (not added)
	orgEventsConsumer, err := jet.Consumer(ctx, StreamOrganizationEvents, ConsumerOrganizationEventsProcess)
	require.NoError(t, err)

	orgEventMsg, err := orgEventsConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent schemas.OrganizationEvent
	err = json.Unmarshal(orgEventMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.Equal(t, schemas.EventOrganizationUserUpdated, receivedEvent.Name)
}

func TestOrganizationUsersHandlerWithoutData(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	pub := pubsub.NewPublisher(jet)
	handler := OrganizationUsersHandler(logger, usersState, pub)

	orgID := uuid.New()
	userID := uuid.New()
	orgUser := schemas.OrganizationUser{
		OrganizationID:         orgID,
		OrganizationExternalID: "org_user_no_data",
		UserID:                 userID,
		ProjectID:              projectID,
		Data:                   nil, // No data
		Version:                0,
	}

	err = pub.Publish(ctx, schemas.OrganizationUsersProcess(projectID), orgUser)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamOrganizationUsers, ConsumerOrganizationUsersProcess)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// Verify NO schema message was published (since Data is nil)
	schemaConsumer, err := jet.Consumer(ctx, StreamOrganizationUsers, ConsumerOrganizationUsersSchema)
	require.NoError(t, err)

	_, err = schemaConsumer.Next(jetstream.FetchMaxWait(1 * time.Second))
	assert.Error(t, err) // Should timeout
}

func TestOrganizationUserSchemasHandlerSuccess(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	handler := OrganizationUserSchemasHandler(logger, usersState)

	orgID := uuid.New()
	userID := uuid.New()
	orgUser := schemas.OrganizationUser{
		OrganizationID:         orgID,
		OrganizationExternalID: "org_user_schema_test",
		UserID:                 userID,
		ProjectID:              projectID,
		Data: map[string]any{
			"role":        "owner",
			"permissions": []string{"read", "write", "admin"},
			"metadata": map[string]any{
				"joined_via": "invitation",
			},
		},
	}

	err = pubsub.NewPublisher(jet).Publish(ctx, schemas.OrganizationUsersSchema(projectID), orgUser)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamOrganizationUsers, ConsumerOrganizationUsersSchema)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)
}

// strPtr is a helper function to create a string pointer.
func strPtr(s string) *string {
	return &s
}

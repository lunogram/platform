package v1

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/claim"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/rules"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupEventsController(t *testing.T) (*EventsController, uuid.UUID) {
	t.Helper()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, config.Store)
	require.NoError(t, err)

	orgsStore := store.NewOrganizationsStore(db)
	orgID, err := orgsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectsStore := store.NewProjectsStore(db)
	projectID, err := projectsStore.CreateProject(ctx, store.Project{
		OrganizationID: &orgID,
		Name:           DefaultProject.Name,
		Timezone:       DefaultProject.Timezone,
		Locale:         DefaultProject.Locale,
	})
	require.NoError(t, err)

	controller := NewEventsController(logger, db)
	return controller, projectID
}

func TestListEvents(t *testing.T) {
	t.Parallel()

	controller, projectID := setupEventsController(t)
	ctx := context.Background()

	eventsStore := controller.store.EventsStore

	eventID1, err := eventsStore.UpsertEvent(ctx, projectID, "purchase_completed")
	require.NoError(t, err)

	paths1 := rules.Paths{
		{Path: ".product_id", Type: rules.TypeString},
		{Path: ".amount", Type: rules.TypeNumber},
		{Path: ".currency", Type: rules.TypeString},
	}
	err = eventsStore.UpsertEventSchema(ctx, projectID, eventID1, paths1)
	require.NoError(t, err)

	eventID2, err := eventsStore.UpsertEvent(ctx, projectID, "page_viewed")
	require.NoError(t, err)

	paths2 := rules.Paths{
		{Path: ".page", Type: rules.TypeString},
		{Path: ".referrer", Type: rules.TypeString},
	}
	err = eventsStore.UpsertEventSchema(ctx, projectID, eventID2, paths2)
	require.NoError(t, err)

	_, err = eventsStore.UpsertEvent(ctx, projectID, "user_logout")
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/events", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	controller.ListEvents(res, req, projectID)

	require.Equal(t, 200, res.Code)

	var response struct {
		Results []oapi.EventWithSchema `json:"results"`
	}
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Results, 3)

	var purchaseEvent *oapi.EventWithSchema
	var pageEvent *oapi.EventWithSchema
	var logoutEvent *oapi.EventWithSchema
	for i := range response.Results {
		switch response.Results[i].Name {
		case "purchase_completed":
			purchaseEvent = &response.Results[i]
		case "page_viewed":
			pageEvent = &response.Results[i]
		case "user_logout":
			logoutEvent = &response.Results[i]
		}
	}

	require.NotNil(t, purchaseEvent)
	require.Equal(t, eventID1, purchaseEvent.Id)
	require.Len(t, purchaseEvent.Paths, 3)
	require.Contains(t, purchaseEvent.Paths, ".product_id")
	require.Contains(t, purchaseEvent.Paths, ".amount")
	require.Contains(t, purchaseEvent.Paths, ".currency")
	require.Len(t, purchaseEvent.Types, 3)

	require.NotNil(t, pageEvent)
	require.Equal(t, eventID2, pageEvent.Id)
	require.Len(t, pageEvent.Paths, 2)
	require.Contains(t, pageEvent.Paths, ".page")
	require.Contains(t, pageEvent.Paths, ".referrer")

	require.NotNil(t, logoutEvent)
	require.Empty(t, logoutEvent.Paths)
	require.Empty(t, logoutEvent.Types)
}

func TestListEventsEmpty(t *testing.T) {
	t.Parallel()

	controller, projectID := setupEventsController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/events", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	controller.ListEvents(res, req, projectID)

	require.Equal(t, 200, res.Code)

	var response struct {
		Results []oapi.EventWithSchema `json:"results"`
	}
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Empty(t, response.Results)
}

func TestListEventsUnauthorized(t *testing.T) {
	t.Parallel()

	controller, projectID := setupEventsController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/events", nil)

	controller.ListEvents(res, req, projectID)

	require.Equal(t, 401, res.Code)
}

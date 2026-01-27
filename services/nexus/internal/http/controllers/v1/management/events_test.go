package v1

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/claim"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/container"
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
	require.Len(t, purchaseEvent.Schema, 3)

	pathMap := make(map[string][]string)
	for _, s := range purchaseEvent.Schema {
		pathMap[s.Path] = s.Types
	}
	require.Contains(t, pathMap, ".product_id")
	require.Contains(t, pathMap, ".amount")
	require.Contains(t, pathMap, ".currency")
	require.Contains(t, pathMap[".product_id"], "string")
	require.Contains(t, pathMap[".amount"], "number")
	require.Contains(t, pathMap[".currency"], "string")

	require.NotNil(t, pageEvent)
	require.Equal(t, eventID2, pageEvent.Id)
	require.Len(t, pageEvent.Schema, 2)

	pagePathMap := make(map[string][]string)
	for _, s := range pageEvent.Schema {
		pagePathMap[s.Path] = s.Types
	}
	require.Contains(t, pagePathMap, ".page")
	require.Contains(t, pagePathMap, ".referrer")

	require.NotNil(t, logoutEvent)
	require.Empty(t, logoutEvent.Schema)
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

func TestListEventsWithMultipleTypes(t *testing.T) {
	t.Parallel()

	controller, projectID := setupEventsController(t)
	ctx := context.Background()

	eventsStore := controller.store.EventsStore

	eventID, err := eventsStore.UpsertEvent(ctx, projectID, "user_action")
	require.NoError(t, err)

	// Insert the same path with different types to simulate real-world scenarios
	// where a field might be sent as different types across different events
	paths := rules.Paths{
		{Path: ".user_id", Type: rules.TypeString},
		{Path: ".user_id", Type: rules.TypeNumber},
		{Path: ".metadata", Type: rules.TypeObject},
		{Path: ".metadata", Type: rules.TypeString},
		{Path: ".tags", Type: rules.TypeArray},
	}
	err = eventsStore.UpsertEventSchema(ctx, projectID, eventID, paths)
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
	require.Len(t, response.Results, 1)

	event := response.Results[0]
	require.Equal(t, "user_action", event.Name)
	require.Len(t, event.Schema, 3)

	pathMap := make(map[string][]string)
	for _, s := range event.Schema {
		pathMap[s.Path] = s.Types
	}

	// Verify .user_id has both string and number types
	require.Contains(t, pathMap, ".user_id")
	require.Len(t, pathMap[".user_id"], 2)
	require.Contains(t, pathMap[".user_id"], "number")
	require.Contains(t, pathMap[".user_id"], "string")

	// Verify .metadata has both object and string types
	require.Contains(t, pathMap, ".metadata")
	require.Len(t, pathMap[".metadata"], 2)
	require.Contains(t, pathMap[".metadata"], "object")
	require.Contains(t, pathMap[".metadata"], "string")

	// Verify .tags has only array type
	require.Contains(t, pathMap, ".tags")
	require.Len(t, pathMap[".tags"], 1)
	require.Contains(t, pathMap[".tags"], "array")
}

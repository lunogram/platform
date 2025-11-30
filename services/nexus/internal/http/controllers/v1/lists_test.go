package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestListCreation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	lists := NewListsController(logger, db)

	type test struct {
		body oapi.CreateListJSONRequestBody
		code int
	}

	tests := map[string]test{
		"static list": {
			body: oapi.CreateListJSONRequestBody{
				Name: "Static Test List",
				Type: oapi.CreateListTypeStatic,
			},
			code: 201,
		},
		"dynamic list": {
			body: oapi.CreateListJSONRequestBody{
				Name: "Dynamic Test List",
				Type: oapi.CreateListTypeDynamic,
			},
			code: 201,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/lists", bytes.NewReader(bb))
			lists.CreateList(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 201 {
				var response oapi.List
				err := json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, test.body.Name, response.Name)
				require.Equal(t, string(test.body.Type), string(response.Type))
			}
		})
	}
}

func TestListLists(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	listsStore := store.NewListsStore(db)

	usersCount := 0
	testLists := []store.List{
		{
			ProjectID:  projectID,
			Name:       "Test List 1",
			Type:       "static",
			State:      "ready",
			IsVisible:  true,
			UsersCount: &usersCount,
			Version:    0,
		},
		{
			ProjectID:  projectID,
			Name:       "Test List 2",
			Type:       "static",
			State:      "ready",
			IsVisible:  true,
			UsersCount: &usersCount,
			Version:    0,
		},
		{
			ProjectID:  projectID,
			Name:       "Test List 3",
			Type:       "static",
			State:      "ready",
			IsVisible:  true,
			UsersCount: &usersCount,
			Version:    0,
		},
	}

	for _, list := range testLists {
		_, err := listsStore.CreateList(ctx, list)
		require.NoError(t, err)
	}

	controller := NewListsController(logger, db)

	type test struct {
		limit  int
		offset int
		total  int
		result int
	}

	tests := map[string]test{
		"default": {
			limit:  10,
			offset: 0,
			total:  3,
			result: 3,
		},
		"with limit": {
			limit:  2,
			offset: 0,
			total:  3,
			result: 2,
		},
		"with offset": {
			limit:  10,
			offset: 1,
			total:  3,
			result: 2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			limit := oapi.Limit(test.limit)
			offset := oapi.Offset(test.offset)

			params := oapi.ListListsParams{
				Limit:  &limit,
				Offset: &offset,
			}

			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/lists", nil)
			controller.ListLists(res, req, projectID, params)

			require.Equal(t, 200, res.Code, res.Body.String())

			var response oapi.ListListResponse
			err := json.Unmarshal(res.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, test.total, response.Total)
			require.Equal(t, test.result, len(response.Results))
		})
	}
}

func TestGetList(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	listsStore := store.NewListsStore(db)
	usersCount := 0
	listID, err := listsStore.CreateList(ctx, store.List{
		ProjectID:  projectID,
		Name:       "Test List",
		Type:       "static",
		State:      "ready",
		IsVisible:  true,
		UsersCount: &usersCount,
		Version:    0,
	})
	require.NoError(t, err)

	controller := NewListsController(logger, db)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/lists/"+listID.String(), nil)
	controller.GetList(res, req, projectID, listID)

	require.Equal(t, 200, res.Code, res.Body.String())

	var response oapi.List
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, listID, response.Id)
	require.Equal(t, "Test List", response.Name)
}

func TestUpdateList(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	listsStore := store.NewListsStore(db)
	usersCount := 0
	listID, err := listsStore.CreateList(ctx, store.List{
		ProjectID:  projectID,
		Name:       "Test List",
		Type:       "dynamic",
		State:      "draft",
		IsVisible:  true,
		UsersCount: &usersCount,
		Version:    0,
	})
	require.NoError(t, err)

	controller := NewListsController(logger, db)

	body := oapi.UpdateListJSONRequestBody{
		Name: "Updated List",
	}

	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/v1/lists/"+listID.String(), bytes.NewReader(bb))
	controller.UpdateList(res, req, projectID, listID)

	require.Equal(t, 200, res.Code, res.Body.String())

	var response oapi.List
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "Updated List", response.Name)
}

func TestDeleteList(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	listsStore := store.NewListsStore(db)
	usersCount := 0
	listID, err := listsStore.CreateList(ctx, store.List{
		ProjectID:  projectID,
		Name:       "Test List",
		Type:       "static",
		State:      "ready",
		IsVisible:  true,
		UsersCount: &usersCount,
		Version:    0,
	})
	require.NoError(t, err)

	controller := NewListsController(logger, db)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/lists/"+listID.String(), nil)
	controller.DeleteList(res, req, projectID, listID)

	require.Equal(t, 204, res.Code, res.Body.String())

	_, err = listsStore.GetList(ctx, projectID, listID)
	require.Error(t, err)
}

func TestDuplicateList(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	listsStore := store.NewListsStore(db)
	usersCount := 0
	listID, err := listsStore.CreateList(ctx, store.List{
		ProjectID:  projectID,
		Name:       "Original List",
		Type:       "static",
		State:      "ready",
		IsVisible:  true,
		UsersCount: &usersCount,
		Version:    1,
	})
	require.NoError(t, err)

	controller := NewListsController(logger, db)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/lists/"+listID.String()+"/duplicate", nil)
	controller.DuplicateList(res, req, projectID, listID)

	require.Equal(t, 201, res.Code, res.Body.String())

	var response oapi.List
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "Copy of Original List", response.Name)
	require.NotEqual(t, listID, response.Id)
	require.Equal(t, 0, response.Version)
}

package v1

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const testMaxUploadSize = 10485760 // 10MB

//go:embed test/users/valid.csv
var validUsersCSV string

//go:embed test/users/no-external-id.csv
var noExternalIDCSV string

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

	lists := NewListsController(logger, db, testMaxUploadSize)

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
			Type:       store.ListTypeStatic,
			State:      store.ListStateReady,
			UsersCount: &usersCount,
			Version:    0,
		},
		{
			ProjectID:  projectID,
			Name:       "Test List 2",
			Type:       store.ListTypeStatic,
			State:      store.ListStateReady,
			UsersCount: &usersCount,
			Version:    0,
		},
		{
			ProjectID:  projectID,
			Name:       "Test List 3",
			Type:       store.ListTypeStatic,
			State:      store.ListStateReady,
			UsersCount: &usersCount,
			Version:    0,
		},
	}

	for _, list := range testLists {
		_, err := listsStore.CreateList(ctx, list)
		require.NoError(t, err)
	}

	controller := NewListsController(logger, db, testMaxUploadSize)

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
		UsersCount: &usersCount,
		Version:    0,
	})
	require.NoError(t, err)

	controller := NewListsController(logger, db, testMaxUploadSize)

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
		Type:       store.ListTypeDynamic,
		State:      store.ListStateDraft,
		UsersCount: &usersCount,
		Version:    0,
	})
	require.NoError(t, err)

	controller := NewListsController(logger, db, testMaxUploadSize)

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
		Type:       store.ListTypeStatic,
		State:      store.ListStateReady,
		UsersCount: &usersCount,
		Version:    0,
	})
	require.NoError(t, err)

	controller := NewListsController(logger, db, testMaxUploadSize)

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
		Type:       store.ListTypeStatic,
		State:      store.ListStateReady,
		UsersCount: &usersCount,
		Version:    1,
	})
	require.NoError(t, err)

	controller := NewListsController(logger, db, testMaxUploadSize)

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

func TestImportListUsers(t *testing.T) {
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

	usersStore := store.NewUsersStore(db)
	listsStore := store.NewListsStore(db)

	list := store.List{
		ProjectID: projectID,
		Name:      "Import Test List",
		Type:      store.ListTypeStatic,
		State:     store.ListStateReady,
	}

	listID, err := listsStore.CreateList(ctx, list)
	require.NoError(t, err)

	controller := NewListsController(logger, db, testMaxUploadSize)

	type test struct {
		csv   string
		code  int
		users int
	}

	tests := map[string]test{
		"successful-import": {
			csv:   validUsersCSV,
			code:  204,
			users: 2,
		},
		"missing-external-id-column": {
			csv:   noExternalIDCSV,
			code:  400,
			users: 0,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			header := textproto.MIMEHeader{}
			header.Set("Content-Disposition", `form-data; name="file"; filename="users.csv"`)
			header.Set("Content-Type", "text/csv")

			part, err := writer.CreatePart(header)
			require.NoError(t, err)

			_, err = part.Write([]byte(test.csv))
			require.NoError(t, err)

			err = writer.Close()
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", fmt.Sprintf("/v1/lists/%s/users", listID), body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			controller.ImportListUsers(res, req, projectID, listID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			users, total, err := usersStore.ListUsers(ctx, projectID, store.Pagination{Limit: 100, Offset: 0}, "")
			require.NoError(t, err)
			require.Equal(t, test.users, total, "expected %d users in total", test.users)
			require.Len(t, users, test.users, "expected %d users to be returned", test.users)
		})
	}
}

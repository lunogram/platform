package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/container"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestTagCreation(t *testing.T) {
	t.Parallel()

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

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	tags := NewTagsController(logger, db)

	type test struct {
		body oapi.CreateTagJSONRequestBody
		code int
	}

	tests := map[string]test{
		"simple": {
			body: oapi.CreateTagJSONRequestBody{
				Name: "important",
			},
			code: 201,
		},
		"with-spaces": {
			body: oapi.CreateTagJSONRequestBody{
				Name: "very important",
			},
			code: 201,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/tags", bytes.NewReader(bb))
			tags.CreateTag(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 201 {
				var result oapi.Tag
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, test.body.Name, result.Name)
				require.Equal(t, projectID, result.ProjectId)
				require.NotEqual(t, uuid.Nil, result.Id)
			}
		})
	}
}

func TestListTags(t *testing.T) {
	t.Parallel()

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

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	tagsStore := store.NewTagsStore(db)

	// Create some test tags
	tagNames := []string{"urgent", "important", "follow-up", "archived"}
	for _, name := range tagNames {
		_, err := tagsStore.CreateTag(ctx, projectID, name)
		require.NoError(t, err)
	}

	tags := NewTagsController(logger, db)

	type test struct {
		params   oapi.ListTagsParams
		expected int
	}

	tests := map[string]test{
		"list-all": {
			params: oapi.ListTagsParams{
				Limit:  ptr(oapi.PaginationLimit(10)),
				Offset: ptr(oapi.PaginationOffset(0)),
			},
			expected: 4,
		},
		"with-pagination": {
			params: oapi.ListTagsParams{
				Limit:  ptr(oapi.PaginationLimit(2)),
				Offset: ptr(oapi.PaginationOffset(0)),
			},
			expected: 2,
		},
		"with-search": {
			params: oapi.ListTagsParams{
				Limit:  ptr(oapi.PaginationLimit(10)),
				Offset: ptr(oapi.PaginationOffset(0)),
				Search: ptr(oapi.PaginationSearch("imp")),
			},
			expected: 1,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/tags", nil)
			tags.ListTags(res, req, projectID, test.params)

			require.Equal(t, 200, res.Code, res.Body.String())

			var result oapi.TagListResponse
			err := json.Unmarshal(res.Body.Bytes(), &result)
			require.NoError(t, err)
			require.Equal(t, test.expected, len(result.Results))
			require.GreaterOrEqual(t, result.Total, test.expected)
		})
	}
}

func TestGetTag(t *testing.T) {
	t.Parallel()

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

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	tagsStore := store.NewTagsStore(db)
	tagID, err := tagsStore.CreateTag(ctx, projectID, "test-tag")
	require.NoError(t, err)

	tags := NewTagsController(logger, db)

	type test struct {
		tagID uuid.UUID
		code  int
	}

	tests := map[string]test{
		"existing-tag": {
			tagID: tagID,
			code:  200,
		},
		"non-existing-tag": {
			tagID: uuid.New(),
			code:  404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", fmt.Sprintf("/v1/tags/%s", test.tagID), nil)
			tags.GetTag(res, req, projectID, test.tagID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result oapi.Tag
				err := json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, test.tagID, result.Id)
				require.Equal(t, "test-tag", result.Name)
			}
		})
	}
}

func TestUpdateTag(t *testing.T) {
	t.Parallel()

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

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	tagsStore := store.NewTagsStore(db)
	tagID, err := tagsStore.CreateTag(ctx, projectID, "old-name")
	require.NoError(t, err)

	tags := NewTagsController(logger, db)

	type test struct {
		tagID uuid.UUID
		body  oapi.UpdateTagJSONRequestBody
		code  int
	}

	tests := map[string]test{
		"successful-update": {
			tagID: tagID,
			body: oapi.UpdateTagJSONRequestBody{
				Name: "new-name",
			},
			code: 200,
		},
		"non-existing-tag": {
			tagID: uuid.New(),
			body: oapi.UpdateTagJSONRequestBody{
				Name: "another-name",
			},
			code: 404, // GetTag will return 404 for non-existent tag
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", fmt.Sprintf("/v1/tags/%s", test.tagID), bytes.NewReader(bb))
			tags.UpdateTag(res, req, projectID, test.tagID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result oapi.Tag
				err := json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, test.body.Name, result.Name)
			}
		})
	}
}

func TestDeleteTag(t *testing.T) {
	t.Parallel()

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

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	tagsStore := store.NewTagsStore(db)
	tagID, err := tagsStore.CreateTag(ctx, projectID, "to-delete")
	require.NoError(t, err)

	tags := NewTagsController(logger, db)

	type test struct {
		tagID uuid.UUID
		code  int
	}

	tests := map[string]test{
		"successful-delete": {
			tagID: tagID,
			code:  204,
		},
		"non-existing-tag": {
			tagID: uuid.New(),
			code:  204, // Succeeds even if tag doesn't exist (idempotent)
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", fmt.Sprintf("/v1/tags/%s", test.tagID), nil)
			tags.DeleteTag(res, req, projectID, test.tagID)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

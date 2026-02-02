package v1

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCreateLocale(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	locales := NewLocalesController(logger, db)

	type test struct {
		body oapi.CreateLocaleJSONRequestBody
		code int
	}

	tests := map[string]test{
		"success": {
			body: oapi.CreateLocaleJSONRequestBody{
				Key:   "en",
				Label: "English",
			},
			code: 201,
		},
		"spanish": {
			body: oapi.CreateLocaleJSONRequestBody{
				Key:   "es",
				Label: "Spanish",
			},
			code: 201,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/locales", bytes.NewReader(bb))
			locales.CreateLocale(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 201 {
				var response map[string]oapi.Locale
				err = json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, test.body.Key, response["data"].Key)
				require.Equal(t, test.body.Label, response["data"].Label)
				require.Equal(t, projectID, response["data"].ProjectId)
			}
		})
	}
}

func TestListLocales(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	localesStore := store.NewLocalesStore(db)
	locales := []struct {
		key   string
		label string
	}{
		{"en", "English"},
		{"es", "Spanish"},
		{"fr", "French"},
	}

	for _, locale := range locales {
		_, err = localesStore.CreateLocale(ctx, store.Locale{
			ProjectID: projectID,
			Key:       locale.key,
			Label:     locale.label,
		})
		require.NoError(t, err)
	}

	controller := NewLocalesController(logger, db)

	type test struct {
		params oapi.ListLocalesParams
		count  int
	}

	limit20 := oapi.PaginationLimit(20)
	limit2 := oapi.PaginationLimit(2)
	offset1 := oapi.PaginationOffset(1)

	tests := map[string]test{
		"all locales": {
			params: oapi.ListLocalesParams{},
			count:  3,
		},
		"with limit": {
			params: oapi.ListLocalesParams{
				Limit: &limit2,
			},
			count: 2,
		},
		"with offset": {
			params: oapi.ListLocalesParams{
				Limit:  &limit20,
				Offset: &offset1,
			},
			count: 2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/locales", nil)
			controller.ListLocales(res, req, projectID, test.params)

			require.Equal(t, 200, res.Code, res.Body.String())

			var response map[string]any
			err = json.Unmarshal(res.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, float64(3), response["total"])

			results := response["results"].([]any)
			require.Len(t, results, test.count)
		})
	}
}

func TestGetLocale(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	localesStore := store.NewLocalesStore(db)
	localeID, err := localesStore.CreateLocale(ctx, store.Locale{
		ProjectID: projectID,
		Key:       "en",
		Label:     "English",
	})
	require.NoError(t, err)

	controller := NewLocalesController(logger, db)

	type test struct {
		localeID string
		code     int
	}

	tests := map[string]test{
		"get by id": {
			localeID: localeID.String(),
			code:     200,
		},
		"get by key": {
			localeID: "en",
			code:     200,
		},
		"not found": {
			localeID: uuid.New().String(),
			code:     404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/locales/"+test.localeID, nil)
			controller.GetLocale(res, req, projectID, test.localeID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var response oapi.Locale
				err = json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, "en", response.Key)
				require.Equal(t, "English", response.Label)
			}
		})
	}
}

func TestDeleteLocale(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	localesStore := store.NewLocalesStore(db)
	localeID, err := localesStore.CreateLocale(ctx, store.Locale{
		ProjectID: projectID,
		Key:       "en",
		Label:     "English",
	})
	require.NoError(t, err)

	controller := NewLocalesController(logger, db)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/locales/"+localeID.String(), nil)
	controller.DeleteLocale(res, req, projectID, localeID)

	require.Equal(t, 204, res.Code, res.Body.String())

	// Verify locale was deleted
	_, err = localesStore.GetLocale(ctx, projectID, localeID.String())
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestLocaleProjectNotFound(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, cfg.Store)
	require.NoError(t, err)

	controller := NewLocalesController(logger, db)
	invalidProjectID := uuid.New()

	t.Run("create locale - project not found", func(t *testing.T) {
		body := oapi.CreateLocaleJSONRequestBody{
			Key:   "en",
			Label: "English",
		}
		bb, err := json.Marshal(body)
		require.NoError(t, err)

		res := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/v1/locales", bytes.NewReader(bb))
		controller.CreateLocale(res, req, invalidProjectID)

		require.Equal(t, 404, res.Code, res.Body.String())
	})

	t.Run("list locales - project not found", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/v1/locales", nil)
		controller.ListLocales(res, req, invalidProjectID, oapi.ListLocalesParams{})

		require.Equal(t, 404, res.Code, res.Body.String())
	})

	t.Run("get locale - project not found", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/v1/locales/en", nil)
		controller.GetLocale(res, req, invalidProjectID, "en")

		require.Equal(t, 404, res.Code, res.Body.String())
	})

	t.Run("delete locale - project not found", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", "/v1/locales/"+uuid.New().String(), nil)
		controller.DeleteLocale(res, req, invalidProjectID, uuid.New())

		require.Equal(t, 404, res.Code, res.Body.String())
	})
}

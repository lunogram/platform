package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCampaignCreation(t *testing.T) {
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

	campaigns := NewCampaignsController(logger, db)

	tests := map[string]oapi.CreateCampaignJSONRequestBody{
		"simple": {
			Channel: oapi.CreateCampaignChannelEmail,
			Name:    "Welcome to the program!",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/campaigns", bytes.NewReader(bb))
			campaigns.CreateCampaign(res, req, projectID)

			require.Equal(t, 201, res.Code, res.Body.String())
		})
	}
}

func TestListCampaigns(t *testing.T) {
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

	campaigns := store.NewCampaignsStore(db)
	templates := store.NewTemplatesStore(db)

	// NOTE: create some test campaigns
	for i := 0; i < 3; i++ {
		campaignID, err := campaigns.CreateCampaign(ctx, store.Campaign{
			ProjectID: projectID,
			Name:      "Test Campaign",
			Channel:   "email",
		})
		require.NoError(t, err)

		_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
		require.NoError(t, err)
	}

	controller := NewCampaignsController(logger, db)

	type test struct {
		Limit  int
		Offset int
		Total  int
		Result int
	}

	tests := map[string]test{
		"default": {
			Limit:  10,
			Offset: 0,
			Total:  3,
			Result: 3,
		},
		"with limit": {
			Limit:  2,
			Offset: 0,
			Total:  3,
			Result: 2,
		},
		"with offset": {
			Limit:  10,
			Offset: 1,
			Total:  3,
			Result: 2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			limit := oapi.Limit(test.Limit)
			offset := oapi.Offset(test.Offset)

			params := oapi.ListCampaignsParams{
				Limit:  &limit,
				Offset: &offset,
			}

			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/campaigns", nil)
			controller.ListCampaigns(res, req, projectID, params)

			require.Equal(t, 200, res.Code, res.Body.String())

			var response oapi.CampaignListResponse
			err := json.Unmarshal(res.Body.Bytes(), &response)
			require.NoError(t, err)

			require.Equal(t, test.Total, response.Total)
			require.Len(t, response.Results, test.Result)
		})
	}
}

func TestGetCampaign(t *testing.T) {
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

	campaigns := store.NewCampaignsStore(db)
	templates := store.NewTemplatesStore(db)

	campaignID, err := campaigns.CreateCampaign(ctx, store.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
	require.NoError(t, err)

	controller := NewCampaignsController(logger, db)

	type test struct {
		id   uuid.UUID
		code int
	}

	tests := map[string]test{
		"found": {
			id:   campaignID,
			code: 200,
		},
		"not found": {
			id:   uuid.Nil,
			code: 404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/campaigns/"+test.id.String(), nil)
			controller.GetCampaign(res, req, projectID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestUpdateCampaign(t *testing.T) {
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

	campaigns := store.NewCampaignsStore(db)
	templates := store.NewTemplatesStore(db)

	campaignID, err := campaigns.CreateCampaign(ctx, store.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
	require.NoError(t, err)

	controller := NewCampaignsController(logger, db)

	tests := map[string]struct {
		id   uuid.UUID
		body oapi.UpdateCampaignJSONRequestBody
		code int
	}{
		"success": {
			id:   campaignID,
			body: oapi.UpdateCampaignJSONRequestBody{Name: ptr("Updated Name")},
			code: 200,
		},
		"not found": {
			id:   uuid.Nil,
			body: oapi.UpdateCampaignJSONRequestBody{Name: ptr("Updated Name")},
			code: 404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/v1/campaigns/"+test.id.String(), bytes.NewReader(bb))
			controller.UpdateCampaign(res, req, projectID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

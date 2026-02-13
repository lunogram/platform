package v1

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCampaignCreation(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	projects := management.NewProjectsStore(db.Management)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := NewCampaignsController(logger, db.Management)

	type test struct {
		body oapi.CreateCampaignJSONRequestBody
	}

	tests := map[string]test{
		"simple": {
			body: oapi.CreateCampaignJSONRequestBody{
				Channel: oapi.Email,
				Name:    "Welcome to the program!",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/campaigns", bytes.NewReader(bb))
			campaigns.CreateCampaign(res, req, projectID)

			require.Equal(t, 201, res.Code, res.Body.String())
		})
	}
}

func TestListCampaigns(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	projects := management.NewProjectsStore(db.Management)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := management.NewCampaignsStore(db.Management)
	templates := management.NewTemplatesStore(db.Management)

	// NOTE: create some test campaigns
	for i := 0; i < 3; i++ {
		campaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
			ProjectID: projectID,
			Name:      "Test Campaign",
			Channel:   "email",
		})
		require.NoError(t, err)

		_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
		require.NoError(t, err)
	}

	controller := NewCampaignsController(logger, db.Management)

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

			require.Equal(t, test.total, response.Total)
			require.Len(t, response.Results, test.result)
		})
	}
}

func TestGetCampaign(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	projects := management.NewProjectsStore(db.Management)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := management.NewCampaignsStore(db.Management)
	templates := management.NewTemplatesStore(db.Management)

	campaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
	require.NoError(t, err)

	controller := NewCampaignsController(logger, db.Management)

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
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	projects := management.NewProjectsStore(db.Management)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := management.NewCampaignsStore(db.Management)
	templates := management.NewTemplatesStore(db.Management)

	campaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
	require.NoError(t, err)

	controller := NewCampaignsController(logger, db.Management)

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

func TestDeleteCampaign(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	projects := management.NewProjectsStore(db.Management)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := management.NewCampaignsStore(db.Management)
	templates := management.NewTemplatesStore(db.Management)

	campaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
	require.NoError(t, err)

	controller := NewCampaignsController(logger, db.Management)

	tests := map[string]struct {
		id   uuid.UUID
		code int
	}{
		"success": {
			id:   campaignID,
			code: 204,
		},
		"not found": {
			id:   uuid.Nil,
			code: 404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", "/v1/campaigns/"+test.id.String(), nil)
			controller.DeleteCampaign(res, req, projectID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 204 {
				// Verify campaign is soft deleted
				campaign, err := campaigns.GetCampaign(ctx, projectID, test.id)
				require.ErrorIs(t, err, sql.ErrNoRows)
				require.Nil(t, campaign)
			}
		})
	}
}

func TestDuplicateCampaign(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	projects := management.NewProjectsStore(db.Management)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := management.NewCampaignsStore(db.Management)
	templates := management.NewTemplatesStore(db.Management)

	campaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Original Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
	require.NoError(t, err)

	controller := NewCampaignsController(logger, db.Management)

	tests := map[string]struct {
		id   uuid.UUID
		code int
	}{
		"success": {
			id:   campaignID,
			code: 201,
		},
		"not found": {
			id:   uuid.Nil,
			code: 404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/campaigns/"+test.id.String()+"/duplicate", nil)
			controller.DuplicateCampaign(res, req, projectID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 201 {
				var response oapi.Campaign
				err := json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)

				require.NotEqual(t, test.id, response.Id)
				require.Equal(t, "Copy of Original Campaign", response.Name)
				require.Equal(t, projectID, response.ProjectId)
				require.Len(t, response.Templates, 1)
			}
		})
	}
}

func TestGetCampaignUsers(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	err = users.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	projects := management.NewProjectsStore(db.Management)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := management.NewCampaignsStore(db.Management)
	templates := management.NewTemplatesStore(db.Management)

	campaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
	require.NoError(t, err)

	controller := NewCampaignsController(logger, db.Management)

	tests := map[string]struct {
		id   uuid.UUID
		code int
	}{
		"success": {
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
			limit := oapi.Limit(10)
			offset := oapi.Offset(0)

			params := oapi.GetCampaignUsersParams{
				Limit:  &limit,
				Offset: &offset,
			}

			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/campaigns/"+test.id.String()+"/users", nil)
			controller.GetCampaignUsers(res, req, projectID, test.id, params)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var response map[string]any
				err := json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)

				require.Contains(t, response, "data")
				require.Contains(t, response, "total")
				require.Contains(t, response, "limit")
				require.Contains(t, response, "offset")
			}
		})
	}
}

package v1

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/rbac"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCampaignCreation(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	campaigns := NewCampaignsController(logger, mgmt, usrs, engine)

	type test struct {
		body oapi.CreateCampaignJSONRequestBody
	}

	tests := map[string]test{
		"simple": {
			body: oapi.CreateCampaignJSONRequestBody{
				Channel: oapi.ChannelEmail,
				Name:    "Welcome to the program!",
			},
		},
		"subscription channel mismatch": {
			body: func() oapi.CreateCampaignJSONRequestBody {
				subscriptions := management.NewSubscriptionsStore(mgmt)
				subscriptionID, err := subscriptions.CreateSubscription(ctx, management.Subscription{
					ProjectID: projectID,
					Name:      "Push Only",
					Channel:   "push",
					IsPublic:  true,
				})
				require.NoError(t, err)

				return oapi.CreateCampaignJSONRequestBody{
					Channel:        oapi.ChannelEmail,
					Name:           "Welcome to the program!",
					SubscriptionId: &subscriptionID,
				}
			}(),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/campaigns", bytes.NewReader(bb))
			req = req.WithContext(actorCtx)
			campaigns.CreateCampaign(res, req, projectID)

			expected := 201
			if name == "subscription channel mismatch" {
				expected = 400
			}
			require.Equal(t, expected, res.Code, res.Body.String())
		})
	}
}

func TestListCampaigns(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaignsStore := management.NewCampaignsStore(mgmt)
	templates := management.NewTemplatesStore(mgmt)

	// NOTE: create some test campaigns
	campaignNames := []string{"Welcome Campaign", "Newsletter Campaign", "Promo Blast"}
	for _, name := range campaignNames {
		campaignID, err := campaignsStore.CreateCampaign(ctx, management.Campaign{
			ProjectID: projectID,
			Name:      name,
			Channel:   "email",
		})
		require.NoError(t, err)

		_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en", nil)
		require.NoError(t, err)
	}

	_, err = mgmt.ExecContext(ctx, `
		INSERT INTO campaigns (project_id, name, channel)
		VALUES ($1, NULL, 'email')`, projectID)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewCampaignsController(logger, mgmt, usrs, engine)

	type test struct {
		limit  int
		offset int
		search string
		total  int
		result int
	}

	tests := map[string]test{
		"default": {
			limit:  10,
			offset: 0,
			total:  4,
			result: 4,
		},
		"with limit": {
			limit:  2,
			offset: 0,
			total:  4,
			result: 2,
		},
		"with offset": {
			limit:  10,
			offset: 1,
			total:  4,
			result: 3,
		},
		"with search": {
			limit:  10,
			offset: 0,
			search: "newsletter",
			total:  1,
			result: 1,
		},
		"with search no results": {
			limit:  10,
			offset: 0,
			search: "nonexistent",
			total:  0,
			result: 0,
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
			if test.search != "" {
				search := oapi.PaginationSearch(test.search)
				params.Search = &search
			}

			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/campaigns", nil)
			req = req.WithContext(actorCtx)
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
	ctx := t.Context()
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaignsStore := management.NewCampaignsStore(mgmt)
	subscriptionsStore := management.NewSubscriptionsStore(mgmt)
	templates := management.NewTemplatesStore(mgmt)

	subscriptionID, err := subscriptionsStore.CreateSubscription(ctx, management.Subscription{
		ProjectID: projectID,
		Name:      "Marketing",
		Channel:   "email",
		IsPublic:  true,
	})
	require.NoError(t, err)

	campaignID, err := campaignsStore.CreateCampaign(ctx, management.Campaign{
		ProjectID:      projectID,
		Name:           "Test Campaign",
		Channel:        "email",
		SubscriptionID: &subscriptionID,
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en", nil)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewCampaignsController(logger, mgmt, usrs, engine)

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
			req = req.WithContext(actorCtx)
			controller.GetCampaign(res, req, projectID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestUpdateCampaign(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaignsStore := management.NewCampaignsStore(mgmt)
	subscriptionsStore := management.NewSubscriptionsStore(mgmt)
	templates := management.NewTemplatesStore(mgmt)

	subscriptionID, err := subscriptionsStore.CreateSubscription(ctx, management.Subscription{
		ProjectID: projectID,
		Name:      "Marketing",
		Channel:   "email",
		IsPublic:  true,
	})
	require.NoError(t, err)

	campaignID, err := campaignsStore.CreateCampaign(ctx, management.Campaign{
		ProjectID:      projectID,
		Name:           "Test Campaign",
		Channel:        "email",
		SubscriptionID: &subscriptionID,
	})
	require.NoError(t, err)

	otherProjectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	otherProjectSubscriptionID, err := subscriptionsStore.CreateSubscription(ctx, management.Subscription{
		ProjectID: otherProjectID,
		Name:      "Other Project",
		Channel:   "email",
		IsPublic:  true,
	})
	require.NoError(t, err)

	pushSubscriptionID, err := subscriptionsStore.CreateSubscription(ctx, management.Subscription{
		ProjectID: projectID,
		Name:      "Push Alerts",
		Channel:   "push",
		IsPublic:  true,
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en", nil)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewCampaignsController(logger, mgmt, usrs, engine)

	tests := map[string]struct {
		id     uuid.UUID
		body   oapi.UpdateCampaignJSONRequestBody
		code   int
		assert func(t *testing.T)
	}{
		"success": {
			id:   campaignID,
			body: oapi.UpdateCampaignJSONRequestBody{Name: ptr.To("Updated Name")},
			code: 200,
		},
		"transactional clears subscription": {
			id:   campaignID,
			body: oapi.UpdateCampaignJSONRequestBody{Transactional: ptr.To(true)},
			code: 200,
			assert: func(t *testing.T) {
				t.Helper()

				updated, err := campaignsStore.GetCampaign(ctx, projectID, campaignID)
				require.NoError(t, err)
				require.True(t, updated.Transactional)
				require.Nil(t, updated.SubscriptionID)
			},
		},
		"rejects cross project subscription": {
			id: campaignID,
			body: oapi.UpdateCampaignJSONRequestBody{
				SubscriptionId: &otherProjectSubscriptionID,
			},
			code: 400,
		},
		"rejects mismatched subscription channel": {
			id: campaignID,
			body: oapi.UpdateCampaignJSONRequestBody{
				SubscriptionId: &pushSubscriptionID,
			},
			code: 400,
		},
		"not found": {
			id:   uuid.Nil,
			body: oapi.UpdateCampaignJSONRequestBody{Name: ptr.To("Updated Name")},
			code: 404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/v1/campaigns/"+test.id.String(), bytes.NewReader(bb))
			req = req.WithContext(actorCtx)
			controller.UpdateCampaign(res, req, projectID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.assert != nil {
				test.assert(t)
			}
		})
	}
}

func TestDeleteCampaign(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaignsStore := management.NewCampaignsStore(mgmt)
	templates := management.NewTemplatesStore(mgmt)

	campaignID, err := campaignsStore.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en", nil)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewCampaignsController(logger, mgmt, usrs, engine)

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
			req = req.WithContext(actorCtx)
			controller.DeleteCampaign(res, req, projectID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 204 {
				// Verify campaign is soft deleted
				campaign, err := campaignsStore.GetCampaign(ctx, projectID, test.id)
				require.ErrorIs(t, err, sql.ErrNoRows)
				require.Nil(t, campaign)
			}
		})
	}
}

func TestDuplicateCampaign(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaignsStore := management.NewCampaignsStore(mgmt)
	templates := management.NewTemplatesStore(mgmt)

	campaignID, err := campaignsStore.CreateCampaign(ctx, management.Campaign{
		ProjectID:     projectID,
		Name:          "Original Campaign",
		Channel:       "email",
		Transactional: true,
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en", nil)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewCampaignsController(logger, mgmt, usrs, engine)

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
			req = req.WithContext(actorCtx)
			controller.DuplicateCampaign(res, req, projectID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 201 {
				var response oapi.Campaign
				err := json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)

				require.NotEqual(t, test.id, response.Id)
				require.Equal(t, "Copy of Original Campaign", response.Name)
				require.Equal(t, projectID, response.ProjectId)
				require.True(t, response.Transactional)
				require.Len(t, response.Templates, 1)
			}
		})
	}
}

func TestGetCampaignUsers(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaignsStore := management.NewCampaignsStore(mgmt)
	templates := management.NewTemplatesStore(mgmt)

	campaignID, err := campaignsStore.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	_, err = templates.CreateTemplate(ctx, projectID, campaignID, "email", "en", nil)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewCampaignsController(logger, mgmt, usrs, engine)

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
			req = req.WithContext(actorCtx)
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

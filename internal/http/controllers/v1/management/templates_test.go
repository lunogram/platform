package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/rbac"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGetTemplate(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := management.NewCampaignsStore(mgmt)
	templates := management.NewTemplatesStore(mgmt)

	campaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	templateID, err := templates.CreateTemplate(ctx, projectID, campaignID, "email", "en", nil)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewTemplatesController(logger, mgmt, mgmt, pubsub.NewEmailRenderer(pubsub.NewNoopCaller()), nil, engine, nil, "")

	type test struct {
		id   uuid.UUID
		code int
	}

	tests := map[string]test{
		"found": {
			id:   templateID,
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
			req := httptest.NewRequest("GET", "/v1/campaigns/"+campaignID.String()+"/templates/"+test.id.String(), nil)
			req = req.WithContext(actorCtx)
			controller.GetTemplate(res, req, projectID, campaignID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestCreateTemplate(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := management.NewCampaignsStore(mgmt)
	campaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewTemplatesController(logger, mgmt, mgmt, pubsub.NewEmailRenderer(pubsub.NewNoopCaller()), nil, engine, nil, "")

	type test struct {
		body any
		code int
	}

	tests := map[string]test{
		"email template": {
			body: oapi.CreateTemplate{
				Locale: "en",
			},
			code: 201,
		},
		"text template": {
			body: oapi.CreateTemplate{
				Locale: "en",
			},
			code: 201,
		},
		"push template": {
			body: oapi.CreateTemplate{
				Locale: "en",
			},
			code: 201,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/campaigns/"+campaignID.String()+"/templates", bytes.NewReader(bb))
			req = req.WithContext(actorCtx)
			controller.CreateTemplate(res, req, projectID, campaignID)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestUpdateTemplate(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := management.NewCampaignsStore(mgmt)
	templates := management.NewTemplatesStore(mgmt)

	campaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	templateID, err := templates.CreateTemplate(ctx, projectID, campaignID, "email", "en", nil)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewTemplatesController(logger, mgmt, mgmt, pubsub.NewEmailRenderer(pubsub.NewNoopCaller()), nil, engine, nil, "")

	type test struct {
		id   uuid.UUID
		body any
		code int
	}

	tests := map[string]test{
		"success": {
			id: templateID,
			body: oapi.UpdateTemplate{
				Data: ptr.To(json.RawMessage(`{"subject":"Updated Subject"}`)),
			},
			code: 200,
		},
		"not found": {
			id: uuid.Nil,
			body: oapi.UpdateTemplate{
				Data: ptr.To(json.RawMessage(`{}`)),
			},
			code: 404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/v1/campaigns/"+campaignID.String()+"/templates/"+test.id.String(), bytes.NewReader(bb))
			req = req.WithContext(actorCtx)
			controller.UpdateTemplate(res, req, projectID, campaignID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestDeleteTemplate(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := management.NewCampaignsStore(mgmt)
	templates := management.NewTemplatesStore(mgmt)

	campaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	templateID, err := templates.CreateTemplate(ctx, projectID, campaignID, "email", "en", nil)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewTemplatesController(logger, mgmt, mgmt, pubsub.NewEmailRenderer(pubsub.NewNoopCaller()), nil, engine, nil, "")

	type test struct {
		id   uuid.UUID
		code int
	}

	tests := map[string]test{
		"success": {
			id:   templateID,
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
			req := httptest.NewRequest("DELETE", "/v1/campaigns/"+campaignID.String()+"/templates/"+test.id.String(), nil)
			req = req.WithContext(actorCtx)
			controller.DeleteTemplate(res, req, projectID, campaignID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

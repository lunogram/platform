package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
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

// compileCaller answers the renderer's compile requests with a bundle made from
// the source, or fails them with err when it is set.
type compileCaller struct {
	err error
}

func (c compileCaller) Call(_ context.Context, _ schemas.Subject, v any) ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}
	request := v.(schemas.CompileEmail)
	return json.Marshal(schemas.CompileEmailResponse{CompiledJS: "compiled:" + request.Source})
}

func TestCreateTemplateCompilesCode(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	newCampaign := func(t *testing.T) uuid.UUID {
		campaignID, err := management.NewCampaignsStore(mgmt).CreateCampaign(ctx, management.Campaign{
			ProjectID: projectID,
			Name:      "Test Campaign",
			Channel:   "email",
		})
		require.NoError(t, err)
		return campaignID
	}

	create := func(t *testing.T, caller pubsub.Caller, campaignID uuid.UUID) *httptest.ResponseRecorder {
		controller := NewTemplatesController(logger, mgmt, mgmt, pubsub.NewEmailRenderer(caller), nil, engine, nil, "")
		bb, err := json.Marshal(oapi.CreateTemplate{
			Locale: "en",
			Data:   ptr.To(json.RawMessage(`{"subject":"Hi","code":{"source":"<Html />"}}`)),
		})
		require.NoError(t, err)

		res := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/v1/campaigns/"+campaignID.String()+"/templates", bytes.NewReader(bb))
		req = req.WithContext(actorCtx)
		controller.CreateTemplate(res, req, projectID, campaignID)
		return res
	}

	t.Run("stores the compiled code with the template", func(t *testing.T) {
		res := create(t, compileCaller{}, newCampaign(t))
		require.Equal(t, 201, res.Code, res.Body.String())

		var created struct {
			Data struct {
				Subject string `json:"subject"`
				Code    struct {
					Source     string `json:"source"`
					Bundle     string `json:"bundle"`
					BundleHash string `json:"bundle_hash"`
				} `json:"code"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &created))
		require.Equal(t, "Hi", created.Data.Subject)
		require.Equal(t, "<Html />", created.Data.Code.Source)
		require.Equal(t, "compiled:<Html />", created.Data.Code.Bundle)
		require.NotEmpty(t, created.Data.Code.BundleHash)
	})

	t.Run("creates nothing when the code doesn't compile", func(t *testing.T) {
		campaignID := newCampaign(t)
		res := create(t, compileCaller{err: errors.New("syntax error")}, campaignID)
		require.Equal(t, 500, res.Code, res.Body.String())

		stored, err := management.NewTemplatesStore(mgmt).ListTemplates(ctx, projectID, campaignID)
		require.NoError(t, err)
		require.Empty(t, stored)
	})
}

// TestGetTemplateMatchesSpec reads the handler's response with the client
// generated from the spec, so the spec and the server can't drift apart again.
func TestGetTemplateMatchesSpec(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaignID, err := management.NewCampaignsStore(mgmt).CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	templateID, err := management.NewTemplatesStore(mgmt).CreateTemplate(ctx, projectID, campaignID, "email", "en", nil)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewTemplatesController(logger, mgmt, mgmt, pubsub.NewEmailRenderer(pubsub.NewNoopCaller()), nil, engine, nil, "")

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/campaigns/"+campaignID.String()+"/templates/"+templateID.String(), nil)
	req = req.WithContext(actorCtx)
	controller.GetTemplate(res, req, projectID, campaignID, templateID)
	require.Equal(t, 200, res.Code, res.Body.String())

	parsed, err := oapi.ParseGetTemplateResponse(res.Result())
	require.NoError(t, err)
	require.NotNil(t, parsed.JSON200)
	require.Equal(t, templateID, parsed.JSON200.Id)
	require.Equal(t, "en", parsed.JSON200.Locale)
}

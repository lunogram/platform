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

func TestGetTemplate(t *testing.T) {
	t.Parallel()

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

	templateID, err := templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
	require.NoError(t, err)

	controller := NewTemplatesController(logger, db)

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
			controller.GetTemplate(res, req, projectID, campaignID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestCreateTemplate(t *testing.T) {
	t.Parallel()

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
	campaignID, err := campaigns.CreateCampaign(ctx, store.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	controller := NewTemplatesController(logger, db)

	type test struct {
		body interface{}
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
			controller.CreateTemplate(res, req, projectID, campaignID)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestUpdateTemplate(t *testing.T) {
	t.Parallel()

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

	templateID, err := templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
	require.NoError(t, err)

	controller := NewTemplatesController(logger, db)

	type test struct {
		id   uuid.UUID
		body interface{}
		code int
	}

	tests := map[string]test{
		"success": {
			id: templateID,
			body: oapi.UpdateTemplate{
				Data: ptr(json.RawMessage(`{"subject":"Updated Subject"}`)),
			},
			code: 200,
		},
		"not found": {
			id: uuid.Nil,
			body: oapi.UpdateTemplate{
				Data: ptr(json.RawMessage(`{}`)),
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
			controller.UpdateTemplate(res, req, projectID, campaignID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestDeleteTemplate(t *testing.T) {
	t.Parallel()

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

	templateID, err := templates.CreateTemplate(ctx, projectID, campaignID, "email", "en-US")
	require.NoError(t, err)

	controller := NewTemplatesController(logger, db)

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
			controller.DeleteTemplate(res, req, projectID, campaignID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

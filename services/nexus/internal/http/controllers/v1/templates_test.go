package v1

import (
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGetTemplate(t *testing.T) {
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

	tests := map[string]struct {
		id   uuid.UUID
		code int
	}{
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
			req := httptest.NewRequest("GET", "/v1/templates/"+test.id.String(), nil)
			controller.GetTemplate(res, req, projectID, test.id)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

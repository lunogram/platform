//go:build !enterprise

package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Variants are an enterprise capability. An open-source build must refuse to
// create one rather than accept a template no build of the console can manage.
func TestCreateTemplateRejectsVariantInOSS(t *testing.T) {
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

	body, err := json.Marshal(oapi.CreateTemplate{Locale: "en", Variant: ptr.To("acme")})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/campaigns/"+campaignID.String()+"/templates", bytes.NewReader(body))
	req = req.WithContext(actorCtx)
	controller.CreateTemplate(res, req, projectID, campaignID)

	require.Equal(t, 404, res.Code, res.Body.String())

	// The default variant is not gated: every open-source template is one.
	body, err = json.Marshal(oapi.CreateTemplate{Locale: "en"})
	require.NoError(t, err)

	res = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/v1/campaigns/"+campaignID.String()+"/templates", bytes.NewReader(body))
	req = req.WithContext(actorCtx)
	controller.CreateTemplate(res, req, projectID, campaignID)

	require.Equal(t, 201, res.Code, res.Body.String())
}

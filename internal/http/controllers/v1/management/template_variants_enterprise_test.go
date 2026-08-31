//go:build enterprise

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
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// A template may only be created for a variant the campaign declares. Without
// that check a typo produces a template no send can ever resolve to and the
// console has no name to list it under.
func TestCreateTemplateRejectsUndeclaredVariant(t *testing.T) {
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

	err = campaigns.UpdateCampaign(ctx, projectID, campaignID, management.CampaignUpdate{
		Variants: &store.JSONB[management.CampaignVariants]{
			Data: management.CampaignVariants{{Key: "acme", Label: "Acme Corp"}},
		},
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewTemplatesController(logger, mgmt, mgmt, pubsub.NewEmailRenderer(pubsub.NewNoopCaller()), nil, engine, nil, "")

	tests := map[string]struct {
		variant *string
		code    int
	}{
		"declared variant":   {variant: ptr.To("acme"), code: 201},
		"undeclared variant": {variant: ptr.To("globex"), code: 400},
		"default variant":    {variant: nil, code: 201},
		"empty variant":      {variant: ptr.To(""), code: 201},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			body, err := json.Marshal(oapi.CreateTemplate{Locale: "en", Variant: test.variant})
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/campaigns/"+campaignID.String()+"/templates", bytes.NewReader(body))
			req = req.WithContext(actorCtx)
			controller.CreateTemplate(res, req, projectID, campaignID)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

// Variants and their selector round-trip through the campaign update path, and
// an empty selector clears it rather than being ignored as a no-op.
func TestUpdateCampaignVariants(t *testing.T) {
	t.Parallel()

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

	campaign, err := campaigns.GetCampaign(ctx, projectID, campaignID)
	require.NoError(t, err)
	require.Empty(t, campaign.Variants.Data)
	require.Nil(t, campaign.VariantSelector)

	err = campaigns.UpdateCampaign(ctx, projectID, campaignID, management.CampaignUpdate{
		Variants: &store.JSONB[management.CampaignVariants]{
			Data: management.CampaignVariants{{Key: "acme", Label: "Acme Corp"}},
		},
		VariantSelector: ptr.To("{{ user.data.tenant }}"),
	})
	require.NoError(t, err)

	campaign, err = campaigns.GetCampaign(ctx, projectID, campaignID)
	require.NoError(t, err)
	require.Equal(t, management.CampaignVariants{{Key: "acme", Label: "Acme Corp"}}, campaign.Variants.Data)
	require.Equal(t, "{{ user.data.tenant }}", *campaign.VariantSelector)

	// An update that touches neither field leaves both alone.
	err = campaigns.UpdateCampaign(ctx, projectID, campaignID, management.CampaignUpdate{
		Name: ptr.To("Renamed"),
	})
	require.NoError(t, err)

	campaign, err = campaigns.GetCampaign(ctx, projectID, campaignID)
	require.NoError(t, err)
	require.Len(t, campaign.Variants.Data, 1)
	require.NotNil(t, campaign.VariantSelector)

	err = campaigns.UpdateCampaign(ctx, projectID, campaignID, management.CampaignUpdate{
		VariantSelector: ptr.To(""),
	})
	require.NoError(t, err)

	campaign, err = campaigns.GetCampaign(ctx, projectID, campaignID)
	require.NoError(t, err)
	require.Nil(t, campaign.VariantSelector)
}

// Duplicating a campaign has to carry variant templates across, or the copy
// silently loses every white-labelled edition.
func TestDuplicateTemplateCarriesVariant(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	campaigns := management.NewCampaignsStore(mgmt)
	templates := management.NewTemplatesStore(mgmt)

	sourceID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID, Name: "Source", Channel: "email",
	})
	require.NoError(t, err)

	targetID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID, Name: "Target", Channel: "email",
	})
	require.NoError(t, err)

	templateID, err := templates.CreateTemplate(ctx, projectID, sourceID, "email", "en", "acme", nil)
	require.NoError(t, err)

	require.NoError(t, templates.DuplicateTemplate(ctx, projectID, templateID, targetID))

	copied, err := templates.ListTemplates(ctx, projectID, targetID)
	require.NoError(t, err)
	require.Len(t, copied, 1)
	require.Equal(t, "acme", copied[0].Variant)
}

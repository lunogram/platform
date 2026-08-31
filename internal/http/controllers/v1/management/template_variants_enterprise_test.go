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
			Data: management.CampaignVariants{
				Options: []management.CampaignVariant{{Key: "acme", Label: "Acme Corp"}},
			},
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
	require.Empty(t, campaign.Variants.Data.Options)
	require.Nil(t, campaign.Variants.Data.Selector)

	selector := management.VariantSelector{
		Type:       management.VariantSelectorExpression,
		Expression: "{{ user.data.tenant }}",
	}
	err = campaigns.UpdateCampaign(ctx, projectID, campaignID, management.CampaignUpdate{
		Variants: &store.JSONB[management.CampaignVariants]{
			Data: management.CampaignVariants{
				Selector: &selector,
				Options:  []management.CampaignVariant{{Key: "acme", Label: "Acme Corp"}},
			},
		},
	})
	require.NoError(t, err)

	campaign, err = campaigns.GetCampaign(ctx, projectID, campaignID)
	require.NoError(t, err)
	require.Equal(t, []management.CampaignVariant{{Key: "acme", Label: "Acme Corp"}}, campaign.Variants.Data.Options)
	require.Equal(t, selector, *campaign.Variants.Data.Selector)

	// An update that does not touch variants leaves the whole object alone.
	err = campaigns.UpdateCampaign(ctx, projectID, campaignID, management.CampaignUpdate{
		Name: ptr.To("Renamed"),
	})
	require.NoError(t, err)

	campaign, err = campaigns.GetCampaign(ctx, projectID, campaignID)
	require.NoError(t, err)
	require.Len(t, campaign.Variants.Data.Options, 1)
	require.NotNil(t, campaign.Variants.Data.Selector)

	// Sending the object without a selector clears it.
	err = campaigns.UpdateCampaign(ctx, projectID, campaignID, management.CampaignUpdate{
		Variants: &store.JSONB[management.CampaignVariants]{
			Data: management.CampaignVariants{
				Options: []management.CampaignVariant{{Key: "acme", Label: "Acme Corp"}},
			},
		},
	})
	require.NoError(t, err)

	campaign, err = campaigns.GetCampaign(ctx, projectID, campaignID)
	require.NoError(t, err)
	require.Nil(t, campaign.Variants.Data.Selector)
	require.Len(t, campaign.Variants.Data.Options, 1)
}

// A request body is rejected before it reaches the database when it declares a
// duplicate or empty key, or a selector pointing at a variant that is not in
// the same body.
func TestCampaignVariantsFromOAPI(t *testing.T) {
	t.Parallel()

	options := []oapi.CampaignVariant{{Key: "acme"}}

	tests := map[string]struct {
		body    oapi.CampaignVariants
		wantErr bool
	}{
		"options only": {
			body: oapi.CampaignVariants{Options: &options},
		},
		"selector naming a declared variant": {
			body: oapi.CampaignVariants{
				Options: &options,
				Selector: &oapi.VariantSelector{
					Type: "static",
					Key:  ptr.To("acme"),
				},
			},
		},
		"selector naming an undeclared variant": {
			body: oapi.CampaignVariants{
				Options: &options,
				Selector: &oapi.VariantSelector{
					Type: "static",
					Key:  ptr.To("globex"),
				},
			},
			wantErr: true,
		},
		"empty key": {
			body:    oapi.CampaignVariants{Options: &[]oapi.CampaignVariant{{Key: "  "}}},
			wantErr: true,
		},
		"duplicate key": {
			body:    oapi.CampaignVariants{Options: &[]oapi.CampaignVariant{{Key: "acme"}, {Key: "acme"}}},
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := management.CampaignVariantsFromOAPI(test.body)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
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

// A broadcast may pin one variant or carry its own expression. A static key is
// checked against the campaign's declared variants when the broadcast is
// created: falling back at send time would quietly put the whole list under
// house branding while the operator believes they picked a client.
func TestCreateBroadcastVariantSelector(t *testing.T) {
	t.Parallel()
	env := newBroadcastTestEnv(t)

	err := env.mgmtState.CampaignsStore.UpdateCampaign(t.Context(), env.projectID, env.campaignID, management.CampaignUpdate{
		Variants: &store.JSONB[management.CampaignVariants]{
			Data: management.CampaignVariants{
				Options: []management.CampaignVariant{{Key: "acme", Label: "Acme Corp"}},
			},
		},
	})
	require.NoError(t, err)

	tests := map[string]struct {
		selector *oapi.VariantSelector
		code     int
	}{
		"no selector defers to the campaign": {
			code: 201,
		},
		"static key the campaign declares": {
			selector: &oapi.VariantSelector{Type: "static", Key: ptr.To("acme")},
			code:     201,
		},
		"static key the campaign does not declare": {
			selector: &oapi.VariantSelector{Type: "static", Key: ptr.To("globex")},
			code:     400,
		},
		"static without a key": {
			selector: &oapi.VariantSelector{Type: "static"},
			code:     400,
		},
		"expression is accepted unvalidated": {
			selector: &oapi.VariantSelector{Type: "expression", Expression: ptr.To("{{ user.data.tenant }}")},
			code:     201,
		},
		"expression without an expression": {
			selector: &oapi.VariantSelector{Type: "expression"},
			code:     400,
		},
		"unknown selector type": {
			selector: &oapi.VariantSelector{Type: "whatever", Key: ptr.To("acme")},
			code:     400,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			body, err := json.Marshal(oapi.CreateBroadcastJSONRequestBody{
				CampaignId: env.campaignID,
				ListId:     env.listID,
				Variant:    test.selector,
			})
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/broadcasts", bytes.NewReader(body))
			req = env.actorCtx(req)
			env.controller.CreateBroadcast(res, req, env.projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())
			if test.code != 201 {
				return
			}

			var broadcast oapi.Broadcast
			require.NoError(t, json.Unmarshal(res.Body.Bytes(), &broadcast))

			if test.selector == nil {
				require.Nil(t, broadcast.Variant)
				return
			}
			require.NotNil(t, broadcast.Variant)
			require.Equal(t, test.selector.Type, broadcast.Variant.Type)
		})
	}
}

package journeys

import (
	"fmt"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/ratelimit"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/store/journey"
	providers "github.com/lunogram/platform/pkg/modules/providers"
)

func HandleCampaign(ctx HandlerContext, step journey.JourneyVersionStep, state journey.JourneyUserState) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.CampaignStepData](step.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to decode campaign step data: %w", err)
	}

	// Resolve Liquid expressions in campaign step data values against the journey context.
	// Each value is either a static string or a Liquid expression like "{{ journey.entrance_data.order_id }}"
	// that resolves to a plain string value at step execution time.
	var data map[string]string
	if len(config.Data) > 0 {
		data = make(map[string]string, len(config.Data))
		for key, value := range config.Data {
			resolved, err := render.RenderString(value, ctx.Data)
			if err != nil {
				return state, nil, fmt.Errorf("failed to render campaign data %q: %w", key, err)
			}
			data[key] = resolved
		}
	}

	// Resolve the provider rate limit so the consumer doesn't need to look
	// up the manifest or DB record.
	var rl ratelimit.Limit
	campaign, err := ctx.Management.GetCampaign(ctx, ctx.ProjectID, config.CampaignId)
	if err != nil {
		return state, nil, fmt.Errorf("failed to get campaign: %w", err)
	}

	if campaign.Provider != nil {
		var manifest *providers.RateLimit
		if ctx.ProviderRegistry != nil {
			if p, ok := ctx.ProviderRegistry.Get(campaign.Provider.Module); ok {
				manifest = p.Manifest().Spec.RateLimit
			}
		}
		rl = providers.ResolveLimit(
			ratelimit.ProviderKey(campaign.Provider.ID),
			manifest,
			campaign.Provider.RateLimit,
			campaign.Provider.RateInterval,
		)
	}

	msg := schemas.SendCampaign{
		ProjectID:  ctx.ProjectID,
		UserID:     ctx.UserID,
		CampaignID: config.CampaignId,
		Data:       data,
		RateLimit:  rl,
	}

	err = ctx.Publisher.Publish(ctx, schemas.Subject(schemas.CampaignsSend(ctx.ProjectID, config.CampaignId)), msg)
	if err != nil {
		return state, nil, fmt.Errorf("failed to publish campaign send: %w", err)
	}

	state.CompletedAt = Now()
	return state, step.Children, nil
}

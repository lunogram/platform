package journeys

import (
	"fmt"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/store/journey"
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

	// A step that names a variant decides the branding for this send outright,
	// so it is resolved here against the journey context and travels with the
	// event. Left unset, the campaign's own selector resolves one per recipient
	// at render time.
	var variant *string
	if config.Variant != nil && *config.Variant != "" {
		resolved, err := render.RenderString(*config.Variant, ctx.Data)
		if err != nil {
			return state, nil, fmt.Errorf("failed to render campaign variant: %w", err)
		}
		variant = &resolved
	}

	msg := schemas.SendCampaign{
		ProjectID:  ctx.ProjectID,
		UserID:     ctx.UserID,
		CampaignID: config.CampaignId,
		Data: &schemas.SendCampaignData{
			JourneyID:      &state.JourneyID,
			JourneyEntryID: &state.JourneyEntryID,
			JourneyStepID:  &step.ExternalID,
		},
		Variables: data,
		Variant:   variant,
	}

	err = ctx.Publisher.Publish(ctx, schemas.Subject(schemas.CampaignsSend(ctx.ProjectID, config.CampaignId)), msg)
	if err != nil {
		return state, nil, fmt.Errorf("failed to publish campaign send: %w", err)
	}

	state.CompletedAt = Now()
	return state, step.Children, nil
}

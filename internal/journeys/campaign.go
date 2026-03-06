package journeys

import (
	"fmt"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
)

func HandleCampaign(ctx HandlerContext, step journey.JourneyVersionStep, state journey.JourneyUserState) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.CampaignStepData](step.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to decode campaign step data: %w", err)
	}

	msg := schemas.SendCampaign{
		ProjectID:  ctx.ProjectID,
		UserID:     ctx.UserID,
		CampaignID: config.CampaignId,
	}

	err = ctx.Publisher.Publish(ctx, schemas.Subject(schemas.CampaignsSend(ctx.ProjectID, config.CampaignId)), msg)
	if err != nil {
		return state, nil, fmt.Errorf("failed to publish campaign send: %w", err)
	}

	state.CompletedAt = Now()
	return state, step.Children, nil
}

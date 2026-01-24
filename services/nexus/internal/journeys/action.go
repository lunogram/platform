package journeys

import (
	"fmt"

	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

func HandleAction(ctx HandlerContext, step store.JourneyVersionStep, state store.JourneyUserState) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.ActionStepData](step.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to decode link step data: %w", err)
	}

	// TODO: support multiple action types
	msg := schemas.SendCampaign{
		ProjectID:  ctx.ProjectID,
		UserID:     ctx.UserID,
		CampaignID: config.CampaignId,
	}

	err = ctx.Publisher.Publish(ctx, schemas.Subject(schemas.CampaignsSend(ctx.ProjectID, config.CampaignId)), msg)
	if err != nil {
		return state, nil, fmt.Errorf("failed to publish user updated event: %w", err)
	}

	state.CompletedAt = Now()
	return state, step.Children, nil
}

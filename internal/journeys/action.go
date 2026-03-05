package journeys

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
	actiontypes "github.com/lunogram/platform/pkg/modules/actions"
	"go.uber.org/zap"
)

func HandleAction(ctx HandlerContext, step journey.JourneyVersionStep, state journey.JourneyUserState) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.ActionStepData](step.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to decode action step data: %w", err)
	}

	// If an ActionId is set, execute the WASM action
	if config.ActionId != nil {
		action, err := ctx.Management.ActionsStore.GetAction(ctx, ctx.ProjectID, *config.ActionId)
		if err != nil {
			return state, nil, fmt.Errorf("failed to get action %s: %w", config.ActionId, err)
		}

		module, exists := ctx.ActionRegistry.Get(action.Type)
		if !exists {
			return state, nil, fmt.Errorf("unknown action type: %s", action.Type)
		}

		// Build the action config from the stored action's config
		var actionConfig map[string]any
		if action.Config.Data != nil {
			b, err := json.Marshal(action.Config.Data)
			if err != nil {
				return state, nil, fmt.Errorf("failed to marshal action config: %w", err)
			}
			if err := json.Unmarshal(b, &actionConfig); err != nil {
				return state, nil, fmt.Errorf("failed to unmarshal action config: %w", err)
			}
		}
		if actionConfig == nil {
			actionConfig = map[string]any{}
		}

		req := &actiontypes.ExecuteRequest[map[string]any]{
			Config:    actionConfig,
			Variables: ctx.Data,
		}

		result, err := module.Execute(ctx, req)
		if err != nil {
			return state, nil, fmt.Errorf("action execution failed: %w", err)
		}

		// Publish metadata to NATS for schema extraction if present
		if result.Metadata != nil {
			schemaMsg := schemas.ActionSchema{
				ProjectID: ctx.ProjectID,
				ActionID:  *config.ActionId,
				Metadata:  result.Metadata,
			}
			if pubErr := ctx.Publisher.Publish(ctx, schemas.ActionsSchema(ctx.ProjectID), schemaMsg); pubErr != nil {
				// Non-fatal: schema extraction failure should not block the action step
				zap.L().Warn("failed to publish action schema", zap.Error(pubErr), zap.Stringer("action_id", *config.ActionId))
			}
		}

		state.CompletedAt = Now()
		return state, step.Children, nil
	}

	// Legacy path: send campaign
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

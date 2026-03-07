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

	action, err := ctx.Management.ActionsStore.GetAction(ctx, ctx.ProjectID, config.ActionId)
	if err != nil {
		return state, nil, fmt.Errorf("failed to get action %s: %w", config.ActionId, err)
	}

	module, exists := ctx.ActionRegistry.Get(action.Type)
	if !exists {
		return state, nil, fmt.Errorf("unknown action type: %s", action.Type)
	}

	req := &actiontypes.ExecuteRequest[json.RawMessage]{}

	req.Config, err = json.Marshal(action.Config.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to marshal action config: %w", err)
	}

	if config.Input != nil {
		req.Input = config.Input
	}

	result, err := module.Execute(ctx, config.FunctionId, req)
	if err != nil {
		return state, nil, fmt.Errorf("action execution failed: %w", err)
	}

	if result.Metadata != nil {
		schema := schemas.ActionSchema{
			ProjectID:  ctx.ProjectID,
			ActionID:   config.ActionId,
			FunctionID: config.FunctionId,
			Metadata:   result.Metadata,
		}

		err = ctx.Publisher.Publish(ctx, schemas.ActionsSchema(ctx.ProjectID), schema)
		if err != nil {
			// Non-fatal: schema extraction failure should not block the action step
			zap.L().Warn("failed to publish action schema", zap.Error(err), zap.Stringer("action_id", config.ActionId))
		}
	}

	state.CompletedAt = Now()
	return state, step.Children, nil
}

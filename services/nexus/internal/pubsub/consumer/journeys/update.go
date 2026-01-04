package journeys

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/osteele/liquid"
)

func HandleUpdate(ctx HandlerContext, step store.JourneyVersionStep, state store.JourneyUserState) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.UpdateStepData](step.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to decode update step data: %w", err)
	}

	if config.Template == "" {
		state.CompletedAt = Now()
		return state, step.Children, nil
	}

	engine := liquid.NewEngine()
	rendered, err := engine.ParseAndRenderString(config.Template, ctx.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to render template: %w", err)
	}

	var check map[string]any
	err = json.Unmarshal([]byte(rendered), &check)
	if err != nil {
		return state, nil, fmt.Errorf("failed to parse rendered template as JSON: %w", err)
	}

	updated := json.RawMessage(rendered)

	userStore := store.NewUsersStore(ctx.DB)
	err = userStore.UpdateUser(ctx, ctx.UserID, store.UserUpdate{
		Data: &updated,
	})
	if err != nil {
		return state, nil, fmt.Errorf("failed to update user: %w", err)
	}

	state.CompletedAt = Now()
	state.Data = updated

	return state, step.Children, nil
}

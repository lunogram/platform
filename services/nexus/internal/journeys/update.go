package journeys

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
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

	users := store.NewUsersStore(ctx.DB)
	err = users.UpdateUser(ctx, ctx.UserID, store.UserUpdate{
		Data: &updated,
	})
	if err != nil {
		return state, nil, fmt.Errorf("failed to update user: %w", err)
	}

	state.CompletedAt = Now()
	state.Data = updated

	user, err := users.GetUser(ctx, ctx.ProjectID, ctx.UserID)
	if err != nil {
		return state, nil, fmt.Errorf("failed to get updated user: %w", err)
	}

	data := map[string]any{}
	err = json.Unmarshal(user.Data, &data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to unmarshal user data: %w", err)
	}

	msg := schemas.User{
		ProjectID:   ctx.ProjectID,
		ID:          user.ID,
		AnonymousID: user.AnonymousID,
		ExternalID:  user.ExternalID,
		Email:       user.Email,
		Phone:       user.Phone,
		Timezone:    user.Timezone,
		Locale:      user.Locale,
		Data:        data,
		Version:     user.Version,
	}

	err = ctx.Publisher.Publish(ctx, schemas.Subject(schemas.UsersProcess(ctx.ProjectID)), msg)
	if err != nil {
		return state, nil, fmt.Errorf("failed to publish user updated event: %w", err)
	}

	return state, step.Children, nil
}

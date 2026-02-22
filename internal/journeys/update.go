package journeys

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/osteele/liquid"
)

func HandleUpdate(ctx HandlerContext, step journey.JourneyVersionStep, state journey.JourneyUserState) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
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

	usersStore := subjects.NewUsersStore(ctx.DB)
	err = usersStore.UpdateUser(ctx, ctx.UserID, subjects.UserUpdate{
		Data: &updated,
	})
	if err != nil {
		return state, nil, fmt.Errorf("failed to update user: %w", err)
	}

	state.CompletedAt = Now()
	state.Data = updated

	user, err := usersStore.GetUser(ctx, ctx.ProjectID, ctx.UserID)
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

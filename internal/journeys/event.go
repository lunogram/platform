package journeys

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/subjects"
)

func HandleEvent(ctx HandlerContext, step journey.JourneyVersionStep, state journey.JourneyUserState) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.EventStepData](step.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to decode event step data: %w", err)
	}

	if config.EventName == "" {
		return state, nil, fmt.Errorf("event_name is required")
	}

	eventName, err := render.RenderString(config.EventName, ctx.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to render event_name: %w", err)
	}

	var payload map[string]any
	if config.Template != nil && *config.Template != "" {
		rendered, err := render.RenderString(*config.Template, ctx.Data)
		if err != nil {
			return state, nil, fmt.Errorf("failed to render template: %w", err)
		}

		if err := json.Unmarshal([]byte(rendered), &payload); err != nil {
			return state, nil, fmt.Errorf("failed to parse rendered template as JSON: %w", err)
		}
	}

	usersStore := subjects.NewUsersStore(ctx.DB)
	user, err := usersStore.GetUser(ctx, ctx.ProjectID, ctx.UserID)
	if err != nil {
		return state, nil, fmt.Errorf("failed to get user: %w", err)
	}

	identifiers := user.ExternalIDs.Params()

	event := schemas.UserEvent{
		Name:        eventName,
		ProjectID:   ctx.ProjectID,
		UserID:      ctx.UserID,
		Identifiers: identifiers,
		Data:        payload,
	}

	err = ctx.Publisher.Publish(ctx, schemas.Subject(schemas.UserEventsProcess(ctx.ProjectID)), event)
	if err != nil {
		return state, nil, fmt.Errorf("failed to publish event: %w", err)
	}

	state.CompletedAt = Now()
	state, err = WithStateData(state, event)
	if err != nil {
		return state, nil, fmt.Errorf("failed to update state with event data: %w", err)
	}

	return state, step.Children, nil
}

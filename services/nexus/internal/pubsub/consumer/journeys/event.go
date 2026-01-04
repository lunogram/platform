package journeys

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/osteele/liquid"
)

func HandleEvent(ctx HandlerContext, step store.JourneyVersionStep, state store.JourneyUserState) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.EventStepData](step.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to decode event step data: %w", err)
	}

	if config.EventName == "" {
		return state, nil, fmt.Errorf("event_name is required")
	}

	var payload map[string]any
	if config.Template != nil && *config.Template != "" {
		engine := liquid.NewEngine()
		rendered, err := engine.ParseAndRenderString(*config.Template, ctx.Data)
		if err != nil {
			return state, nil, fmt.Errorf("failed to render template: %w", err)
		}

		if err := json.Unmarshal([]byte(rendered), &payload); err != nil {
			return state, nil, fmt.Errorf("failed to parse rendered template as JSON: %w", err)
		}
	}

	users := store.NewUsersStore(ctx.DB)
	user, err := users.GetUser(ctx, ctx.ProjectID, ctx.UserID)
	if err != nil {
		return state, nil, fmt.Errorf("failed to get user: %w", err)
	}

	event := schemas.Event{
		Name:        config.EventName,
		ProjectID:   ctx.ProjectID,
		UserID:      ctx.UserID,
		ExternalId:  user.ExternalID,
		AnonymousId: user.AnonymousID,
		Data:        payload,
	}

	err = ctx.Publisher.Publish(ctx, schemas.Subject(schemas.EventsProjectSubject(ctx.ProjectID)), event)
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

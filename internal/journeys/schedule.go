package journeys

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func HandleSchedule(ctx HandlerContext, step journey.JourneyVersionStep, state journey.JourneyUserState) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.ScheduleStepData](step.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to decode schedule step data: %w", err)
	}

	if config.ScheduleName == "" {
		return state, nil, fmt.Errorf("schedule_name is required")
	}

	scheduleName, err := render.RenderString(config.ScheduleName, ctx.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to render schedule_name: %w", err)
	}

	scheduledStore := subjects.NewScheduledStore(ctx.DB, zap.NewNop())

	schedule, err := scheduledStore.GetScheduleByName(ctx, ctx.ProjectID, scheduleName)
	if err != nil {
		return state, nil, fmt.Errorf("failed to get schedule %q: %w", scheduleName, err)
	}

	// Parse optional scheduled_at
	var scheduledAt *time.Time
	if config.ScheduledAt != nil && *config.ScheduledAt != "" {
		rendered, err := render.RenderString(*config.ScheduledAt, ctx.Data)
		if err != nil {
			return state, nil, fmt.Errorf("failed to render scheduled_at: %w", err)
		}

		t, err := time.Parse(time.RFC3339, rendered)
		if err != nil {
			return state, nil, fmt.Errorf("failed to parse scheduled_at %q as RFC3339: %w", rendered, err)
		}
		scheduledAt = &t
	}

	// Parse optional start_at
	var startAt *time.Time
	if config.StartAt != nil && *config.StartAt != "" {
		rendered, err := render.RenderString(*config.StartAt, ctx.Data)
		if err != nil {
			return state, nil, fmt.Errorf("failed to render start_at: %w", err)
		}

		t, err := time.Parse(time.RFC3339, rendered)
		if err != nil {
			return state, nil, fmt.Errorf("failed to parse start_at %q as RFC3339: %w", rendered, err)
		}
		startAt = &t
	}

	// Parse optional interval
	var interval *string
	if config.Interval != nil && *config.Interval != "" {
		rendered, err := render.RenderString(*config.Interval, ctx.Data)
		if err != nil {
			return state, nil, fmt.Errorf("failed to render interval: %w", err)
		}
		interval = &rendered
	}

	// Parse optional data template
	var data json.RawMessage
	if config.Template != nil && *config.Template != "" {
		rendered, err := render.RenderString(*config.Template, ctx.Data)
		if err != nil {
			return state, nil, fmt.Errorf("failed to render template: %w", err)
		}

		// Validate that the rendered template is valid JSON
		if !json.Valid([]byte(rendered)) {
			return state, nil, fmt.Errorf("rendered template is not valid JSON: %s", rendered)
		}
		data = json.RawMessage(rendered)
	}

	userSchedule, err := scheduledStore.UpsertUserSchedule(ctx, ctx.UserID, schedule.ID, scheduledAt, startAt, interval, data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to upsert user schedule: %w", err)
	}

	state.CompletedAt = Now()
	state, err = WithStateData(state, userSchedule)
	if err != nil {
		return state, nil, fmt.Errorf("failed to update state with schedule data: %w", err)
	}

	return state, step.Children, nil
}

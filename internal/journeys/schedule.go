package journeys

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

// scheduleAssignmentNamespace is a fixed namespace for deriving deterministic
// schedule-assignment ids from (user, schedule) in journey steps.
var scheduleAssignmentNamespace = uuid.MustParse("8e0f6a2c-3b4d-5e6f-7a8b-9c0d1e2f3a4b")

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

	scheduleType := "single"
	if config.Interval != nil && *config.Interval != "" {
		scheduleType = "recurring"
	}

	scheduleID, err := scheduledStore.UpsertSchedule(ctx, ctx.ProjectID, scheduleName, scheduleType)
	if err != nil {
		return state, nil, fmt.Errorf("failed to upsert schedule %q: %w", scheduleName, err)
	}

	// Parse optional scheduled_at
	scheduledAt, err := render.RenderTime(config.ScheduledAt, ctx.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to resolve scheduled_at: %w", err)
	}

	// Parse optional start_at
	startAt, err := render.RenderTime(config.StartAt, ctx.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to resolve start_at: %w", err)
	}

	// Apply sensible defaults matching the client API behaviour:
	// - single schedules default scheduled_at to now
	// - recurring schedules default start_at to now
	now := time.Now().UTC()
	if scheduleType == "single" && scheduledAt == nil {
		scheduledAt = &now
	}
	if scheduleType == "recurring" && startAt == nil {
		startAt = &now
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

	// Journey placements stay idempotent per (user, schedule): a deterministic
	// assignment id means re-running this step (e.g. on retry or journey
	// re-entry) updates the same assignment rather than creating duplicates,
	// preserving the behaviour from before assignments became individually
	// addressable.
	assignmentID := uuid.NewSHA1(scheduleAssignmentNamespace, append(ctx.UserID[:], scheduleID[:]...))

	userSchedule, err := scheduledStore.UpsertUserSchedule(ctx, assignmentID, ctx.UserID, scheduleID, scheduledAt, startAt, interval, data)
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

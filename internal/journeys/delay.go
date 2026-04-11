package journeys

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/store/journey"
)

func HandleDelay(ctx HandlerContext, step journey.JourneyVersionStep, state journey.JourneyUserState) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
	// NOTE: If state is not nil, it means the delay has already been processed once,
	// so we mark it as completed and proceed to the next step.
	if state.ResumeAt != nil && (state.ResumeAt.Before(time.Now()) || state.ResumeAt.Equal(time.Now())) {
		state.CompletedAt = Now()
		state.ResumeAt = nil
		return state, step.Children, nil
	}

	config, err := DecodeStepData[oapi.DelayStepData](step.Data)
	if err != nil {
		return state, nil, err
	}

	now := time.Now()
	state.ResumeAt, err = calculateResumeTime(now, config, ctx.Data)
	if err != nil {
		return state, nil, err
	}

	return state, nil, nil
}

// resolveInt renders a StringOrInt field as a Liquid template and parses
// the result as an integer. Returns 0 when the pointer is nil.
func resolveInt(field *oapi.StringOrInt, data map[string]any) (int, error) {
	if field == nil {
		return 0, nil
	}

	raw := field.String()
	rendered, err := render.RenderString(raw, data)
	if err != nil {
		return 0, fmt.Errorf("failed to render duration field: %w", err)
	}

	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return 0, nil
	}

	v, err := strconv.Atoi(rendered)
	if err != nil {
		return 0, fmt.Errorf("duration field %q is not a valid integer: %w", rendered, err)
	}
	return v, nil
}

// resolveTimeString renders a StringOrInt field as a Liquid template and
// returns the resulting string. Used for the "time" format (HH:mm).
func resolveTimeString(field *oapi.StringOrInt, data map[string]any) (string, error) {
	if field == nil {
		return "", nil
	}

	raw := field.String()
	rendered, err := render.RenderString(raw, data)
	if err != nil {
		return "", fmt.Errorf("failed to render time field: %w", err)
	}
	return strings.TrimSpace(rendered), nil
}

// userTimezone extracts the user's timezone from the template data and loads
// the corresponding *time.Location. Falls back to UTC when unavailable.
func userTimezone(data map[string]any) *time.Location {
	user, ok := data["user"].(map[string]any)
	if !ok {
		return time.UTC
	}

	tz, ok := user["timezone"].(string)
	if !ok || tz == "" {
		return time.UTC
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func calculateResumeTime(now time.Time, config oapi.DelayStepData, data map[string]any) (*time.Time, error) {
	var resumeAt time.Time

	switch config.Format {
	case oapi.Duration:
		days, err := resolveInt(config.Days, data)
		if err != nil {
			return nil, fmt.Errorf("days: %w", err)
		}
		hours, err := resolveInt(config.Hours, data)
		if err != nil {
			return nil, fmt.Errorf("hours: %w", err)
		}
		minutes, err := resolveInt(config.Minutes, data)
		if err != nil {
			return nil, fmt.Errorf("minutes: %w", err)
		}

		duration := time.Duration(days)*24*time.Hour +
			time.Duration(hours)*time.Hour +
			time.Duration(minutes)*time.Minute
		resumeAt = now.Add(duration)

	case oapi.Time:
		timeStr, err := resolveTimeString(config.Time, data)
		if err != nil {
			return nil, err
		}
		if timeStr == "" {
			return nil, errors.New("time is required for time format")
		}

		timeParts, err := time.Parse("15:04", timeStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse time %q: %w", timeStr, err)
		}

		// Use the user's timezone so the delay fires at the correct local time.
		loc := userTimezone(data)
		nowLocal := now.In(loc)
		resumeAt = time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(),
			timeParts.Hour(), timeParts.Minute(), 0, 0, loc)
		if resumeAt.Before(now) {
			resumeAt = resumeAt.Add(24 * time.Hour)
		}

	case oapi.Date:
		if config.Date == nil {
			return nil, errors.New("date is required for date format")
		}

		loc := userTimezone(data)
		parsed, err := render.RenderTime(config.Date, data,
			render.WithFormats(time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"),
			render.WithFallbackLocation(loc),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve date: %w", err)
		}
		if parsed == nil {
			return nil, errors.New("date resolved to an empty value")
		}
		resumeAt = *parsed

	default:
		return nil, errors.New("unsupported delay format")
	}

	if config.ExclusionDays != nil && len(*config.ExclusionDays) > 0 {
		resumeAt = skipExcludedDays(resumeAt, *config.ExclusionDays)
	}

	return &resumeAt, nil
}

func skipExcludedDays(t time.Time, exclusionDays []int) time.Time {
	exclusionMap := make(map[int]bool)
	for _, day := range exclusionDays {
		exclusionMap[day] = true
	}

	for exclusionMap[int(t.Weekday())] {
		t = t.Add(24 * time.Hour)
	}

	return t
}

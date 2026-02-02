package journeys

import (
	"errors"
	"time"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

func HandleDelay(ctx HandlerContext, step store.JourneyVersionStep, state store.JourneyUserState) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
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
	state.ResumeAt, err = calculateResumeTime(now, config)
	if err != nil {
		return state, nil, err
	}

	return state, nil, nil
}

func calculateResumeTime(now time.Time, config oapi.DelayStepData) (*time.Time, error) {
	var resumeAt time.Time

	switch config.Format {
	case oapi.Duration:
		duration := time.Duration(0)
		if config.Days != nil {
			duration += time.Duration(*config.Days) * 24 * time.Hour
		}
		if config.Hours != nil {
			duration += time.Duration(*config.Hours) * time.Hour
		}
		if config.Minutes != nil {
			duration += time.Duration(*config.Minutes) * time.Minute
		}
		resumeAt = now.Add(duration)

	case oapi.Time:
		if config.Time == nil {
			return nil, errors.New("time is required for time format")
		}
		timeParts, err := time.Parse("15:04", *config.Time)
		if err != nil {
			return nil, err
		}
		resumeAt = time.Date(now.Year(), now.Month(), now.Day(), timeParts.Hour(), timeParts.Minute(), 0, 0, now.Location())
		if resumeAt.Before(now) {
			resumeAt = resumeAt.Add(24 * time.Hour)
		}

	case oapi.Date:
		if config.Date == nil {
			return nil, errors.New("date is required for date format")
		}
		parsed, err := time.Parse(time.RFC3339, *config.Date)
		if err != nil {
			return nil, err
		}
		resumeAt = parsed

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

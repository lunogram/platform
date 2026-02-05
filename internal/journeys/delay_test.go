package journeys

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleDelay(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()

	type test struct {
		step      journey.JourneyVersionStep
		state     *journey.JourneyUserState
		wantState journey.JourneyUserState
		wantErr   bool
	}

	fifteenMinutes := 15
	durationData := oapi.DelayStepData{
		Format:  oapi.Duration,
		Minutes: &fifteenMinutes,
	}
	durationJSON, _ := json.Marshal(durationData)
	durationRaw := json.RawMessage(durationJSON)

	timeStr := "14:00"
	timeData := oapi.DelayStepData{
		Format: oapi.Time,
		Time:   &timeStr,
	}
	timeJSON, _ := json.Marshal(timeData)
	timeRaw := json.RawMessage(timeJSON)

	tests := map[string]test{
		"nil state with duration delay": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: "delay",
				Data: durationRaw,
			},
			state: nil,
			wantState: journey.JourneyUserState{
				ResumeAt: func() *time.Time {
					t := now.Add(15 * time.Minute)
					return &t
				}(),
			},
			wantErr: false,
		},
		"existing state marks as completed": {
			step: journey.JourneyVersionStep{
				ID:       uuid.New(),
				Type:     "delay",
				Data:     durationRaw,
				Children: []journey.JourneyVersionStepChild{{ChildExternalID: "child1"}},
			},
			state: &journey.JourneyUserState{
				ResumeAt: func() *time.Time {
					t := now.Add(-1 * time.Hour)
					return &t
				}(),
			},
			wantState: journey.JourneyUserState{
				ResumeAt: func() *time.Time {
					t := now.Add(-1 * time.Hour)
					return &t
				}(),
				CompletedAt: &now,
			},
			wantErr: false,
		},
		"past resume time marks as completed immediately": {
			step: journey.JourneyVersionStep{
				ID:       uuid.New(),
				Type:     "delay",
				Data:     timeRaw,
				Children: []journey.JourneyVersionStepChild{{ChildExternalID: "child1"}},
			},
			state:   nil,
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			hctx := HandlerContext{
				Context: ctx,
			}
			var state journey.JourneyUserState
			if tc.state != nil {
				state = *tc.state
			}
			gotState, gotChildren, err := HandleDelay(hctx, tc.step, state)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tc.state != nil {
				assert.NotNil(t, gotState.CompletedAt)
				assert.Equal(t, tc.step.Children, gotChildren)
			}

			if tc.state == nil && tc.step.Data != nil {
				if gotState.ResumeAt != nil {
					assert.NotNil(t, gotState.ResumeAt)
					assert.True(t, gotState.ResumeAt.After(now) || gotState.ResumeAt.Equal(now))
				} else {
					assert.NotNil(t, gotState.CompletedAt)
				}
			}
		})
	}
}

func TestCalculateResumeTime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)

	type test struct {
		delayData oapi.DelayStepData
		checkFunc func(t *testing.T, resumeAt time.Time)
		wantErr   bool
	}

	tests := map[string]test{
		"duration with days, hours, minutes": {
			delayData: oapi.DelayStepData{
				Format:  oapi.Duration,
				Days:    ptr(1),
				Hours:   ptr(2),
				Minutes: ptr(30),
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				expected := now.Add(26*time.Hour + 30*time.Minute)
				assert.Equal(t, expected, resumeAt)
			},
			wantErr: false,
		},
		"duration with only minutes": {
			delayData: oapi.DelayStepData{
				Format:  oapi.Duration,
				Minutes: ptr(45),
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				expected := now.Add(45 * time.Minute)
				assert.Equal(t, expected, resumeAt)
			},
			wantErr: false,
		},
		"time format - future time today": {
			delayData: oapi.DelayStepData{
				Format: oapi.Time,
				Time:   ptr("15:30"),
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				assert.Equal(t, 2026, resumeAt.Year())
				assert.Equal(t, time.January, resumeAt.Month())
				assert.Equal(t, 2, resumeAt.Day())
				assert.Equal(t, 15, resumeAt.Hour())
				assert.Equal(t, 30, resumeAt.Minute())
			},
			wantErr: false,
		},
		"time format - past time rolls to next day": {
			delayData: oapi.DelayStepData{
				Format: oapi.Time,
				Time:   ptr("08:00"),
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				assert.Equal(t, 2026, resumeAt.Year())
				assert.Equal(t, time.January, resumeAt.Month())
				assert.Equal(t, 3, resumeAt.Day())
				assert.Equal(t, 8, resumeAt.Hour())
			},
			wantErr: false,
		},
		"time format - missing time": {
			delayData: oapi.DelayStepData{
				Format: oapi.Time,
			},
			wantErr: true,
		},
		"date format - valid RFC3339": {
			delayData: oapi.DelayStepData{
				Format: oapi.Date,
				Date:   ptr("2026-02-15T14:30:00Z"),
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				assert.Equal(t, 2026, resumeAt.Year())
				assert.Equal(t, time.February, resumeAt.Month())
				assert.Equal(t, 15, resumeAt.Day())
				assert.Equal(t, 14, resumeAt.Hour())
				assert.Equal(t, 30, resumeAt.Minute())
			},
			wantErr: false,
		},
		"date format - missing date": {
			delayData: oapi.DelayStepData{
				Format: oapi.Date,
			},
			wantErr: true,
		},
		"duration with exclusion days": {
			delayData: oapi.DelayStepData{
				Format:        oapi.Duration,
				Hours:         ptr(1),
				ExclusionDays: &[]int{4, 5},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				weekday := int(resumeAt.Weekday())
				assert.NotEqual(t, 4, weekday)
				assert.NotEqual(t, 5, weekday)
			},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resumeAt, err := calculateResumeTime(now, tc.delayData)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resumeAt)
			if tc.checkFunc != nil {
				tc.checkFunc(t, *resumeAt)
			}
		})
	}
}

func TestSkipExcludedDays(t *testing.T) {
	t.Parallel()

	type test struct {
		startTime     time.Time
		exclusionDays []int
		checkFunc     func(t *testing.T, result time.Time)
	}

	tests := map[string]test{
		"skip saturday and sunday": {
			startTime:     time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC),
			exclusionDays: []int{0, 6},
			checkFunc: func(t *testing.T, result time.Time) {
				weekday := int(result.Weekday())
				assert.NotEqual(t, 0, weekday)
				assert.NotEqual(t, 6, weekday)
				assert.Equal(t, 1, weekday)
			},
		},
		"no exclusion needed": {
			startTime:     time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC),
			exclusionDays: []int{0, 6},
			checkFunc: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2026, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 5, result.Day())
			},
		},
		"skip multiple consecutive days": {
			startTime:     time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC),
			exclusionDays: []int{0, 1, 6},
			checkFunc: func(t *testing.T, result time.Time) {
				weekday := int(result.Weekday())
				assert.Equal(t, 2, weekday)
			},
		},
		"empty exclusion list": {
			startTime:     time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC),
			exclusionDays: []int{},
			checkFunc: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2026, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 3, result.Day())
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := skipExcludedDays(tc.startTime, tc.exclusionDays)
			tc.checkFunc(t, result)
		})
	}
}

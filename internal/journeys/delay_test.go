package journeys

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func soiInt(v int) *oapi.StringOrInt {
	s := oapi.StringOrInt(strconv.Itoa(v))
	return &s
}

func soiStr(v string) *oapi.StringOrInt {
	s := oapi.StringOrInt(v)
	return &s
}

func TestHandleDelay(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()

	type test struct {
		step      journey.JourneyVersionStep
		state     *journey.JourneyUserState
		data      map[string]any
		wantState journey.JourneyUserState
		wantErr   bool
	}

	durationData := oapi.DelayStepData{
		Format:  oapi.Duration,
		Minutes: soiInt(15),
	}
	durationJSON, _ := json.Marshal(durationData)
	durationRaw := json.RawMessage(durationJSON)

	timeData := oapi.DelayStepData{
		Format: oapi.Time,
		Time:   soiStr("14:00"),
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
			data:  map[string]any{},
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
			data: map[string]any{},
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
			data:    map[string]any{},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			hctx := HandlerContext{
				Context: ctx,
				Data:    tc.data,
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
		data      map[string]any
		checkFunc func(t *testing.T, resumeAt time.Time)
		wantErr   bool
	}

	tests := map[string]test{
		"duration with days, hours, minutes": {
			delayData: oapi.DelayStepData{
				Format:  oapi.Duration,
				Days:    soiInt(1),
				Hours:   soiInt(2),
				Minutes: soiInt(30),
			},
			data: map[string]any{},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				expected := now.Add(26*time.Hour + 30*time.Minute)
				assert.Equal(t, expected, resumeAt)
			},
			wantErr: false,
		},
		"duration with only minutes": {
			delayData: oapi.DelayStepData{
				Format:  oapi.Duration,
				Minutes: soiInt(45),
			},
			data: map[string]any{},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				expected := now.Add(45 * time.Minute)
				assert.Equal(t, expected, resumeAt)
			},
			wantErr: false,
		},
		"duration with liquid template variables": {
			delayData: oapi.DelayStepData{
				Format:  oapi.Duration,
				Days:    soiStr("{{ journey.entrance.wait_days }}"),
				Hours:   soiInt(0),
				Minutes: soiInt(0),
			},
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"wait_days": "3",
					},
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				expected := now.Add(3 * 24 * time.Hour)
				assert.Equal(t, expected, resumeAt)
			},
			wantErr: false,
		},
		"duration with all fields as liquid templates": {
			delayData: oapi.DelayStepData{
				Format:  oapi.Duration,
				Days:    soiStr("{{ journey.entrance.d }}"),
				Hours:   soiStr("{{ journey.entrance.h }}"),
				Minutes: soiStr("{{ journey.entrance.m }}"),
			},
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"d": "1",
						"h": "6",
						"m": "15",
					},
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				expected := now.Add(24*time.Hour + 6*time.Hour + 15*time.Minute)
				assert.Equal(t, expected, resumeAt)
			},
			wantErr: false,
		},
		"duration with liquid template that resolves to non-integer": {
			delayData: oapi.DelayStepData{
				Format:  oapi.Duration,
				Days:    soiStr("{{ journey.entrance.val }}"),
				Hours:   soiInt(0),
				Minutes: soiInt(0),
			},
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"val": "abc",
					},
				},
			},
			wantErr: true,
		},
		"time format - future time today (UTC user)": {
			delayData: oapi.DelayStepData{
				Format: oapi.Time,
				Time:   soiStr("15:30"),
			},
			data: map[string]any{
				"user": map[string]any{
					"timezone": "UTC",
				},
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
				Time:   soiStr("08:00"),
			},
			data: map[string]any{
				"user": map[string]any{
					"timezone": "UTC",
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				assert.Equal(t, 2026, resumeAt.Year())
				assert.Equal(t, time.January, resumeAt.Month())
				assert.Equal(t, 3, resumeAt.Day())
				assert.Equal(t, 8, resumeAt.Hour())
			},
			wantErr: false,
		},
		"time format - uses user timezone": {
			delayData: oapi.DelayStepData{
				Format: oapi.Time,
				Time:   soiStr("12:00"),
			},
			// now is 2026-01-02T10:00:00Z which is 2026-01-02T11:00:00+01:00 in Europe/Amsterdam
			// Target: 12:00 Amsterdam time = 11:00 UTC (still in the future from 10:00 UTC)
			data: map[string]any{
				"user": map[string]any{
					"timezone": "Europe/Amsterdam",
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				ams, _ := time.LoadLocation("Europe/Amsterdam")
				local := resumeAt.In(ams)
				assert.Equal(t, 12, local.Hour())
				assert.Equal(t, 0, local.Minute())
				assert.Equal(t, 2, local.Day())
			},
			wantErr: false,
		},
		"time format - user timezone causes next-day rollover": {
			delayData: oapi.DelayStepData{
				Format: oapi.Time,
				Time:   soiStr("10:00"),
			},
			// now is 2026-01-02T10:00:00Z = 2026-01-02T11:00:00+01:00 in Amsterdam
			// Target: 10:00 Amsterdam = 09:00 UTC, which is in the past, so should roll to next day
			data: map[string]any{
				"user": map[string]any{
					"timezone": "Europe/Amsterdam",
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				ams, _ := time.LoadLocation("Europe/Amsterdam")
				local := resumeAt.In(ams)
				assert.Equal(t, 10, local.Hour())
				assert.Equal(t, 0, local.Minute())
				assert.Equal(t, 3, local.Day()) // next day
			},
			wantErr: false,
		},
		"time format - no user timezone falls back to UTC": {
			delayData: oapi.DelayStepData{
				Format: oapi.Time,
				Time:   soiStr("15:30"),
			},
			data: map[string]any{},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				utc := resumeAt.In(time.UTC)
				assert.Equal(t, 15, utc.Hour())
				assert.Equal(t, 30, utc.Minute())
			},
			wantErr: false,
		},
		"time format - invalid timezone falls back to UTC": {
			delayData: oapi.DelayStepData{
				Format: oapi.Time,
				Time:   soiStr("15:30"),
			},
			data: map[string]any{
				"user": map[string]any{
					"timezone": "Not/A/Timezone",
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				utc := resumeAt.In(time.UTC)
				assert.Equal(t, 15, utc.Hour())
				assert.Equal(t, 30, utc.Minute())
			},
			wantErr: false,
		},
		"time format - liquid template for time value": {
			delayData: oapi.DelayStepData{
				Format: oapi.Time,
				Time:   soiStr("{{ journey.entrance.notify_at }}"),
			},
			data: map[string]any{
				"user": map[string]any{
					"timezone": "UTC",
				},
				"journey": map[string]any{
					"entrance": map[string]any{
						"notify_at": "18:00",
					},
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				assert.Equal(t, 18, resumeAt.Hour())
				assert.Equal(t, 0, resumeAt.Minute())
			},
			wantErr: false,
		},
		"time format - missing time": {
			delayData: oapi.DelayStepData{
				Format: oapi.Time,
			},
			data:    map[string]any{},
			wantErr: true,
		},
		"date format - valid RFC3339": {
			delayData: oapi.DelayStepData{
				Format: oapi.Date,
				Date:   ptr("2026-02-15T14:30:00Z"),
			},
			data: map[string]any{},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				assert.Equal(t, 2026, resumeAt.Year())
				assert.Equal(t, time.February, resumeAt.Month())
				assert.Equal(t, 15, resumeAt.Day())
				assert.Equal(t, 14, resumeAt.Hour())
				assert.Equal(t, 30, resumeAt.Minute())
			},
			wantErr: false,
		},
		"date format - liquid template rendering": {
			delayData: oapi.DelayStepData{
				Format: oapi.Date,
				Date:   ptr("{{ journey.entrance.resume_date }}"),
			},
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"resume_date": "2026-06-15T09:00:00Z",
					},
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				assert.Equal(t, 2026, resumeAt.Year())
				assert.Equal(t, time.June, resumeAt.Month())
				assert.Equal(t, 15, resumeAt.Day())
				assert.Equal(t, 9, resumeAt.Hour())
			},
			wantErr: false,
		},
		"date format - date-only string uses user timezone at midnight": {
			delayData: oapi.DelayStepData{
				Format: oapi.Date,
				Date:   ptr("2026-03-10"),
			},
			data: map[string]any{
				"user": map[string]any{
					"timezone": "America/New_York",
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				ny, _ := time.LoadLocation("America/New_York")
				local := resumeAt.In(ny)
				assert.Equal(t, 2026, local.Year())
				assert.Equal(t, time.March, local.Month())
				assert.Equal(t, 10, local.Day())
				assert.Equal(t, 0, local.Hour())
				assert.Equal(t, 0, local.Minute())
			},
			wantErr: false,
		},
		"date format - datetime without timezone uses user timezone": {
			delayData: oapi.DelayStepData{
				Format: oapi.Date,
				Date:   ptr("2026-07-04T09:30:00"),
			},
			data: map[string]any{
				"user": map[string]any{
					"timezone": "Europe/Amsterdam",
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				ams, _ := time.LoadLocation("Europe/Amsterdam")
				local := resumeAt.In(ams)
				assert.Equal(t, 2026, local.Year())
				assert.Equal(t, time.July, local.Month())
				assert.Equal(t, 4, local.Day())
				assert.Equal(t, 9, local.Hour())
				assert.Equal(t, 30, local.Minute())
			},
			wantErr: false,
		},
		"date format - date-only without user timezone falls back to UTC": {
			delayData: oapi.DelayStepData{
				Format: oapi.Date,
				Date:   ptr("2026-03-10"),
			},
			data: map[string]any{},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				utc := resumeAt.In(time.UTC)
				assert.Equal(t, 2026, utc.Year())
				assert.Equal(t, time.March, utc.Month())
				assert.Equal(t, 10, utc.Day())
				assert.Equal(t, 0, utc.Hour())
				assert.Equal(t, 0, utc.Minute())
			},
			wantErr: false,
		},
		"date format - RFC3339 with offset ignores user timezone": {
			delayData: oapi.DelayStepData{
				Format: oapi.Date,
				Date:   ptr("2026-07-04T09:30:00+05:30"),
			},
			data: map[string]any{
				"user": map[string]any{
					"timezone": "America/New_York",
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				// The offset in the value (+05:30) should be honoured, not the user tz
				_, offset := resumeAt.Zone()
				assert.Equal(t, 5*60*60+30*60, offset)
				assert.Equal(t, 9, resumeAt.Hour())
				assert.Equal(t, 30, resumeAt.Minute())
			},
			wantErr: false,
		},
		"date format - liquid resolves to date-only with user timezone": {
			delayData: oapi.DelayStepData{
				Format: oapi.Date,
				Date:   ptr("{{ journey.entrance.target_date }}"),
			},
			data: map[string]any{
				"user": map[string]any{
					"timezone": "Asia/Tokyo",
				},
				"journey": map[string]any{
					"entrance": map[string]any{
						"target_date": "2026-12-25",
					},
				},
			},
			checkFunc: func(t *testing.T, resumeAt time.Time) {
				tokyo, _ := time.LoadLocation("Asia/Tokyo")
				local := resumeAt.In(tokyo)
				assert.Equal(t, 2026, local.Year())
				assert.Equal(t, time.December, local.Month())
				assert.Equal(t, 25, local.Day())
				assert.Equal(t, 0, local.Hour())
				assert.Equal(t, 0, local.Minute())
			},
			wantErr: false,
		},
		"date format - invalid date string": {
			delayData: oapi.DelayStepData{
				Format: oapi.Date,
				Date:   ptr("not-a-date"),
			},
			data:    map[string]any{},
			wantErr: true,
		},
		"date format - missing date": {
			delayData: oapi.DelayStepData{
				Format: oapi.Date,
			},
			data:    map[string]any{},
			wantErr: true,
		},
		"duration with exclusion days": {
			delayData: oapi.DelayStepData{
				Format:        oapi.Duration,
				Hours:         soiInt(1),
				ExclusionDays: &[]int{4, 5},
			},
			data: map[string]any{},
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
			resumeAt, err := calculateResumeTime(now, tc.delayData, tc.data)

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

func TestCalculateResumeTimeBackwardCompat(t *testing.T) {
	t.Parallel()

	// Simulate legacy JSON data with integer fields: {"format":"duration","minutes":15,"days":1}
	legacyJSON := json.RawMessage(`{"format":"duration","minutes":15,"days":1}`)

	var config oapi.DelayStepData
	require.NoError(t, json.Unmarshal(legacyJSON, &config))

	assert.Equal(t, oapi.Duration, config.Format)
	require.NotNil(t, config.Minutes)
	assert.Equal(t, "15", config.Minutes.String())
	require.NotNil(t, config.Days)
	assert.Equal(t, "1", config.Days.String())

	now := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	resumeAt, err := calculateResumeTime(now, config, map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, resumeAt)

	expected := now.Add(24*time.Hour + 15*time.Minute)
	assert.Equal(t, expected, *resumeAt)
}

func TestCalculateResumeTimeStringFields(t *testing.T) {
	t.Parallel()

	// Simulate new JSON data with string fields: {"format":"duration","minutes":"30","hours":"2"}
	newJSON := json.RawMessage(`{"format":"duration","minutes":"30","hours":"2"}`)

	var config oapi.DelayStepData
	require.NoError(t, json.Unmarshal(newJSON, &config))

	assert.Equal(t, oapi.Duration, config.Format)
	require.NotNil(t, config.Minutes)
	assert.Equal(t, "30", config.Minutes.String())
	require.NotNil(t, config.Hours)
	assert.Equal(t, "2", config.Hours.String())

	now := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	resumeAt, err := calculateResumeTime(now, config, map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, resumeAt)

	expected := now.Add(2*time.Hour + 30*time.Minute)
	assert.Equal(t, expected, *resumeAt)
}

func TestStringOrIntUnmarshal(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input    string
		expected string
	}{
		"integer value":       {input: `5`, expected: "5"},
		"zero":                {input: `0`, expected: "0"},
		"large integer":       {input: `120`, expected: "120"},
		"string number":       {input: `"15"`, expected: "15"},
		"string template":     {input: `"{{ user.days }}"`, expected: "{{ user.days }}"},
		"string time":         {input: `"14:30"`, expected: "14:30"},
		"empty string":        {input: `""`, expected: ""},
		"negative integer":    {input: `-1`, expected: "-1"},
		"string negative":     {input: `"-5"`, expected: "-5"},
		"float rounds to int": {input: `3.9`, expected: "3"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var s oapi.StringOrInt
			err := json.Unmarshal([]byte(tc.input), &s)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, s.String())
		})
	}

	t.Run("null is handled", func(t *testing.T) {
		type wrapper struct {
			Val *oapi.StringOrInt `json:"val"`
		}
		var w wrapper
		require.NoError(t, json.Unmarshal([]byte(`{"val":null}`), &w))
		assert.Nil(t, w.Val)
	})

	t.Run("omitted field stays nil", func(t *testing.T) {
		type wrapper struct {
			Val *oapi.StringOrInt `json:"val,omitempty"`
		}
		var w wrapper
		require.NoError(t, json.Unmarshal([]byte(`{}`), &w))
		assert.Nil(t, w.Val)
	})

	t.Run("boolean fails", func(t *testing.T) {
		var s oapi.StringOrInt
		err := json.Unmarshal([]byte(`true`), &s)
		assert.Error(t, err)
	})

	t.Run("object fails", func(t *testing.T) {
		var s oapi.StringOrInt
		err := json.Unmarshal([]byte(`{"a":1}`), &s)
		assert.Error(t, err)
	})
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

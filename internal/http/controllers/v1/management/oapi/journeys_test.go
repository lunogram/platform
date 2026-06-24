package oapi

import (
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rules"
	"github.com/stretchr/testify/require"
)

func TestEntranceStepData_Validate(t *testing.T) {
	t.Parallel()

	listID := uuid.New()

	cases := []struct {
		name    string
		data    EntranceStepData
		wantErr bool
	}{
		{
			name: "valid manual trigger",
			data: EntranceStepData{Trigger: TriggerManual},
		},
		{
			name: "manual trigger must not carry a block",
			data: EntranceStepData{
				Trigger: TriggerManual,
				Event:   &EventTrigger{Name: "order.created"},
			},
			wantErr: true,
		},
		{
			name: "valid event trigger",
			data: EntranceStepData{
				Trigger: TriggerEvent,
				Event:   &EventTrigger{Name: "order.created"},
			},
		},
		{
			name: "valid scheduled trigger",
			data: EntranceStepData{
				Trigger:   TriggerScheduled,
				Scheduled: &ScheduledTrigger{Name: "reminder"},
			},
		},
		{
			name: "valid list trigger with default direction",
			data: EntranceStepData{
				Trigger: TriggerList,
				List:    &ListTrigger{ID: listID},
			},
		},
		{
			name: "valid list trigger leaves",
			data: EntranceStepData{
				Trigger: TriggerList,
				List:    &ListTrigger{ID: listID, Direction: ListLeaves},
			},
		},
		{
			name:    "unknown trigger",
			data:    EntranceStepData{Trigger: "segment"},
			wantErr: true,
		},
		{
			name:    "empty trigger",
			data:    EntranceStepData{},
			wantErr: true,
		},
		{
			name: "trigger without its block",
			data: EntranceStepData{
				Trigger: TriggerEvent,
			},
			wantErr: true,
		},
		{
			name: "mismatched block",
			data: EntranceStepData{
				Trigger: TriggerEvent,
				List:    &ListTrigger{ID: listID},
			},
			wantErr: true,
		},
		{
			name: "two blocks set",
			data: EntranceStepData{
				Trigger: TriggerList,
				List:    &ListTrigger{ID: listID},
				Event:   &EventTrigger{Name: "order.created"},
			},
			wantErr: true,
		},
		{
			name: "event trigger missing event name",
			data: EntranceStepData{
				Trigger: TriggerEvent,
				Event:   &EventTrigger{},
			},
			wantErr: true,
		},
		{
			name: "list trigger missing list id",
			data: EntranceStepData{
				Trigger: TriggerList,
				List:    &ListTrigger{},
			},
			wantErr: true,
		},
		{
			name: "list trigger invalid direction",
			data: EntranceStepData{
				Trigger: TriggerList,
				List:    &ListTrigger{ID: listID, Direction: "sideways"},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.data.Validate()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestEntranceStepData_RuleAccessors(t *testing.T) {
	t.Parallel()

	rule := &rules.RuleSet{}
	member := &rules.RuleSet{}

	event := EntranceStepData{
		Trigger: TriggerEvent,
		Event:   &EventTrigger{Name: "x", Rule: rule, UserRule: member},
	}
	require.Equal(t, rule, event.EntranceRule())
	require.Equal(t, member, event.MemberRule())

	// List triggers carry no rule; the backend match is authoritative.
	list := EntranceStepData{
		Trigger: TriggerList,
		List:    &ListTrigger{ID: uuid.New()},
	}
	require.Nil(t, list.EntranceRule())
	require.Nil(t, list.MemberRule())
}

package v1

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/stretchr/testify/require"
)

func entranceStep(t *testing.T, data oapi.EntranceStepData) oapi.JourneyStep {
	t.Helper()
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	return oapi.JourneyStep{Type: oapi.JourneyStepTypeEntrance, Data: raw}
}

func TestJourneyEntranceEventDependencies_ListTrigger(t *testing.T) {
	t.Parallel()

	listID := uuid.New()

	cases := []struct {
		name      string
		data      oapi.EntranceStepData
		wantInMap bool
		wantEnter string
		wantExit  string
	}{
		{
			name: "join with exit on leave",
			data: oapi.EntranceStepData{
				Trigger: oapi.TriggerList,
				List: &oapi.ListTrigger{
					ID:          listID,
					Direction:   oapi.ListJoins,
					ExitOnLeave: true,
				},
			},
			wantInMap: true,
			wantEnter: schemas.EventListUserAdded,
			wantExit:  schemas.EventListUserRemoved,
		},
		{
			name: "join without exit on leave",
			data: oapi.EntranceStepData{
				Trigger: oapi.TriggerList,
				List: &oapi.ListTrigger{
					ID:          listID,
					Direction:   oapi.ListJoins,
					ExitOnLeave: false,
				},
			},
			wantInMap: true,
			wantEnter: schemas.EventListUserAdded,
			wantExit:  "",
		},
		{
			name: "leave direction never exits",
			data: oapi.EntranceStepData{
				Trigger: oapi.TriggerList,
				List: &oapi.ListTrigger{
					ID:          listID,
					Direction:   oapi.ListLeaves,
					ExitOnLeave: true,
				},
			},
			wantInMap: true,
			wantEnter: schemas.EventListUserRemoved,
			wantExit:  "",
		},
		{
			name: "default direction is join",
			data: oapi.EntranceStepData{
				Trigger: oapi.TriggerList,
				List: &oapi.ListTrigger{
					ID:          listID,
					ExitOnLeave: true,
				},
			},
			wantInMap: true,
			wantEnter: schemas.EventListUserAdded,
			wantExit:  schemas.EventListUserRemoved,
		},
		{
			name: "list trigger without a list is ignored",
			data: oapi.EntranceStepData{
				Trigger: oapi.TriggerList,
			},
			wantInMap: false,
		},
		{
			name: "event trigger only registers enter",
			data: oapi.EntranceStepData{
				Trigger: oapi.TriggerEvent,
				Event: &oapi.EventTrigger{
					Name: "order.created",
				},
			},
			wantInMap: true,
			wantEnter: "order.created",
			wantExit:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			steps := oapi.JourneyStepMap{"entrance-1": entranceStep(t, tc.data)}
			deps, err := journeyEntranceEventDependencies(steps)
			require.NoError(t, err)

			dep, ok := deps["entrance-1"]
			require.Equal(t, tc.wantInMap, ok)
			if !tc.wantInMap {
				return
			}

			require.Equal(t, tc.wantEnter, dep.Enter)
			require.Equal(t, tc.wantExit, dep.Exit)
		})
	}
}

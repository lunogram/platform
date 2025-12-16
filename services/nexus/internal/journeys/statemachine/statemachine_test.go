package statemachine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// mockAction is a test action implementation
type mockAction struct {
	executeFn func(ctx context.Context, sctx *Context) error
}

func (m *mockAction) Execute(ctx context.Context, sctx *Context) error {
	if m.executeFn != nil {
		return m.executeFn(ctx, sctx)
	}
	return nil
}

// mockScheduler is a test delay scheduler implementation
type mockScheduler struct {
	scheduleFn func(ctx context.Context, userID, journeyID, stepID, entranceID uuid.UUID, delayUntil time.Time, data map[string]interface{}) error
	calls      []scheduleCall
}

type scheduleCall struct {
	userID     uuid.UUID
	journeyID  uuid.UUID
	stepID     uuid.UUID
	entranceID uuid.UUID
	delayUntil time.Time
	data       map[string]interface{}
}

func (m *mockScheduler) ScheduleDelay(ctx context.Context, userID, journeyID, stepID, entranceID uuid.UUID, delayUntil time.Time, data map[string]interface{}) error {
	m.calls = append(m.calls, scheduleCall{
		userID:     userID,
		journeyID:  journeyID,
		stepID:     stepID,
		entranceID: entranceID,
		delayUntil: delayUntil,
		data:       data,
	})
	if m.scheduleFn != nil {
		return m.scheduleFn(ctx, userID, journeyID, stepID, entranceID, delayUntil, data)
	}
	return nil
}

// mockState is a test state implementation
type mockState struct {
	executeFn func(ctx context.Context, sctx *Context) (*Transition, error)
}

func (m *mockState) Execute(ctx context.Context, sctx *Context) (*Transition, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, sctx)
	}
	return nil, nil
}

func TestStateMachine_RegisterState(t *testing.T) {
	t.Parallel()

	scheduler := &mockScheduler{}
	sm := NewStateMachine(scheduler)

	state := &mockState{}
	sm.RegisterState("test", state)

	retrievedState, exists := sm.GetState("test")
	require.True(t, exists)
	require.Equal(t, state, retrievedState)
}

func TestStateMachine_Execute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	type test struct {
		setupFn     func(*StateMachine, *mockScheduler)
		stateName   string
		stateCtx    *Context
		expectError bool
		validateFn  func(*testing.T, *Transition, *mockScheduler)
	}

	tests := map[string]test{
		"state_not_found": {
			setupFn:     func(sm *StateMachine, sched *mockScheduler) {},
			stateName:   "nonexistent",
			stateCtx:    &Context{},
			expectError: true,
		},
		"state_returns_error": {
			setupFn: func(sm *StateMachine, sched *mockScheduler) {
				state := &mockState{
					executeFn: func(ctx context.Context, sctx *Context) (*Transition, error) {
						return nil, errors.New("execution failed")
					},
				}
				sm.RegisterState("error_state", state)
			},
			stateName:   "error_state",
			stateCtx:    &Context{},
			expectError: true,
		},
		"state_returns_nil_transition": {
			setupFn: func(sm *StateMachine, sched *mockScheduler) {
				state := &mockState{
					executeFn: func(ctx context.Context, sctx *Context) (*Transition, error) {
						return nil, nil
					},
				}
				sm.RegisterState("exit_state", state)
			},
			stateName:   "exit_state",
			stateCtx:    &Context{},
			expectError: false,
			validateFn: func(t *testing.T, transition *Transition, sched *mockScheduler) {
				require.Nil(t, transition)
				require.Len(t, sched.calls, 0)
			},
		},
		"state_returns_transition_without_delay": {
			setupFn: func(sm *StateMachine, sched *mockScheduler) {
				nextStepID := uuid.New()
				state := &mockState{
					executeFn: func(ctx context.Context, sctx *Context) (*Transition, error) {
						return &Transition{
							NextStepID: &nextStepID,
						}, nil
					},
				}
				sm.RegisterState("next_state", state)
			},
			stateName:   "next_state",
			stateCtx:    &Context{},
			expectError: false,
			validateFn: func(t *testing.T, transition *Transition, sched *mockScheduler) {
				require.NotNil(t, transition)
				require.NotNil(t, transition.NextStepID)
				require.Nil(t, transition.Delay)
				require.Len(t, sched.calls, 0)
			},
		},
		"state_returns_transition_with_delay": {
			setupFn: func(sm *StateMachine, sched *mockScheduler) {
				nextStepID := uuid.New()
				delay := 5 * time.Minute
				state := &mockState{
					executeFn: func(ctx context.Context, sctx *Context) (*Transition, error) {
						return &Transition{
							NextStepID: &nextStepID,
							Delay:      &delay,
						}, nil
					},
				}
				sm.RegisterState("delay_state", state)
			},
			stateName: "delay_state",
			stateCtx: &Context{
				UserID:     uuid.New(),
				JourneyID:  uuid.New(),
				EntranceID: uuid.New(),
			},
			expectError: false,
			validateFn: func(t *testing.T, transition *Transition, sched *mockScheduler) {
				require.NotNil(t, transition)
				require.NotNil(t, transition.NextStepID)
				require.NotNil(t, transition.Delay)
				require.Len(t, sched.calls, 1)
				require.Equal(t, *transition.NextStepID, sched.calls[0].stepID)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scheduler := &mockScheduler{}
			sm := NewStateMachine(scheduler)

			if tc.setupFn != nil {
				tc.setupFn(sm, scheduler)
			}

			transition, err := sm.Execute(ctx, tc.stateName, tc.stateCtx)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.validateFn != nil {
					tc.validateFn(t, transition, scheduler)
				}
			}
		})
	}
}

func TestEntryState_Execute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	nextStepID := uuid.New()

	type test struct {
		action      Action
		expectError bool
		validateFn  func(*testing.T, *Transition)
	}

	tests := map[string]test{
		"without_action": {
			action:      nil,
			expectError: false,
			validateFn: func(t *testing.T, transition *Transition) {
				require.NotNil(t, transition)
				require.NotNil(t, transition.NextStepID)
				require.Equal(t, nextStepID, *transition.NextStepID)
				require.Nil(t, transition.Delay)
			},
		},
		"with_successful_action": {
			action: &mockAction{
				executeFn: func(ctx context.Context, sctx *Context) error {
					sctx.Data["executed"] = true
					return nil
				},
			},
			expectError: false,
			validateFn: func(t *testing.T, transition *Transition) {
				require.NotNil(t, transition)
				require.NotNil(t, transition.NextStepID)
				require.Equal(t, nextStepID, *transition.NextStepID)
			},
		},
		"with_failing_action": {
			action: &mockAction{
				executeFn: func(ctx context.Context, sctx *Context) error {
					return errors.New("action failed")
				},
			},
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			state := NewEntryState(nextStepID, tc.action)
			sctx := &Context{
				Data: make(map[string]interface{}),
			}

			transition, err := state.Execute(ctx, sctx)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.validateFn != nil {
					tc.validateFn(t, transition)
				}
			}
		})
	}
}

func TestDelayState_Execute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	nextStepID := uuid.New()
	duration := 10 * time.Minute

	state := NewDelayState(duration, nextStepID)
	sctx := &Context{}

	transition, err := state.Execute(ctx, sctx)

	require.NoError(t, err)
	require.NotNil(t, transition)
	require.NotNil(t, transition.NextStepID)
	require.Equal(t, nextStepID, *transition.NextStepID)
	require.NotNil(t, transition.Delay)
	require.Equal(t, duration, *transition.Delay)
}

func TestActionState_Execute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	nextStepID := uuid.New()

	type test struct {
		action      Action
		nextStepID  *uuid.UUID
		expectError bool
		validateFn  func(*testing.T, *Transition)
	}

	tests := map[string]test{
		"with_action_and_next_step": {
			action: &mockAction{
				executeFn: func(ctx context.Context, sctx *Context) error {
					sctx.Data["processed"] = true
					return nil
				},
			},
			nextStepID:  &nextStepID,
			expectError: false,
			validateFn: func(t *testing.T, transition *Transition) {
				require.NotNil(t, transition)
				require.NotNil(t, transition.NextStepID)
				require.Equal(t, nextStepID, *transition.NextStepID)
			},
		},
		"with_action_without_next_step": {
			action: &mockAction{
				executeFn: func(ctx context.Context, sctx *Context) error {
					return nil
				},
			},
			nextStepID:  nil,
			expectError: false,
			validateFn: func(t *testing.T, transition *Transition) {
				require.Nil(t, transition)
			},
		},
		"with_failing_action": {
			action: &mockAction{
				executeFn: func(ctx context.Context, sctx *Context) error {
					return errors.New("action failed")
				},
			},
			nextStepID:  &nextStepID,
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			state := NewActionState(tc.action, tc.nextStepID)
			sctx := &Context{
				Data: make(map[string]interface{}),
			}

			transition, err := state.Execute(ctx, sctx)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.validateFn != nil {
					tc.validateFn(t, transition)
				}
			}
		})
	}
}

func TestExitState_Execute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	type test struct {
		action      Action
		expectError bool
	}

	tests := map[string]test{
		"without_action": {
			action:      nil,
			expectError: false,
		},
		"with_successful_action": {
			action: &mockAction{
				executeFn: func(ctx context.Context, sctx *Context) error {
					sctx.Data["completed"] = true
					return nil
				},
			},
			expectError: false,
		},
		"with_failing_action": {
			action: &mockAction{
				executeFn: func(ctx context.Context, sctx *Context) error {
					return errors.New("action failed")
				},
			},
			expectError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			state := NewExitState(tc.action)
			sctx := &Context{
				Data: make(map[string]interface{}),
			}

			transition, err := state.Execute(ctx, sctx)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Nil(t, transition)
			}
		})
	}
}

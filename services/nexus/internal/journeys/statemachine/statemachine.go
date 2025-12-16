package statemachine

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Context carries data through state machine execution
type Context struct {
	UserID     uuid.UUID
	JourneyID  uuid.UUID
	EntranceID uuid.UUID
	StepID     uuid.UUID
	Data       map[string]interface{}
}

// Action represents an action that can be performed by a step
type Action interface {
	Execute(ctx context.Context, sctx *Context) error
}

// DelayScheduler handles scheduling delays in the database
type DelayScheduler interface {
	ScheduleDelay(ctx context.Context, userID, journeyID, stepID, entranceID uuid.UUID, delayUntil time.Time, data map[string]interface{}) error
}

// Transition represents a move to the next step
type Transition struct {
	NextStepID *uuid.UUID
	Delay      *time.Duration
}

// State represents a step in the journey that can be executed
type State interface {
	// Execute runs the state logic and returns the next transition
	Execute(ctx context.Context, sctx *Context) (*Transition, error)
}

// StateMachine manages journey execution
type StateMachine struct {
	states    map[string]State
	scheduler DelayScheduler
}

// NewStateMachine creates a new state machine
func NewStateMachine(scheduler DelayScheduler) *StateMachine {
	return &StateMachine{
		states:    make(map[string]State),
		scheduler: scheduler,
	}
}

// RegisterState adds a state to the state machine
func (sm *StateMachine) RegisterState(name string, state State) {
	sm.states[name] = state
}

// Execute runs a state and handles transitions including delays
func (sm *StateMachine) Execute(ctx context.Context, stateName string, sctx *Context) (*Transition, error) {
	state, exists := sm.states[stateName]
	if !exists {
		return nil, ErrStateNotFound
	}

	transition, err := state.Execute(ctx, sctx)
	if err != nil {
		return nil, err
	}

	if transition != nil && transition.Delay != nil && transition.NextStepID != nil {
		delayUntil := time.Now().Add(*transition.Delay)
		if err := sm.scheduler.ScheduleDelay(ctx, sctx.UserID, sctx.JourneyID, *transition.NextStepID, sctx.EntranceID, delayUntil, sctx.Data); err != nil {
			return nil, err
		}
	}

	return transition, nil
}

// GetState retrieves a registered state by name
func (sm *StateMachine) GetState(name string) (State, bool) {
	state, exists := sm.states[name]
	return state, exists
}

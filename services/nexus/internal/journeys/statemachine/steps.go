package statemachine

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EntryState represents an entry point into a journey
type EntryState struct {
	nextStepID uuid.UUID
	action     Action
}

// NewEntryState creates a new entry state
func NewEntryState(nextStepID uuid.UUID, action Action) *EntryState {
	return &EntryState{
		nextStepID: nextStepID,
		action:     action,
	}
}

// Execute runs the entry state logic
func (s *EntryState) Execute(ctx context.Context, sctx *Context) (*Transition, error) {
	if s.action != nil {
		if err := s.action.Execute(ctx, sctx); err != nil {
			return nil, err
		}
	}

	return &Transition{
		NextStepID: &s.nextStepID,
	}, nil
}

// DelayState represents a delay/wait step
type DelayState struct {
	duration   time.Duration
	nextStepID uuid.UUID
}

// NewDelayState creates a new delay state
func NewDelayState(duration time.Duration, nextStepID uuid.UUID) *DelayState {
	return &DelayState{
		duration:   duration,
		nextStepID: nextStepID,
	}
}

// Execute schedules the delay
func (s *DelayState) Execute(ctx context.Context, sctx *Context) (*Transition, error) {
	return &Transition{
		NextStepID: &s.nextStepID,
		Delay:      &s.duration,
	}, nil
}

// ActionState represents a step that performs an action
type ActionState struct {
	action     Action
	nextStepID *uuid.UUID
}

// NewActionState creates a new action state
func NewActionState(action Action, nextStepID *uuid.UUID) *ActionState {
	return &ActionState{
		action:     action,
		nextStepID: nextStepID,
	}
}

// Execute runs the action
func (s *ActionState) Execute(ctx context.Context, sctx *Context) (*Transition, error) {
	if s.action != nil {
		if err := s.action.Execute(ctx, sctx); err != nil {
			return nil, err
		}
	}

	if s.nextStepID == nil {
		return nil, nil
	}

	return &Transition{
		NextStepID: s.nextStepID,
	}, nil
}

// ExitState represents an exit point from a journey
type ExitState struct {
	action Action
}

// NewExitState creates a new exit state
func NewExitState(action Action) *ExitState {
	return &ExitState{
		action: action,
	}
}

// Execute runs the exit state logic
func (s *ExitState) Execute(ctx context.Context, sctx *Context) (*Transition, error) {
	if s.action != nil {
		if err := s.action.Execute(ctx, sctx); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

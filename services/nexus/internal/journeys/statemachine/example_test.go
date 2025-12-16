package statemachine_test

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/journeys/statemachine"
)

// Example demonstrating a simple welcome journey with a delay
func Example_welcomeJourney() {
	// Create a mock scheduler for this example
	scheduler := &mockScheduler{}

	// Create state machine
	sm := statemachine.NewStateMachine(scheduler)

	// Define step IDs
	welcomeStepID := uuid.New()
	delayStepID := uuid.New()
	followupStepID := uuid.New()

	// Define actions
	sendWelcome := &mockAction{
		executeFn: func(ctx context.Context, sctx *statemachine.Context) error {
			fmt.Println("Sending welcome message")
			return nil
		},
	}

	sendFollowup := &mockAction{
		executeFn: func(ctx context.Context, sctx *statemachine.Context) error {
			fmt.Println("Sending follow-up message")
			return nil
		},
	}

	// Register states
	sm.RegisterState("welcome", statemachine.NewEntryState(delayStepID, sendWelcome))
	sm.RegisterState("delay", statemachine.NewDelayState(24*time.Hour, followupStepID))
	sm.RegisterState("followup", statemachine.NewActionState(sendFollowup, nil))

	// Execute the journey
	ctx := context.Background()
	sctx := &statemachine.Context{
		UserID:     uuid.New(),
		JourneyID:  uuid.New(),
		StepID:     welcomeStepID,
		EntranceID: uuid.New(),
		Data:       make(map[string]interface{}),
	}

	// Step 1: Welcome
	transition, err := sm.Execute(ctx, "welcome", sctx)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Transition to next step: %v\n", transition.NextStepID != nil)

	// Step 2: Delay (schedules in database)
	sctx.StepID = delayStepID
	transition, err = sm.Execute(ctx, "delay", sctx)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Delay scheduled: %v\n", transition.Delay != nil)

	// Step 3: Follow-up (would be executed by scheduler after delay)
	sctx.StepID = followupStepID
	transition, err = sm.Execute(ctx, "followup", sctx)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Journey complete: %v\n", transition == nil)

	// Output:
	// Sending welcome message
	// Transition to next step: true
	// Delay scheduled: true
	// Sending follow-up message
	// Journey complete: true
}

// Mock types for the example
type mockScheduler struct {
	calls []scheduleCall
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
	return nil
}

type mockAction struct {
	executeFn func(ctx context.Context, sctx *statemachine.Context) error
}

func (m *mockAction) Execute(ctx context.Context, sctx *statemachine.Context) error {
	if m.executeFn != nil {
		return m.executeFn(ctx, sctx)
	}
	return nil
}

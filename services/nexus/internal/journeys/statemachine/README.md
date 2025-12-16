# Journeys State Machine

A simple, flexible state machine implementation for managing user journey flows.

## Overview

The state machine provides a framework for defining and executing multi-step user journeys. Each step can perform actions and transition to the next step, with support for delays scheduled in the database.

## Core Concepts

### Context

The `Context` carries data through state machine execution:

```go
type Context struct {
    UserID     uuid.UUID
    JourneyID  uuid.UUID
    EntranceID uuid.UUID
    StepID     uuid.UUID
    Data       map[string]interface{}
}
```

### State

A `State` represents a step in the journey that can be executed:

```go
type State interface {
    Execute(ctx context.Context, sctx *Context) (*Transition, error)
}
```

Each state's `Execute` method can:
- Perform actions (e.g., send a message, update data)
- Return a transition to the next step
- Return `nil` to end the journey

### Action

An `Action` performs a specific operation:

```go
type Action interface {
    Execute(ctx context.Context, sctx *Context) error
}
```

Actions are composable and can be attached to any state.

### Transition

A `Transition` defines the next step and optional delay:

```go
type Transition struct {
    NextStepID *uuid.UUID
    Delay      *time.Duration
}
```

### Delay Scheduling

Delays are scheduled in the database via the `DelayScheduler` interface. When a state returns a transition with a delay, the state machine:

1. Calculates `delay_until = now + delay`
2. Creates a `journey_user_step` record with:
   - `user_id`, `journey_id`, `step_id`, `entrance_id`
   - `delay_until` timestamp
   - `type = 'scheduled'`
   - Optional `data` as JSON

A separate scheduler process (to be implemented) will:
1. Query `journey_user_step` records where `delay_until <= now AND type = 'scheduled'`
2. Execute the corresponding step
3. Update or delete the record

## Built-in States

### EntryState

Entry point for a journey. Optionally performs an action and transitions to the next step.

```go
state := statemachine.NewEntryState(nextStepID, action)
```

### DelayState

Schedules a delay before transitioning to the next step.

```go
state := statemachine.NewDelayState(5*time.Minute, nextStepID)
```

### ActionState

Performs an action and optionally transitions to another step.

```go
state := statemachine.NewActionState(action, &nextStepID)
```

### ExitState

Exit point for a journey. Optionally performs a cleanup action.

```go
state := statemachine.NewExitState(action)
```

## Usage Example

```go
// Create state machine with database scheduler
db := /* your *sqlx.DB */
scheduler := statemachine.NewDBScheduler(db)
sm := statemachine.NewStateMachine(scheduler)

// Define steps
welcomeID := uuid.New()
delayID := uuid.New()
followupID := uuid.New()

// Register states
sm.RegisterState("welcome", statemachine.NewEntryState(delayID, sendWelcomeEmail))
sm.RegisterState("delay", statemachine.NewDelayState(24*time.Hour, followupID))
sm.RegisterState("followup", statemachine.NewActionState(sendFollowupEmail, nil))

// Execute a step
ctx := context.Background()
sctx := &statemachine.Context{
    UserID:     userID,
    JourneyID:  journeyID,
    EntranceID: entranceID,
    Data:       make(map[string]interface{}),
}

transition, err := sm.Execute(ctx, "welcome", sctx)
if err != nil {
    // handle error
}

// If transition has a delay, it's been scheduled in the database
// The scheduler will pick it up later
```

## Custom States

Implement the `State` interface for custom behavior:

```go
type MyCustomState struct {
    // your fields
}

func (s *MyCustomState) Execute(ctx context.Context, sctx *Context) (*Transition, error) {
    // your logic here
    
    // Return transition or nil to end
    return &Transition{
        NextStepID: &nextStepID,
        Delay:      &duration,
    }, nil
}
```

## Database Schema

The state machine uses the existing `journey_user_step` table:

```sql
CREATE TABLE journey_user_step (
    id uuid PRIMARY KEY,
    user_id uuid,
    journey_id uuid,
    step_id uuid,
    entrance_id uuid,
    type varchar(255),
    delay_until timestamptz,
    data jsonb,
    created_at timestamptz,
    updated_at timestamptz
);
```

For scheduled delays:
- `type = 'scheduled'`
- `delay_until` contains the execution timestamp
- `step_id` references the next step to execute (nullable if step doesn't exist in journey_steps yet)

## Future Integration

This state machine will be integrated with NATS in a future PR for:
- Distributed execution
- Event-driven transitions
- Real-time updates

The scheduler that picks up delayed steps from the database will also be implemented separately.

## Testing

Run tests:

```bash
go test ./services/nexus/internal/journeys/statemachine/... -v
```

Tests use testcontainers with PostgreSQL for integration testing of the database scheduler.

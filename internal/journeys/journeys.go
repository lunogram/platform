package journeys

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
)

const (
	ActionStepType     = "action"
	BalancerStepType   = "balancer"
	CampaignStepType   = "campaign"
	DelayStepType      = "delay"
	EventStepType      = "event"
	ExitStepType       = "exit"
	ExperimentStepType = "experiment"
	GateStepType       = "gate"
	ScheduleStepType   = "schedule"
	UpdateStepType     = "update"
)

// Publisher publishes messages to NATS JetStream.
type Publisher interface {
	Publish(ctx context.Context, subject schemas.Subject, v any) error
}

type HandlerContext struct {
	context.Context
	DB             *sqlx.DB
	Publisher      Publisher
	ProjectID      uuid.UUID
	UserID         uuid.UUID
	Step           journey.JourneyVersionStep
	Data           map[string]any
	Management     *management.State
	ActionRegistry *actions.Registry
}

func Handle(parent context.Context, db *sqlx.DB, pub pubsub.Publisher, projectID, userID uuid.UUID, step journey.JourneyVersionStep, state *journey.JourneyUserState, data map[string]any, mgmt *management.State, actionRegistry *actions.Registry) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
	ctx := HandlerContext{
		Context:        parent,
		DB:             db,
		Publisher:      pub,
		ProjectID:      projectID,
		UserID:         userID,
		Step:           step,
		Data:           data,
		Management:     mgmt,
		ActionRegistry: actionRegistry,
	}

	var s journey.JourneyUserState
	if state != nil {
		s = *state
	}

	switch step.Type {
	case ActionStepType:
		return HandleAction(ctx, step, s)
	case BalancerStepType:
		return HandleBalancer(ctx, step, s)
	case CampaignStepType:
		return HandleCampaign(ctx, step, s)
	case DelayStepType:
		return HandleDelay(ctx, step, s)
	case EventStepType:
		return HandleEvent(ctx, step, s)
	case ExitStepType:
		return HandleExit(ctx, step, s)
	case ExperimentStepType:
		return HandleExperiment(ctx, step, s)
	case GateStepType:
		return HandleGate(ctx, step, s)
	case ScheduleStepType:
		return HandleSchedule(ctx, step, s)
	case UpdateStepType:
		return HandleUpdate(ctx, step, s)
	}

	return journey.JourneyUserState{}, nil, errors.New("unsupported step type")
}

func Now() *time.Time {
	t := time.Now()
	return &t
}

func WithStateData[T any](state journey.JourneyUserState, val T) (_ journey.JourneyUserState, err error) {
	state.Data, err = EncodeStateData(val)
	if err != nil {
		return state, err
	}

	return state, nil
}

func DecodeStepData[T any](raw json.RawMessage) (empty T, _ error) {
	if raw == nil {
		return empty, nil
	}

	var v T
	err := json.Unmarshal(raw, &v)
	return v, err
}

func EncodeStateData[T any](v T) (json.RawMessage, error) {
	bb, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return bb, nil
}

package journeys

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

const (
	ActionStepType     = "action"
	BalancerStepType   = "balancer"
	DelayStepType      = "delay"
	EventStepType      = "event"
	ExitStepType       = "exit"
	ExperimentStepType = "experiment"
	GateStepType       = "gate"
	LinkStepType       = "link"
	UpdateStepType     = "update"
)

// Publisher publishes messages to NATS JetStream.
type Publisher interface {
	Publish(ctx context.Context, subject schemas.Subject, v any) error
}

type HandlerContext struct {
	context.Context
	DB        *sqlx.DB
	Publisher Publisher
	ProjectID uuid.UUID
	UserID    uuid.UUID
	Step      store.JourneyVersionStep
	Data      map[string]any
}

func Handle(parent context.Context, db *sqlx.DB, pub pubsub.Publisher, projectID, userID uuid.UUID, step store.JourneyVersionStep, state *store.JourneyUserState, data map[string]any) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	ctx := HandlerContext{
		Context:   parent,
		DB:        db,
		Publisher: pub,
		ProjectID: projectID,
		UserID:    userID,
		Step:      step,
		Data:      data,
	}

	var s store.JourneyUserState
	if state != nil {
		s = *state
	}

	switch step.Type {
	case ActionStepType:
		return HandleAction(ctx, step, s)
	case BalancerStepType:
		return HandleBalancer(ctx, step, s)
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
	case LinkStepType:
		return HandleLink(ctx, step, s)
	case UpdateStepType:
		return HandleUpdate(ctx, step, s)
	}

	return store.JourneyUserState{}, nil, errors.New("unsupported step type")
}

func Now() *time.Time {
	t := time.Now()
	return &t
}

func WithStateData[T any](state store.JourneyUserState, val T) (_ store.JourneyUserState, err error) {
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

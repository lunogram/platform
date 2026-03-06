package consumer

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store/subjects"
	actiontypes "github.com/lunogram/platform/pkg/modules/actions"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// ActionSchemasHandler creates a handler that extracts and stores action result schema information.
func ActionSchemasHandler(logger *zap.Logger, usrs *subjects.State) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		event := schemas.ActionSchema{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal action schema message", zap.Error(err))
			return err
		}

		logger.Info("incoming action schema", zap.Stringer("action_id", event.ActionID), zap.Stringer("project_id", event.ProjectID))

		paths := rules.ParsePaths(event.Metadata)
		err = usrs.UpsertActionSchema(ctx, event.ActionID, paths)
		if err != nil {
			logger.Error("failed to upsert action schema", zap.Error(err))
			return err
		}

		logger.Info("action schema processed successfully", zap.Stringer("action_id", event.ActionID))
		return nil
	}
}

// ActionExecuteHandler creates a handler that processes action execution requests
// using NATS core request/reply. JetStream consumers cannot reply to NATS core
// requests, so this handler is used with a plain NATS subscription.
func ActionExecuteHandler(logger *zap.Logger, actionRegistry *actions.Registry, pub pubsub.Publisher) CallerHandlerFunc {
	return func(ctx context.Context, msg *nats.Msg) {
		logger.Info("received action execution request")

		var req schemas.ExecuteAction
		action := json.Unmarshal(msg.Data, &req)
		if action != nil {
			logger.Error("failed to unmarshal execute request", zap.Error(action))
			respondWithError(msg, logger, "failed to parse request")
			return
		}

		log := logger.With(zap.String("action_type", req.Type), zap.Stringer("project_id", req.ProjectID), zap.Stringer("action_id", req.ActionID))

		module, exists := actionRegistry.Get(req.Type)
		if !exists {
			log.Warn("unknown action type")
			respondWithError(msg, log, "unknown action type: "+req.Type)
			return
		}

		execReq := &actiontypes.ExecuteRequest[json.RawMessage]{}

		execReq.Config, action = actions.MarshalAndRender(req.Config, req.Variables)
		if action != nil {
			log.Error("failed to render config", zap.Error(action))
			respondWithError(msg, log, "failed to render config")
			return
		}

		if req.Payload != nil {
			execReq.Payload, action = actions.MarshalAndRender(req.Payload, req.Variables)
			if action != nil {
				log.Error("failed to render payload", zap.Error(action))
				respondWithError(msg, log, "failed to render payload")
				return
			}
		}

		log.Info("calling WASM execute")

		result, action := module.Execute(ctx, execReq)
		if action != nil {
			log.Error("action execution failed", zap.Error(action))
			respondWithError(msg, log, "action execution failed: "+action.Error())
			return
		}

		log.Info("action execution completed", zap.String("status", result.Status))

		resp := schemas.ExecuteActionResponse{
			Status:     result.Status,
			StatusCode: result.StatusCode,
			Metadata:   result.Metadata,
		}

		data, action := json.Marshal(resp)
		if action != nil {
			log.Error("failed to marshal response", zap.Error(action))
			respondWithError(msg, log, "failed to marshal response")
			return
		}

		if err := msg.Respond(data); err != nil {
			log.Error("failed to send reply", zap.Error(err))
		}

		// Publish metadata to NATS for schema extraction if present and action is saved
		if result.Metadata != nil && req.ActionID != uuid.Nil {
			schemaMsg := schemas.ActionSchema{
				ProjectID: req.ProjectID,
				ActionID:  req.ActionID,
				Metadata:  result.Metadata,
			}
			if pubErr := pub.Publish(ctx, schemas.ActionsSchema(req.ProjectID), schemaMsg); pubErr != nil {
				// Non-fatal: schema extraction failure should not block the response
				log.Warn("failed to publish action schema", zap.Error(pubErr))
			}
		}

		log.Info("action executed successfully", zap.String("status", result.Status))
	}
}

// respondWithError sends an error response back through the NATS reply subject.
func respondWithError(msg *nats.Msg, log *zap.Logger, errMsg string) {
	resp := schemas.ExecuteActionResponse{
		Status: "error",
		Error:  errMsg,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		log.Error("failed to marshal error response", zap.Error(err))
		return
	}

	if err := msg.Respond(data); err != nil {
		log.Error("failed to send error reply", zap.Error(err))
	}
}

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

		logger.Info("incoming action schema", zap.Stringer("action_id", event.ActionID), zap.String("function_id", event.FunctionID), zap.Stringer("project_id", event.ProjectID))

		paths := rules.ParsePaths(event.Metadata)
		err = usrs.UpsertActionSchema(ctx, event.ActionID, event.FunctionID, paths)
		if err != nil {
			logger.Error("failed to upsert action schema", zap.Error(err))
			return err
		}

		logger.Info("action schema processed successfully", zap.Stringer("action_id", event.ActionID), zap.String("function_id", event.FunctionID))
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

		log := logger.With(zap.String("action_type", req.Type), zap.String("function_id", req.FunctionID), zap.Stringer("project_id", req.ProjectID), zap.Stringer("action_id", req.ActionID))

		module, exists := actionRegistry.Get(req.Type)
		if !exists {
			log.Warn("unknown action type")
			respondWithError(msg, log, "unknown action type: "+req.Type)
			return
		}

		execReq := &actiontypes.ExecuteRequest[json.RawMessage]{}

		var err error
		execReq.Config, err = json.Marshal(req.Config)
		if err != nil {
			log.Error("failed to marshal config", zap.Error(err))
			respondWithError(msg, log, "failed to marshal config")
			return
		}

		if req.Input != nil {
			execReq.Input, err = json.Marshal(req.Input)
			if err != nil {
				log.Error("failed to marshal input", zap.Error(err))
				respondWithError(msg, log, "failed to marshal input")
				return
			}
		}

		log.Info("calling WASM execute")

		result, execErr := module.Execute(ctx, req.FunctionID, execReq)
		if execErr != nil {
			log.Error("action execution failed", zap.Error(execErr))
			respondWithError(msg, log, "action execution failed: "+execErr.Error())
			return
		}

		log.Info("action execution completed", zap.Int("status_code", result.StatusCode))

		resp := schemas.ExecuteActionResponse{
			StatusCode: result.StatusCode,
			Metadata:   result.Metadata,
		}

		data, marshalErr := json.Marshal(resp)
		if marshalErr != nil {
			log.Error("failed to marshal response", zap.Error(marshalErr))
			respondWithError(msg, log, "failed to marshal response")
			return
		}

		if err := msg.Respond(data); err != nil {
			log.Error("failed to send reply", zap.Error(err))
		}

		// Publish metadata to NATS for schema extraction if present and action is saved
		if result.Metadata != nil && req.ActionID != uuid.Nil {
			schemaMsg := schemas.ActionSchema{
				ProjectID:  req.ProjectID,
				ActionID:   req.ActionID,
				FunctionID: req.FunctionID,
				Metadata:   result.Metadata,
			}
			if pubErr := pub.Publish(ctx, schemas.ActionsSchema(req.ProjectID), schemaMsg); pubErr != nil {
				// Non-fatal: schema extraction failure should not block the response
				log.Warn("failed to publish action schema", zap.Error(pubErr))
			}
		}

		log.Info("action executed successfully", zap.Int("status_code", result.StatusCode))
	}
}

// ActionValidateHandler creates a handler that processes action config validation
// requests using NATS core request/reply.
func ActionValidateHandler(logger *zap.Logger, actionRegistry *actions.Registry) CallerHandlerFunc {
	return func(ctx context.Context, msg *nats.Msg) {
		logger.Info("received action validation request")

		var req schemas.ValidateAction
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			logger.Error("failed to unmarshal validate request", zap.Error(err))
			respondWithValidateError(msg, logger, "failed to parse request")
			return
		}

		log := logger.With(zap.String("action_type", req.Type), zap.Stringer("project_id", req.ProjectID))

		module, exists := actionRegistry.Get(req.Type)
		if !exists {
			log.Warn("unknown action type")
			respondWithValidateError(msg, log, "unknown action type: "+req.Type)
			return
		}

		validateReq := &actiontypes.ValidateRequest[json.RawMessage]{}

		var err error
		validateReq.Config, err = json.Marshal(req.Config)
		if err != nil {
			log.Error("failed to marshal config", zap.Error(err))
			respondWithValidateError(msg, log, "failed to marshal config")
			return
		}

		log.Info("calling WASM validate")

		result, err := module.Validate(ctx, validateReq)
		if err != nil {
			log.Error("action validation failed", zap.Error(err))
			respondWithValidateError(msg, log, "action validation failed: "+err.Error())
			return
		}

		log.Info("action validation completed", zap.Int("status_code", result.StatusCode))

		resp := schemas.ValidateActionResponse{
			StatusCode: result.StatusCode,
			Message:    result.Message,
		}

		data, err := json.Marshal(resp)
		if err != nil {
			log.Error("failed to marshal response", zap.Error(err))
			respondWithValidateError(msg, log, "failed to marshal response")
			return
		}

		if err := msg.Respond(data); err != nil {
			log.Error("failed to send reply", zap.Error(err))
		}

		log.Info("action validated successfully", zap.Int("status_code", result.StatusCode))
	}
}

// respondWithValidateError sends a validation error response back through the NATS reply subject.
func respondWithValidateError(msg *nats.Msg, log *zap.Logger, errMsg string) {
	resp := schemas.ValidateActionResponse{
		StatusCode: 500,
		Message:    errMsg,
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

// respondWithError sends an error response back through the NATS reply subject.
func respondWithError(msg *nats.Msg, log *zap.Logger, errMsg string) {
	resp := schemas.ExecuteActionResponse{
		StatusCode: 500,
		Error:      errMsg,
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

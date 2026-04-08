package v1

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func (srv *ClientController) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("devices", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.DeviceRegistration
	if err := json.Decode(r.Body, &req); err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.String("device_id", req.DeviceId),
	)
	logger.Info("registering device")

	userID, err := srv.users.LookupUserID(ctx, projectID, oapi.ToParams(req.Identifier))
	if errors.Is(err, subjects.ErrUserNotFound) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	var osStr *string
	if req.Os != nil {
		osVal := string(*req.Os)
		osStr = &osVal
	}

	device := subjects.Device{
		ProjectID:  projectID,
		UserID:     userID,
		DeviceID:   req.DeviceId,
		OS:         osStr,
		OSVersion:  req.OsVersion,
		Model:      req.Model,
		AppVersion: req.AppVersion,
	}

	pc := subjects.PushConfig{
		Type: subjects.PushConfigType(req.PushConfig.Type),
	}
	switch pc.Type {
	case subjects.PushConfigTypeFCM, subjects.PushConfigTypeAPNs:
		if req.PushConfig.Token == nil || *req.PushConfig.Token == "" {
			logger.Error("push_config.token is required for fcm/apns")
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("push_config.token is required for fcm/apns")))
			return
		}
		pc.Token = *req.PushConfig.Token
	case subjects.PushConfigTypeWebPush:
		if req.PushConfig.Endpoint == nil || *req.PushConfig.Endpoint == "" {
			logger.Error("push_config.endpoint is required for webpush")
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("push_config.endpoint is required for webpush")))
			return
		}
		pc.Endpoint = *req.PushConfig.Endpoint
		pc.ExpirationTime = req.PushConfig.ExpirationTime
		if req.PushConfig.Keys != nil {
			pc.Keys = &subjects.PushConfigKeys{
				Auth:   req.PushConfig.Keys.Auth,
				P256dh: req.PushConfig.Keys.P256dh,
			}
		}
	default:
		logger.Error("invalid push_config.type", zap.String("type", string(req.PushConfig.Type)))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("push_config.type must be fcm, apns, or webpush")))
		return
	}
	device.PushConfig = &pc

	if err := srv.users.UpsertDevice(ctx, device); err != nil {
		logger.Error("failed to upsert device", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("device registered successfully")
	w.WriteHeader(http.StatusCreated)
}

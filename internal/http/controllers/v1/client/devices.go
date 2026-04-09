package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

type DevicesController struct {
	*ClientController
}

func NewDevicesController(client *ClientController) *DevicesController {
	return &DevicesController{ClientController: client}
}

func (srv *DevicesController) GetVapidPublicKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", actor.ProjectID))
	logger.Info("retrieving VAPID public key")

	key, err := srv.mgmt.VapidKeysStore.GetVapidKeyByName(ctx, management.DefaultVapidKeyName)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("VAPID key not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("vapid key not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get VAPID key", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("VAPID public key retrieved successfully")
	json.Write(w, http.StatusOK, oapi.VapidPublicKey{PublicKey: key.PublicKey})
}

func (srv *DevicesController) RegisterDevice(w http.ResponseWriter, r *http.Request) {
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
	if req.Os == nil {
		logger.Error("os is required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("os is required")))
		return
	}

	inferredType, ok := pushConfigTypeFromOS(*req.Os)
	if !ok {
		logger.Error("invalid os", zap.String("os", string(*req.Os)))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("os must be web, ios, or android")))
		return
	}

	if subjects.PushConfigType(req.PushConfig.Type) != inferredType {
		logger.Error(
			"push_config.type does not match os",
			zap.String("os", string(*req.Os)),
			zap.String("push_config_type", string(req.PushConfig.Type)),
			zap.String("expected_type", string(inferredType)),
		)
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("push_config.type does not match os")))
		return
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

	pc := subjects.PushConfig{Type: inferredType}
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
		if req.PushConfig.Keys == nil || req.PushConfig.Keys.Auth == "" || req.PushConfig.Keys.P256dh == "" {
			logger.Error("push_config.keys.auth and push_config.keys.p256dh are required for webpush")
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("push_config.keys.auth and push_config.keys.p256dh are required for webpush")))
			return
		}
		pc.Endpoint = *req.PushConfig.Endpoint
		pc.ExpirationTime = req.PushConfig.ExpirationTime
		pc.Keys = &subjects.PushConfigKeys{
			Auth:   req.PushConfig.Keys.Auth,
			P256dh: req.PushConfig.Keys.P256dh,
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

func pushConfigTypeFromOS(os oapi.DeviceRegistrationOs) (subjects.PushConfigType, bool) {
	switch os {
	case oapi.Android:
		return subjects.PushConfigTypeFCM, true
	case oapi.Ios:
		return subjects.PushConfigTypeAPNs, true
	case oapi.Web:
		return subjects.PushConfigTypeWebPush, true
	default:
		return "", false
	}
}

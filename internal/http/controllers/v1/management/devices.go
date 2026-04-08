package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewDevicesController(logger *zap.Logger, managementDB, usersDB *sqlx.DB, engine *rbac.Engine) *DevicesController {
	return &DevicesController{
		logger:       logger,
		managementDB: managementDB,
		usersDB:      usersDB,
		engine:       engine,
	}
}

type DevicesController struct {
	logger       *zap.Logger
	managementDB *sqlx.DB
	usersDB      *sqlx.DB
	engine       *rbac.Engine
}

func (srv *DevicesController) RegisterDevice(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	var req oapi.DeviceRegistration
	err := json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.String("device_id", req.DeviceId),
	)
	logger.Info("registering device")

	devicesStore := subjects.NewDevicesStore(srv.usersDB)
	usersStore := subjects.NewUsersStore(srv.usersDB)

	var userID uuid.UUID
	if req.ExternalId != nil && *req.ExternalId != "" {
		var err error
		userID, err = usersStore.LookupUserID(ctx, projectID, req.ExternalId, nil)
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn("user not found for external_id, creating device without user association")
		} else if err != nil {
			logger.Error("failed to lookup user by external_id", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	} else if req.AnonymousId != nil && *req.AnonymousId != "" {
		var err error
		userID, err = usersStore.LookupUserID(ctx, projectID, nil, req.AnonymousId)
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn("user not found for anonymous_id, creating device without user association")
		} else if err != nil {
			logger.Error("failed to lookup user by anonymous_id", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	if userID == uuid.Nil {
		logger.Info("no user association for device")
	}

	var osStr *string
	if req.Os != nil {
		osVal := string(*req.Os)
		osStr = &osVal
	}

	// Use userID resolved from external_id/anonymous_id lookup, or fall back to req.UserId
	userId := userID
	if userId == uuid.Nil {
		if req.UserId == nil || *req.UserId == "" {
			logger.Error("no user ID provided")
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("user ID is required")))
			return
		}
		var err error
		userId, err = uuid.Parse(*req.UserId)
		if err != nil {
			logger.Error("failed to parse user ID", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid user ID")))
			return
		}
	}

	device := subjects.Device{
		ProjectID:  projectID,
		UserID:     userId,
		DeviceID:   req.DeviceId,
		OS:         osStr,
		OSVersion:  req.OsVersion,
		Model:      req.Model,
		AppVersion: req.AppVersion,
	}

	// Map push_config from request to internal type
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

	err = devicesStore.UpsertDevice(ctx, device)
	if err != nil {
		logger.Info("Device: ", zap.Any("device", device))
		logger.Error("failed to upsert device", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("device registered successfully")
	w.WriteHeader(http.StatusCreated)
}

func (srv *DevicesController) GetVapidPublicKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	srv.logger.Info("retrieving VAPID public key")

	vapidStore := management.NewVapidKeysStore(srv.managementDB)
	key, err := vapidStore.GetVapidKeyByName("default")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			srv.logger.Warn("no VAPID key found, creating one")
			err = vapidStore.CreateVapidKeysIfNotExist()
			if err != nil {
				srv.logger.Error("failed to create VAPID keys", zap.Error(err))
				oapi.WriteProblem(w, problem.ErrInternal())
				return
			}
			key, err = vapidStore.GetVapidKeyByName("default")
			if err != nil {
				srv.logger.Error("failed to get VAPID key after creation", zap.Error(err))
				oapi.WriteProblem(w, problem.ErrInternal())
				return
			}
		} else {
			srv.logger.Error("failed to get VAPID key", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	response := struct {
		PublicKey string `json:"public_key"`
	}{
		PublicKey: key.PublicKey,
	}

	srv.logger.Info("VAPID public key retrieved successfully")
	json.Write(w, http.StatusOK, response)
}

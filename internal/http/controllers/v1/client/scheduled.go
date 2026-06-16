package v1

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

type ScheduledController struct {
	*ClientController
}

func NewScheduledController(client *ClientController) *ScheduledController {
	return &ScheduledController{ClientController: client}
}

func (srv *ScheduledController) UpsertUserScheduledClient(w http.ResponseWriter, r *http.Request) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "scheduled", rbac.Create)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	var req oapi.UpsertUserScheduledRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	if req.Identifier == nil || len(*req.Identifier) == 0 {
		srv.logger.Error("at least one identifier is required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("at least one identifier is required")))
		return
	}

	scheduleType := "single"
	if req.Interval != nil {
		scheduleType = "recurring"
		if !srv.users.ValidateInterval(ctx, *req.Interval) {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid interval")))
			return
		}
	}

	if scheduleType == "recurring" && req.StartAt == nil {
		now := time.Now().UTC()
		req.StartAt = &now
	}

	if scheduleType == "single" && req.ScheduledAt == nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled_at is required for single schedules")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("scheduled_name", req.Name))
	logger.Info("upserting user scheduled", zap.String("type", scheduleType))

	var data map[string]any
	if req.Data != nil {
		data = *req.Data
	}

	userIDParams := oapi.ToParams(*req.Identifier)
	msg := schemas.ScheduledMsg{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        req.Name,
		Type:        scheduleType,
		SubjectType: "user",
		Data:        data,
		Identifiers: userIDParams,
		StartAt:     req.StartAt,
		Interval:    req.Interval,
	}

	if req.ScheduledAt != nil {
		msg.ScheduledAt = *req.ScheduledAt
	}

	err = srv.pubsub.Publish(ctx, schemas.ScheduledProcess(projectID), msg)
	if err != nil {
		logger.Error("failed to publish user scheduled", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("user scheduled accepted for processing", zap.Stringer("id", msg.ID))

	var scheduledAt time.Time
	if req.ScheduledAt != nil {
		scheduledAt = *req.ScheduledAt
	} else if req.StartAt != nil {
		scheduledAt = *req.StartAt
	}

	json.Write(w, http.StatusAccepted, oapi.ScheduledAccepted{
		Id:          msg.ID,
		Name:        req.Name,
		ScheduledAt: scheduledAt,
		Data:        req.Data,
	})
}

func (srv *ScheduledController) DeleteUserScheduledClient(w http.ResponseWriter, r *http.Request) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "scheduled", rbac.Delete)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	var req oapi.DeleteUserScheduledRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	if req.Identifier == nil || len(*req.Identifier) == 0 {
		srv.logger.Error("at least one identifier is required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("at least one identifier is required")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("scheduled_name", req.Name))
	logger.Info("deleting user scheduled")

	userID, err := srv.users.LookupUserID(ctx, projectID, oapi.ToParams(*req.Identifier))
	if errors.Is(err, subjects.ErrUserNotFound) {
		logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	schedule, err := srv.users.GetScheduleByName(ctx, projectID, req.Name)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("schedule not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("schedule not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get schedule by name", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = srv.users.DeleteUserScheduleByScheduleID(ctx, userID, schedule.ID)
	if err != nil {
		logger.Error("failed to delete user schedule", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("user scheduled deleted")
	w.WriteHeader(http.StatusOK)
}

func (srv *ScheduledController) UpsertOrganizationScheduledClient(w http.ResponseWriter, r *http.Request) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "scheduled", rbac.Create)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	var req oapi.UpsertOrganizationScheduledRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("scheduled_name", req.Name))
	logger.Info("upserting organization scheduled")

	orgIdentifiers := oapi.ToParams(req.Identifier)
	orgID, err := srv.users.LookupOrganizationID(ctx, projectID, orgIdentifiers)
	if errors.Is(err, subjects.ErrOrgNotFound) {
		logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	scheduleType := "single"
	if req.Interval != nil {
		scheduleType = "recurring"
		if !srv.users.ValidateInterval(ctx, *req.Interval) {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid interval")))
			return
		}
	}

	if scheduleType == "recurring" && req.StartAt == nil {
		now := time.Now().UTC()
		req.StartAt = &now
	}

	if scheduleType == "single" && req.ScheduledAt == nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled_at is required for single schedules")))
		return
	}

	var data map[string]any
	if req.Data != nil {
		data = *req.Data
	}

	msg := schemas.ScheduledMsg{
		ID:             uuid.New(),
		ProjectID:      projectID,
		Name:           req.Name,
		Type:           scheduleType,
		SubjectType:    "organization",
		Data:           data,
		OrganizationID: orgID,
		Identifiers:    orgIdentifiers,
		StartAt:        req.StartAt,
		Interval:       req.Interval,
	}

	if req.ScheduledAt != nil {
		msg.ScheduledAt = *req.ScheduledAt
	}

	err = srv.pubsub.Publish(ctx, schemas.ScheduledProcess(projectID), msg)
	if err != nil {
		logger.Error("failed to publish organization scheduled", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("organization scheduled accepted for processing", zap.Stringer("id", msg.ID))

	var scheduledAt time.Time
	if req.ScheduledAt != nil {
		scheduledAt = *req.ScheduledAt
	} else if req.StartAt != nil {
		scheduledAt = *req.StartAt
	}

	json.Write(w, http.StatusAccepted, oapi.ScheduledAccepted{
		Id:          msg.ID,
		Name:        req.Name,
		ScheduledAt: scheduledAt,
		Data:        req.Data,
	})
}

func (srv *ScheduledController) DeleteOrganizationScheduledClient(w http.ResponseWriter, r *http.Request) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "scheduled", rbac.Delete)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	var req oapi.DeleteOrganizationScheduledRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("scheduled_name", req.Name))
	logger.Info("deleting organization scheduled")

	orgID, err := srv.users.LookupOrganizationID(ctx, projectID, oapi.ToParams(req.Identifier))
	if errors.Is(err, subjects.ErrOrgNotFound) {
		logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	schedule, err := srv.users.GetScheduleByName(ctx, projectID, req.Name)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("schedule not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("schedule not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get schedule by name", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = srv.users.DeleteOrganizationScheduleByScheduleID(ctx, orgID, schedule.ID)
	if err != nil {
		logger.Error("failed to delete organization schedule", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("organization scheduled deleted")
	w.WriteHeader(http.StatusOK)
}

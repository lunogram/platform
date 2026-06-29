package v1

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewScheduledController(logger *zap.Logger, store *subjects.State, pub pubsub.Publisher, engine *rbac.Engine) *ScheduledController {
	return &ScheduledController{
		logger: logger,
		store:  store,
		pub:    pub,
		engine: engine,
	}
}

type ScheduledController struct {
	logger *zap.Logger
	store  *subjects.State
	pub    pubsub.Publisher
	engine *rbac.Engine
}

// ListScheduledSchemas lists all scheduled definitions with schemas for a project.
// GET /api/admin/projects/{projectID}/subjects/user/scheduled/schema
func (srv *ScheduledController) ListScheduledSchemas(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		srv.logger.Error("failed to authorize scheduled schema list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("listing scheduled schemas")

	scheduled, err := srv.store.ListSchedules(ctx, projectID)
	if err != nil {
		logger.Error("failed to list schedules", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("schedules listed", zap.Int("count", len(scheduled)))

	results := make([]oapi.ScheduledEventWithSchema, len(scheduled))
	for i, s := range scheduled {
		schema := make([]oapi.SchemaPath, len(s.Schema))
		for j, sp := range s.Schema {
			path := sp.Path
			if path != ".data" && !strings.HasPrefix(path, ".data.") && !strings.HasPrefix(path, ".data[") {
				path = ".data" + path
			}
			schema[j] = oapi.SchemaPath{
				Path:  path,
				Types: []string(sp.Types),
			}
		}

		// Fetch offsets for this schedule
		storeOffsets, err := srv.store.ListScheduleOffsets(ctx, s.ID)
		if err != nil {
			logger.Error("failed to list schedule offsets", zap.Error(err), zap.String("schedule_id", s.ID.String()))
			oapi.WriteProblem(w, err)
			return
		}

		offsets := make([]oapi.ScheduleOffset, len(storeOffsets))
		for j, o := range storeOffsets {
			offsets[j] = oapi.ScheduleOffset{
				Id:         o.ID,
				ScheduleId: o.ScheduleID,
				Offset:     o.Offset,
				Direction:  oapi.ScheduleOffsetDirection(o.Direction),
				CreatedAt:  o.CreatedAt,
				UpdatedAt:  o.UpdatedAt,
			}
		}

		results[i] = oapi.ScheduledEventWithSchema{
			Id:      s.ID,
			Name:    s.Name,
			Schema:  schema,
			Offsets: offsets,
		}
	}

	json.Write(w, http.StatusOK, oapi.ScheduledEventListResponse{
		Results: results,
	})
}

// DeleteScheduledSchema soft-deletes a scheduled definition by project and scheduled ID.
// DELETE /api/admin/projects/{projectID}/subjects/user/scheduled/schema/{scheduledID}
func (srv *ScheduledController) DeleteScheduledSchema(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, scheduledID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		srv.logger.Error("failed to authorize scheduled schema delete", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("scheduled_id", scheduledID.String()),
	)

	logger.Info("deleting scheduled schema")

	err = srv.store.DeleteScheduleByID(ctx, projectID, scheduledID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("scheduled not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("scheduled not found")))
		return
	}

	if err != nil {
		logger.Error("failed to delete scheduled", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("scheduled schema deleted")
	w.WriteHeader(http.StatusOK)
}

// CreateScheduleOffset creates a new offset for a schedule definition.
// POST /api/admin/projects/{projectID}/subjects/user/scheduled/schema/{scheduledID}/offsets
func (srv *ScheduledController) CreateScheduleOffset(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, scheduledID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		srv.logger.Error("failed to authorize schedule offset create", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("scheduled_id", scheduledID.String()),
	)

	// Verify the schedule exists and belongs to this project.
	schedule, err := srv.store.GetScheduleByID(ctx, scheduledID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("schedule not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("schedule not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get schedule", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if schedule.ProjectID != projectID {
		logger.Info("schedule does not belong to project")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("schedule not found")))
		return
	}

	var body oapi.CreateScheduleOffsetJSONRequestBody
	err = json.Decode(r.Body, &body)
	if err != nil {
		logger.Error("failed to decode create schedule offset request body", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("creating schedule offset", zap.String("offset", body.Offset), zap.String("direction", string(body.Direction)))

	offset, err := srv.store.CreateScheduleOffset(ctx, scheduledID, body.Offset, string(body.Direction))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			logger.Info("offset already exists", zap.String("offset", body.Offset))
			oapi.WriteProblem(w, problem.ErrConflict(problem.Describe("an offset with this value already exists for this schedule")))
			return
		}
		logger.Error("failed to create schedule offset", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("schedule offset created", zap.String("offset_id", offset.ID.String()))

	// Publish a backfill message so that a NATS consumer asynchronously
	// creates scheduled events for all existing user/org schedules that
	// reference this schedule at the new offset.
	err = srv.pub.Publish(ctx, schemas.ScheduledBackfill(projectID), schemas.ScheduleOffsetBackfillMsg{
		ProjectID:  projectID,
		ScheduleID: scheduledID,
		OffsetID:   offset.ID,
		Offset:     offset.Offset,
		Direction:  offset.Direction,
	})
	if err != nil {
		logger.Error("failed to publish schedule offset backfill message", zap.Error(err))
		// The offset was created successfully; log the publish failure but
		// don't fail the HTTP request. The backfill can be retried manually.
	}

	json.Write(w, http.StatusCreated, oapi.ScheduleOffset{
		Id:         offset.ID,
		ScheduleId: offset.ScheduleID,
		Offset:     offset.Offset,
		Direction:  oapi.ScheduleOffsetDirection(offset.Direction),
		CreatedAt:  offset.CreatedAt,
		UpdatedAt:  offset.UpdatedAt,
	})
}

// GetUserScheduled lists scheduled instances for a specific user.
// GET /api/admin/projects/{projectID}/subjects/users/{userID}/scheduled
func (srv *ScheduledController) GetUserScheduled(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID, params oapi.GetUserScheduledParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		srv.logger.Error("failed to authorize user scheduled read", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.store.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing user scheduled", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	items, total, err := srv.store.ListUserSchedules(ctx, projectID, userID, pagination)
	if err != nil {
		logger.Error("failed to list user schedules", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user schedules listed", zap.Int("total", total), zap.Int("count", len(items)))

	results := make([]oapi.UserScheduled, len(items))
	for i, item := range items {
		results[i] = userScheduleToOAPI(item)
	}

	json.Write(w, http.StatusOK, oapi.UserScheduledList{
		Results: results,
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	})
}

// UpsertUserScheduled creates or updates a scheduled instance for a specific user.
// PUT /api/admin/projects/{projectID}/subjects/users/{userID}/scheduled
func (srv *ScheduledController) UpsertUserScheduled(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		srv.logger.Error("failed to authorize user scheduled create", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.store.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.UpsertUserScheduledJSONRequestBody
	err = json.Decode(r.Body, &body)
	if err != nil {
		srv.logger.Error("failed to decode user scheduled request body", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Resolve the schedule ID: either from scheduled_id directly, or by
	// upserting a schedule definition from scheduled_name (same approach as the client API).
	var scheduleID uuid.UUID
	switch {
	case body.ScheduledName != nil && *body.ScheduledName != "":
		scheduleType := "single"
		if body.Interval != nil {
			scheduleType = "recurring"
		}
		scheduleID, err = srv.store.UpsertSchedule(ctx, projectID, *body.ScheduledName, scheduleType)
		if err != nil {
			srv.logger.Error("failed to upsert schedule definition", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	case body.ScheduledId != nil:
		scheduleID = *body.ScheduledId
	default:
		srv.logger.Error("either scheduled_id or scheduled_name is required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("either scheduled_id or scheduled_name is required")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
		zap.String("scheduled_id", scheduleID.String()),
	)

	logger.Info("upserting user scheduled")

	var data json.RawMessage
	if body.Data != nil {
		data = json.RawMessage(*body.Data)
	}

	// For single schedules (no interval), scheduled_at is required.
	if body.Interval == nil && body.ScheduledAt == nil {
		srv.logger.Error("scheduled_at is required for single schedules")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled_at is required for single schedules")))
		return
	}

	// For recurring schedules, default start_at to now if not provided.
	// This ensures the scheduler has a valid anchor for computing occurrences.
	if body.Interval != nil && body.StartAt == nil {
		now := time.Now().UTC()
		body.StartAt = &now
	}

	// A supplied id updates that specific assignment; omitting it creates a new
	// one (the store mints an id). Multiple assignments may share the same name.
	var assignmentID uuid.UUID
	if body.Id != nil {
		assignmentID = *body.Id
	}

	upserted, err := srv.store.UpsertUserSchedule(ctx, assignmentID, userID, scheduleID, body.ScheduledAt, body.StartAt, body.Interval, data)
	if errors.Is(err, subjects.ErrScheduleOwnershipMismatch) {
		logger.Warn("scheduled instance id does not match user or schedule", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled instance id does not match the user or schedule")))
		return
	}
	if err != nil {
		logger.Error("failed to upsert user schedule", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user schedule upserted", zap.String("instance_id", upserted.ID.String()))

	json.Write(w, http.StatusOK, userScheduleToOAPI(*upserted))
}

// DeleteUserScheduled deletes a specific scheduled instance for a user.
// DELETE /api/admin/projects/{projectID}/subjects/users/{userID}/scheduled/{scheduledInstanceID}
func (srv *ScheduledController) DeleteUserScheduled(w http.ResponseWriter, r *http.Request, projectID, userID, scheduledInstanceID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		srv.logger.Error("failed to authorize user scheduled delete", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
		zap.String("scheduled_instance_id", scheduledInstanceID.String()),
	)

	logger.Info("deleting user scheduled instance")

	// Verify the instance belongs to the specified user before deleting.
	existing, err := srv.store.GetUserScheduleByID(ctx, scheduledInstanceID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("user scheduled instance not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("scheduled instance not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get user schedule", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if existing.UserID != userID {
		logger.Warn("user scheduled instance does not belong to specified user",
			zap.String("actual_user_id", existing.UserID.String()))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("scheduled instance not found")))
		return
	}

	err = srv.store.DeleteUserSchedule(ctx, scheduledInstanceID)
	if err != nil {
		logger.Error("failed to delete user schedule", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user scheduled instance deleted")
	w.WriteHeader(http.StatusNoContent)
}

// UpdateUserScheduled updates the scheduled_at for a specific scheduled instance for a user.
// PATCH /api/admin/projects/{projectID}/subjects/users/{userID}/scheduled/{scheduledInstanceID}
func (srv *ScheduledController) UpdateUserScheduled(w http.ResponseWriter, r *http.Request, projectID, userID, scheduledInstanceID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		srv.logger.Error("failed to authorize user scheduled update", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.UpdateUserScheduledRequest
	err = json.Decode(r.Body, &body)
	if err != nil {
		srv.logger.Error("failed to decode update user scheduled request body", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
		zap.String("scheduled_instance_id", scheduledInstanceID.String()),
	)

	logger.Info("updating user scheduled instance")

	// Handle pause/resume actions
	if body.Pause != nil && body.Resume != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("cannot pause and resume at the same time")))
		return
	}

	if body.Pause != nil {
		deleteEvents := *body.Pause == oapi.UpdateUserScheduledRequestPauseImmediately
		updated, err := srv.store.PauseUserSchedule(ctx, scheduledInstanceID, deleteEvents)
		if err != nil {
			logger.Error("failed to pause user schedule", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		logger.Info("user scheduled instance paused")
		json.Write(w, http.StatusOK, userScheduleToOAPI(*updated))
		return
	}

	if body.Resume != nil {
		recalculate := *body.Resume == oapi.UpdateUserScheduledRequestResumeImmediately
		updated, err := srv.store.ResumeUserSchedule(ctx, scheduledInstanceID, recalculate)
		if err != nil {
			logger.Error("failed to resume user schedule", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		logger.Info("user scheduled instance resumed")
		json.Write(w, http.StatusOK, userScheduleToOAPI(*updated))
		return
	}

	if body.ScheduledAt == nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled_at is required when not pausing or resuming")))
		return
	}

	scheduledAt := *body.ScheduledAt
	updated, err := srv.store.UpdateUserSchedule(ctx, scheduledInstanceID, &scheduledAt, nil, nil, nil)
	if err != nil {
		logger.Error("failed to update user schedule", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user scheduled instance updated")
	json.Write(w, http.StatusOK, userScheduleToOAPI(*updated))
}

// GetOrganizationScheduled lists scheduled instances for a specific organization.
// GET /api/admin/projects/{projectID}/subjects/organizations/{organizationID}/scheduled
func (srv *ScheduledController) GetOrganizationScheduled(w http.ResponseWriter, r *http.Request, projectID, organizationID uuid.UUID, params oapi.GetOrganizationScheduledParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		srv.logger.Error("failed to authorize organization scheduled read", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.store.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
	)

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing organization scheduled", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	items, total, err := srv.store.ListOrganizationSchedules(ctx, projectID, organizationID, pagination)
	if err != nil {
		logger.Error("failed to list organization schedules", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization schedules listed", zap.Int("total", total), zap.Int("count", len(items)))

	results := make([]oapi.UserScheduled, len(items))
	for i, item := range items {
		results[i] = orgScheduleToOAPI(item)
	}

	json.Write(w, http.StatusOK, oapi.UserScheduledList{
		Results: results,
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	})
}

// UpsertOrganizationScheduled creates or updates a scheduled instance for a specific organization.
// PUT /api/admin/projects/{projectID}/subjects/organizations/{organizationID}/scheduled
func (srv *ScheduledController) UpsertOrganizationScheduled(w http.ResponseWriter, r *http.Request, projectID, organizationID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		srv.logger.Error("failed to authorize organization scheduled create", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.store.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.UpsertOrganizationScheduledJSONRequestBody
	err = json.Decode(r.Body, &body)
	if err != nil {
		srv.logger.Error("failed to decode organization scheduled request body", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Resolve the schedule ID: either from scheduled_id directly, or by
	// upserting a schedule definition from scheduled_name (same approach as the client API).
	var scheduleID uuid.UUID
	switch {
	case body.ScheduledName != nil && *body.ScheduledName != "":
		scheduleType := "single"
		if body.Interval != nil {
			scheduleType = "recurring"
		}
		scheduleID, err = srv.store.UpsertSchedule(ctx, projectID, *body.ScheduledName, scheduleType)
		if err != nil {
			srv.logger.Error("failed to upsert schedule definition", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	case body.ScheduledId != nil:
		scheduleID = *body.ScheduledId
	default:
		srv.logger.Error("either scheduled_id or scheduled_name is required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("either scheduled_id or scheduled_name is required")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
		zap.String("scheduled_id", scheduleID.String()),
	)

	logger.Info("upserting organization scheduled")

	var data json.RawMessage
	if body.Data != nil {
		data = json.RawMessage(*body.Data)
	}

	// For single schedules (no interval), scheduled_at is required.
	if body.Interval == nil && body.ScheduledAt == nil {
		srv.logger.Error("scheduled_at is required for single schedules")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled_at is required for single schedules")))
		return
	}

	// For recurring schedules, default start_at to now if not provided.
	// This ensures the scheduler has a valid anchor for computing occurrences.
	if body.Interval != nil && body.StartAt == nil {
		now := time.Now().UTC()
		body.StartAt = &now
	}

	// A supplied id updates that specific assignment; omitting it creates a new
	// one (the store mints an id). Multiple assignments may share the same name.
	var assignmentID uuid.UUID
	if body.Id != nil {
		assignmentID = *body.Id
	}

	upserted, err := srv.store.UpsertOrganizationSchedule(ctx, assignmentID, organizationID, scheduleID, body.ScheduledAt, body.StartAt, body.Interval, data)
	if errors.Is(err, subjects.ErrScheduleOwnershipMismatch) {
		logger.Warn("scheduled instance id does not match organization or schedule", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled instance id does not match the organization or schedule")))
		return
	}
	if err != nil {
		logger.Error("failed to upsert organization schedule", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization schedule upserted", zap.String("instance_id", upserted.ID.String()))

	json.Write(w, http.StatusOK, orgScheduleToOAPI(*upserted))
}

// DeleteOrganizationScheduled deletes a specific scheduled instance for an organization.
// DELETE /api/admin/projects/{projectID}/subjects/organizations/{organizationID}/scheduled/{scheduledInstanceID}
func (srv *ScheduledController) DeleteOrganizationScheduled(w http.ResponseWriter, r *http.Request, projectID, organizationID, scheduledInstanceID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		srv.logger.Error("failed to authorize organization scheduled delete", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
		zap.String("scheduled_instance_id", scheduledInstanceID.String()),
	)

	logger.Info("deleting organization scheduled instance")

	// Verify the instance belongs to the specified organization before deleting.
	existing, err := srv.store.GetOrganizationScheduleByID(ctx, scheduledInstanceID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("organization scheduled instance not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("scheduled instance not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get organization schedule", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if existing.OrganizationID != organizationID {
		logger.Warn("organization scheduled instance does not belong to specified organization",
			zap.String("actual_organization_id", existing.OrganizationID.String()))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("scheduled instance not found")))
		return
	}

	err = srv.store.DeleteOrganizationSchedule(ctx, scheduledInstanceID)
	if err != nil {
		logger.Error("failed to delete organization schedule", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization scheduled instance deleted")
	w.WriteHeader(http.StatusNoContent)
}

// UpdateOrganizationScheduled updates the scheduled_at for a specific scheduled instance for an organization.
// PATCH /api/admin/projects/{projectID}/subjects/organizations/{organizationID}/scheduled/{scheduledInstanceID}
func (srv *ScheduledController) UpdateOrganizationScheduled(w http.ResponseWriter, r *http.Request, projectID, organizationID, scheduledInstanceID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		srv.logger.Error("failed to authorize organization scheduled update", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.UpdateOrganizationScheduledRequest
	err = json.Decode(r.Body, &body)
	if err != nil {
		srv.logger.Error("failed to decode update organization scheduled request body", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
		zap.String("scheduled_instance_id", scheduledInstanceID.String()),
	)

	logger.Info("updating organization scheduled instance")

	// Handle pause/resume actions
	if body.Pause != nil && body.Resume != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("cannot pause and resume at the same time")))
		return
	}

	if body.Pause != nil {
		deleteEvents := *body.Pause == oapi.UpdateOrganizationScheduledRequestPauseImmediately
		updated, err := srv.store.PauseOrganizationSchedule(ctx, scheduledInstanceID, deleteEvents)
		if err != nil {
			logger.Error("failed to pause organization schedule", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		logger.Info("organization scheduled instance paused")
		json.Write(w, http.StatusOK, orgScheduleToOAPI(*updated))
		return
	}

	if body.Resume != nil {
		recalculate := *body.Resume == oapi.UpdateOrganizationScheduledRequestResumeImmediately
		updated, err := srv.store.ResumeOrganizationSchedule(ctx, scheduledInstanceID, recalculate)
		if err != nil {
			logger.Error("failed to resume organization schedule", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		logger.Info("organization scheduled instance resumed")
		json.Write(w, http.StatusOK, orgScheduleToOAPI(*updated))
		return
	}

	if body.ScheduledAt == nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled_at is required when not pausing or resuming")))
		return
	}

	scheduledAt := *body.ScheduledAt
	updated, err := srv.store.UpdateOrganizationSchedule(ctx, scheduledInstanceID, &scheduledAt, nil, nil, nil)
	if err != nil {
		logger.Error("failed to update organization schedule", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization scheduled instance updated")
	json.Write(w, http.StatusOK, orgScheduleToOAPI(*updated))
}

// userScheduleToOAPI converts a store UserSchedule to the OAPI UserScheduled type.
func userScheduleToOAPI(us subjects.UserSchedule) oapi.UserScheduled {
	result := oapi.UserScheduled{
		Id:               us.ID,
		UserId:           us.UserID,
		ScheduledId:      us.ScheduleID,
		Data:             us.Data,
		Interval:         us.Interval,
		StartAt:          us.StartAt,
		AnchorAt:         us.AnchorAt,
		PausedAt:         us.PausedAt,
		CreatedAt:        us.CreatedAt,
		UpdatedAt:        us.UpdatedAt,
		HasPendingEvents: &us.HasPendingEvents,
	}
	if us.ScheduledAt != nil {
		result.ScheduledAt = *us.ScheduledAt
	} else if us.StartAt != nil {
		// For recurring schedules, fall back to start_at so the API
		// never returns the zero time for the required scheduled_at field.
		result.ScheduledAt = *us.StartAt
	}
	return result
}

// orgScheduleToOAPI converts a store OrganizationSchedule to the OAPI UserScheduled type.
// Reuses UserScheduled since the wire format is the same; UserId carries OrganizationID.
func orgScheduleToOAPI(os subjects.OrganizationSchedule) oapi.UserScheduled {
	result := oapi.UserScheduled{
		Id:               os.ID,
		UserId:           os.OrganizationID,
		ScheduledId:      os.ScheduleID,
		Data:             os.Data,
		Interval:         os.Interval,
		StartAt:          os.StartAt,
		AnchorAt:         os.AnchorAt,
		PausedAt:         os.PausedAt,
		CreatedAt:        os.CreatedAt,
		UpdatedAt:        os.UpdatedAt,
		HasPendingEvents: &os.HasPendingEvents,
	}
	if os.ScheduledAt != nil {
		result.ScheduledAt = *os.ScheduledAt
	} else if os.StartAt != nil {
		result.ScheduledAt = *os.StartAt
	}
	return result
}

//go:build enterprise

package v1

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/http/sse"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"

	"go.uber.org/zap"
)

func NewBroadcastsController(logger *zap.Logger, managementDB, usersDB *sqlx.DB, pub pubsub.Publisher, jet jetstream.JetStream, engine *rbac.Engine, namespace consumer.Namespace) *BroadcastsController {
	return &BroadcastsController{
		logger:    logger,
		mgmt:      management.NewState(managementDB),
		usrs:      subjects.NewState(usersDB, logger),
		usersDB:   usersDB,
		pub:       pub,
		jet:       jet,
		engine:    engine,
		namespace: namespace,
	}
}

type BroadcastsController struct {
	logger    *zap.Logger
	mgmt      *management.State
	usrs      *subjects.State
	usersDB   *sqlx.DB
	pub       pubsub.Publisher
	jet       jetstream.JetStream
	engine    *rbac.Engine
	namespace consumer.Namespace
}

func (srv *BroadcastsController) CreateBroadcast(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("broadcasts", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.CreateBroadcastJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	// Validate that scheduled_at, when provided, is in the future.
	if body.ScheduledAt != nil && !body.ScheduledAt.After(time.Now()) {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled_at must be in the future")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", body.CampaignId), zap.Stringer("list_id", body.ListId))
	logger.Info("creating broadcast")

	// Load campaign and verify it has a provider configured
	campaign, err := srv.mgmt.GetCampaign(ctx, projectID, body.CampaignId)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("campaign not found", zap.Stringer("campaign_id", body.CampaignId))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	list, err := srv.usrs.GetList(ctx, projectID, body.ListId)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("list not found", zap.Stringer("list_id", body.ListId))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	broadcast, err := srv.mgmt.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:   projectID,
		CampaignID:  body.CampaignId,
		ListID:      body.ListId,
		ListName:    list.Name,
		ListType:    string(list.Type),
		ScheduledAt: body.ScheduledAt,
	})
	if err != nil {
		logger.Error("failed to create broadcast", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	broadcast.Campaign = &management.Campaign{
		ID:   campaign.ID,
		Name: campaign.Name,
	}

	logger.Info("broadcast created", zap.Stringer("broadcast_id", broadcast.ID))
	json.Write(w, http.StatusCreated, broadcast.OAPI())
}

func (srv *BroadcastsController) ListBroadcasts(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListBroadcastsParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("broadcasts", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Debug("listing broadcasts")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	var state *management.BroadcastState
	if params.State != nil {
		s := management.BroadcastState(*params.State)
		state = &s
	}

	result, total, err := srv.mgmt.ListBroadcasts(ctx, projectID, pagination, params.Search.ToString(), params.CampaignId, params.ListId, state)
	if err != nil {
		logger.Error("failed to list broadcasts", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Sent is populated directly from the persisted column on campaign_broadcasts.

	logger.Debug("listed broadcasts", zap.Int("count", len(result)))
	json.Write(w, http.StatusOK, oapi.BroadcastListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: result.OAPI(),
	})
}

func (srv *BroadcastsController) GetBroadcast(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, broadcastID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("broadcasts", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("broadcast_id", broadcastID))
	logger.Debug("getting broadcast")

	broadcast, err := srv.mgmt.GetBroadcast(ctx, projectID, broadcastID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Debug("broadcast not found", zap.Stringer("broadcast_id", broadcastID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("broadcast not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch broadcast", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Debug("broadcast retrieved")
	json.Write(w, http.StatusOK, broadcast.OAPI())
}

func (srv *BroadcastsController) SendBroadcast(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, broadcastID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("broadcasts", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("broadcast_id", broadcastID))
	logger.Info("sending broadcast")

	// Atomically transition from pending → sending to prevent duplicate sends.
	if err := srv.mgmt.BroadcastsStore.TransitionPendingBroadcastToSending(ctx, broadcastID); err != nil {
		logger.Warn("failed to transition broadcast to sending", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("broadcast is not in pending state")))
		return
	}

	// Publish NATS message to trigger broadcast processing
	err = srv.pub.Publish(ctx, schemas.BroadcastsProcess(projectID, broadcastID), schemas.ProcessBroadcast{
		ProjectID:   projectID,
		BroadcastID: broadcastID,
	})
	if err != nil {
		logger.Error("failed to publish broadcast process message", zap.Error(err))
		// Revert state back to pending since NATS publish failed
		if revertErr := srv.mgmt.UpdateBroadcastState(ctx, projectID, broadcastID, management.BroadcastStatePending, 0, nil); revertErr != nil {
			logger.Error("failed to revert broadcast state to pending", zap.Error(revertErr))
		}
		oapi.WriteProblem(w, err)
		return
	}

	// Re-fetch to get the updated state with campaign data
	broadcast, err := srv.mgmt.GetBroadcast(ctx, projectID, broadcastID)
	if err != nil {
		logger.Error("failed to re-fetch broadcast after send", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("broadcast send triggered", zap.Stringer("broadcast_id", broadcastID))
	json.Write(w, http.StatusOK, broadcast.OAPI())
}

func (srv *BroadcastsController) UpdateBroadcast(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, broadcastID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("broadcasts", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.UpdateBroadcastJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	// Validate that scheduled_at, when provided, is in the future.
	if body.ScheduledAt != nil && !body.ScheduledAt.After(time.Now()) {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled_at must be in the future")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("broadcast_id", broadcastID))
	logger.Info("updating broadcast")

	updated, err := srv.mgmt.UpdateBroadcast(ctx, projectID, broadcastID, body.ScheduledAt)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("broadcast not found or not in updatable state", zap.Stringer("broadcast_id", broadcastID))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("broadcast not found or not in an updatable state (must be pending or scheduled)")))
		return
	}

	if err != nil {
		logger.Error("failed to update broadcast", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Re-fetch to get joined campaign data
	broadcast, err := srv.mgmt.GetBroadcast(ctx, projectID, broadcastID)
	if err != nil {
		logger.Error("failed to fetch updated broadcast", zap.Error(err))
		// Return the partial result from the update
		json.Write(w, http.StatusOK, updated.OAPI())
		return
	}

	logger.Info("broadcast updated", zap.Stringer("broadcast_id", broadcastID))
	json.Write(w, http.StatusOK, broadcast.OAPI())
}

func (srv *BroadcastsController) CancelBroadcast(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, broadcastID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("broadcasts", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("broadcast_id", broadcastID))
	logger.Info("cancelling broadcast")

	cancelled, err := srv.mgmt.CancelBroadcast(ctx, projectID, broadcastID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("broadcast not found or not in cancellable state", zap.Stringer("broadcast_id", broadcastID))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("broadcast not found or not in a cancellable state (must be pending or scheduled)")))
		return
	}

	if err != nil {
		logger.Error("failed to cancel broadcast", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Re-fetch to get joined campaign data
	broadcast, err := srv.mgmt.GetBroadcast(ctx, projectID, broadcastID)
	if err != nil {
		logger.Error("failed to fetch cancelled broadcast", zap.Error(err))
		json.Write(w, http.StatusOK, cancelled.OAPI())
		return
	}

	logger.Info("broadcast cancelled", zap.Stringer("broadcast_id", broadcastID))
	json.Write(w, http.StatusOK, broadcast.OAPI())
}

func (srv *BroadcastsController) GetBroadcastUsers(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, broadcastID uuid.UUID, params oapi.GetBroadcastUsersParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("broadcasts", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("broadcast_id", broadcastID))
	logger.Debug("getting broadcast users")

	// Verify broadcast exists and belongs to this project
	_, err = srv.mgmt.GetBroadcast(ctx, projectID, broadcastID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Debug("broadcast not found", zap.Stringer("broadcast_id", broadcastID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("broadcast not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get broadcast", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	users, total, err := srv.mgmt.BroadcastsStore.GetBroadcastUsers(ctx, srv.usersDB, broadcastID, pagination, params.Search.ToString())
	if err != nil {
		logger.Error("failed to get broadcast users", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Debug("broadcast users retrieved", zap.Int("count", len(users)))
	json.Write(w, http.StatusOK, map[string]any{
		"total":   total,
		"limit":   pagination.Limit,
		"offset":  pagination.Offset,
		"results": users,
	})
}

// progressTickInterval is how often the SSE handler queries the database for
// the current sent count and broadcast state.
const progressTickInterval = 2 * time.Second

func (srv *BroadcastsController) StreamBroadcastProgress(
	w http.ResponseWriter,
	r *http.Request,
	projectID uuid.UUID,
	broadcastID uuid.UUID,
) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("broadcasts", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	enc := sse.NewEncoder(ctx, w)
	enc.WriteEvent("message", "connected") //nolint:errcheck
	rc := http.NewResponseController(w)

	err = rc.SetWriteDeadline(time.Time{})
	if err != nil {
		srv.logger.Error("failed to set write deadline", zap.Error(err))
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("broadcast_id", broadcastID),
	)
	logger.Debug("streaming broadcast progress")

	ticker := time.NewTicker(progressTickInterval)
	defer ticker.Stop()

	for {
		broadcast, err := srv.mgmt.GetBroadcast(ctx, projectID, broadcastID)
		if err != nil {
			logger.Error("failed to get broadcast for progress", zap.Error(err))
			enc.WriteEvent("error", map[string]string{"message": "failed to get broadcast"}) //nolint:errcheck
			return
		}

		sent := broadcast.Sent

		terminal := isTerminalState(broadcast.State)
		evt := map[string]any{
			"state":    string(broadcast.State),
			"sent":     sent,
			"total":    broadcast.Total,
			"terminal": terminal,
		}
		enc.WriteEvent("progress", evt) //nolint:errcheck

		if terminal {
			logger.Debug("broadcast progress stream completed", zap.String("state", string(broadcast.State)))
			return
		}

		select {
		case <-ctx.Done():
			logger.Debug("broadcast progress stream disconnected")
			return
		case <-ticker.C:
		}
	}
}

// isTerminalState returns true if the broadcast state indicates that no more
// progress events will be published (completed, failed, or cancelled).
func isTerminalState(state management.BroadcastState) bool {
	switch state {
	case management.BroadcastStateCompleted, management.BroadcastStateFailed, management.BroadcastStateCancelled:
		return true
	default:
		return false
	}
}

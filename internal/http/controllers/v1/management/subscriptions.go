package v1

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewSubscriptionsController(logger *zap.Logger, db *sqlx.DB) *SubscriptionsController {
	return &SubscriptionsController{
		logger: logger,
		db:     db,
		store:  management.NewState(db),
	}
}

type SubscriptionsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *management.State
}

func (srv *SubscriptionsController) CreateSubscription(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	body := oapi.CreateSubscriptionJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("name", body.Name))
	logger.Info("creating subscription type")

	subscription := management.Subscription{
		ProjectID: projectID,
		Name:      body.Name,
		Channel:   string(body.Channel),
		IsPublic:  true,
	}

	if body.IsPublic != nil {
		subscription.IsPublic = *body.IsPublic
	}

	subscriptionID, err := srv.store.SubscriptionsStore.CreateSubscription(ctx, subscription)
	if err != nil {
		logger.Error("failed to create subscription type", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	sub, err := srv.store.SubscriptionsStore.GetSubscription(ctx, projectID, subscriptionID)
	if err != nil {
		logger.Error("failed to fetch created subscription type", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("subscription type created", zap.Stringer("subscription_id", subscriptionID))
	json.Write(w, http.StatusCreated, sub.OAPI())
}

func (srv *SubscriptionsController) ListSubscriptions(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListSubscriptionsParams) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing subscription types")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	result, total, err := srv.store.SubscriptionsStore.ListSubscriptions(ctx, projectID, pagination)
	if err != nil {
		logger.Error("failed to list subscription types", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listed subscription types", zap.Int("count", len(result)))
	json.Write(w, http.StatusOK, oapi.SubscriptionListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: result.OAPI(),
	})
}

func (srv *SubscriptionsController) GetSubscription(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, subscriptionID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("subscription_id", subscriptionID))
	logger.Info("getting subscription type")

	subscription, err := srv.store.SubscriptionsStore.GetSubscription(ctx, projectID, subscriptionID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("subscription type not found", zap.Stringer("subscription_id", subscriptionID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("subscription type not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch subscription type", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("subscription type retrieved")
	json.Write(w, http.StatusOK, subscription.OAPI())
}

func (srv *SubscriptionsController) UpdateSubscription(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, subscriptionID uuid.UUID) {
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("subscription_id", subscriptionID))
	logger.Info("updating subscription type")

	ctx := r.Context()
	body := oapi.UpdateSubscriptionJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.store.SubscriptionsStore.UpdateSubscription(ctx, subscriptionID, body.Name, body.IsPublic)
	if err != nil {
		logger.Error("failed to update subscription type", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	subscription, err := srv.store.SubscriptionsStore.GetSubscription(ctx, projectID, subscriptionID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("subscription type not found", zap.Stringer("subscription_id", subscriptionID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("subscription type not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch updated subscription type", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("subscription type updated")
	json.Write(w, http.StatusOK, subscription.OAPI())
}

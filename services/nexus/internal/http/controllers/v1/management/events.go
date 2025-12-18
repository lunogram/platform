package v1

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/claim"
	"github.com/lunogram/platform/pkg/http/json"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

func NewEventsController(logger *zap.Logger, db *sqlx.DB) *EventsController {
	return &EventsController{
		logger: logger,
		db:     db,
		store:  store.NewStores(db),
	}
}

type EventsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.Stores
}

func (srv *EventsController) ListEvents(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("listing events")

	events, err := srv.store.ListEvents(ctx, projectID)
	if err != nil {
		logger.Error("failed to list events", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("events listed", zap.Int("count", len(events)))

	results := make([]oapi.EventWithSchema, len(events))
	for i, event := range events {
		results[i] = oapi.EventWithSchema{
			Id:    event.ID,
			Name:  event.Name,
			Paths: []string(event.Paths),
			Types: []string(event.Types),
		}
	}

	response := struct {
		Results []oapi.EventWithSchema `json:"results"`
	}{
		Results: results,
	}

	json.Write(w, http.StatusOK, response)
}

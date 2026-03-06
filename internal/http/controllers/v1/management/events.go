package v1

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/claim"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewEventsController(logger *zap.Logger, db *sqlx.DB) *EventsController {
	return &EventsController{
		logger: logger,
		db:     db,
		store:  subjects.NewState(db),
	}
}

type EventsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *subjects.State
}

func (srv *EventsController) ListUserEventSchemas(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("listing user event schemas")

	events, err := srv.store.ListEventSchemas(ctx, projectID, subjects.SubjectTypeUser)
	if err != nil {
		logger.Error("failed to list events", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("events listed", zap.Int("count", len(events)))

	results := make([]oapi.EventWithSchema, len(events))
	for i, event := range events {
		schema := make([]oapi.SchemaPath, len(event.Schema))
		for j, s := range event.Schema {
			// Add .data prefix so paths target the JSONB data column,
			// matching the user schema API convention.
			path := s.Path
			if path != ".data" && !strings.HasPrefix(path, ".data.") && !strings.HasPrefix(path, ".data[") {
				path = ".data" + path
			}
			schema[j] = oapi.SchemaPath{
				Path:  path,
				Types: []string(s.Types),
			}
		}

		results[i] = oapi.EventWithSchema{
			Id:     event.ID,
			Name:   event.Name,
			Schema: schema,
		}
	}

	json.Write(w, http.StatusOK, oapi.EventListResponse{
		Results: results,
	})
}

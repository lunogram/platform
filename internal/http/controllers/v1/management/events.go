package v1

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewEventsController(logger *zap.Logger, db *sqlx.DB, engine *rbac.Engine) *EventsController {
	return &EventsController{
		logger: logger,
		db:     db,
		store:  subjects.NewState(db),
		engine: engine,
	}
}

type EventsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *subjects.State
	engine *rbac.Engine
}

func (srv *EventsController) ListUserEventSchemas(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("events", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
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

func (srv *EventsController) DeleteUserEventSchema(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, eventID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("events", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("event_id", eventID.String()),
	)

	logger.Info("deleting user event schema")

	err = srv.store.DeleteEvent(ctx, projectID, eventID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("event not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("event not found")))
		return
	}

	if err != nil {
		logger.Error("failed to delete event", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user event schema deleted")
	w.WriteHeader(http.StatusNoContent)
}

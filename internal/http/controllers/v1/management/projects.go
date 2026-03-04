package v1

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/claim"
	"github.com/lunogram/platform/internal/claim/rbac"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"go.uber.org/zap"
)

func NewProjectsController(logger *zap.Logger, managementDB, usersDB, journeyDB *sqlx.DB) *ProjectsController {
	return &ProjectsController{
		logger:       logger,
		managementDB: managementDB,
		store:        management.NewState(managementDB),
		journey:      journey.NewState(journeyDB),
		users:        users.NewState(usersDB),
	}
}

type ProjectsController struct {
	logger       *zap.Logger
	managementDB *sqlx.DB
	store        *management.State
	journey      *journey.State
	users        *users.State
}

func (srv *ProjectsController) loadProjectCounts(ctx context.Context, project *management.Project) {
	wg := sync.WaitGroup{}

	wg.Go(func() {
		project.JourneysCount, _ = srv.journey.CountJourneys(ctx, project.ID) //nolint:errcheck
	})

	wg.Go(func() {
		project.UsersCount, _ = srv.users.CountUsers(ctx, project.ID) //nolint:errcheck
	})

	wg.Go(func() {
		project.ListsCount, _ = srv.users.CountLists(ctx, project.ID) //nolint:errcheck
	})

	wg.Wait()
}

func (srv *ProjectsController) ListProjects(w http.ResponseWriter, r *http.Request, params oapi.ListProjectsParams) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With()
	logger.Info("listing projects")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	search := ""
	if params.Search != nil {
		search = string(*params.Search)
	}

	projects, total, err := srv.store.ListProjects(ctx, scope.OrganizationID, pagination, search)
	if err != nil {
		logger.Error("failed to list projects", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	results := make([]oapi.Project, len(projects))
	for i, p := range projects {
		results[i] = p.OAPI()
	}

	logger.Info("listed projects", zap.Int("count", len(results)))
	json.Write(w, http.StatusOK, oapi.ProjectList{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: results,
	})
}

func (srv *ProjectsController) GetProject(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("getting project")

	project, err := srv.store.GetProject(ctx, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("project not found", zap.Stringer("project_id", projectID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	srv.loadProjectCounts(ctx, project)

	logger.Info("project retrieved")
	json.Write(w, http.StatusOK, project.OAPI())
}

func (srv *ProjectsController) CreateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	body := oapi.CreateProjectJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With()
	logger.Info("creating project", zap.String("name", body.Name))

	tx, err := srv.managementDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("unexpected error while attempting to start a transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	defer tx.Rollback() //nolint:errcheck

	projects := management.NewProjectsStore(tx)
	subscriptions := management.NewSubscriptionsStore(tx)

	var tools pq.StringArray
	if body.Tools != nil {
		tools = pq.StringArray(*body.Tools)
	}

	projectID, err := projects.CreateProject(ctx, management.Project{
		OrganizationID:    &scope.OrganizationID,
		Name:              body.Name,
		Description:       body.Description,
		Timezone:          body.Timezone,
		Locale:            body.Locale,
		TextOptOutMessage: body.TextOptOutMessage,
		TextHelpMessage:   body.TextHelpMessage,
		LinkWrapEmail:     body.LinkWrapEmail != nil && *body.LinkWrapEmail,
		LinkWrapPush:      body.LinkWrapPush != nil && *body.LinkWrapPush,
		Tools:             tools,
	})
	if err != nil {
		logger.Error("failed to create project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// NOTE: we add the session as admin to the project if available, the
	// session is not available for API key requests
	session, has := claim.FromContext(ctx)
	if has {
		admin, err := srv.store.GetAdminBySubject(ctx, session)
		if errors.Is(err, sql.ErrNoRows) {
			logger.Info("admin not found")
			oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
			return
		}

		err = projects.AddProjectAdmin(ctx, projectID, admin.ID, "admin")
		if err != nil {
			logger.Error("failed to add admin to project", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	// Create default subscriptions for each channel
	for _, channel := range []string{"email", "sms", "push"} {
		_, err = subscriptions.CreateSubscription(ctx, management.Subscription{
			ProjectID: projectID,
			Name:      "Default " + channel,
			Channel:   channel,
			IsPublic:  true,
		})
		if err != nil {
			logger.Error("failed to create default subscription", zap.Error(err), zap.String("channel", channel))
			oapi.WriteProblem(w, err)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("unexpected error while attempting to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("project created", zap.Stringer("project_id", projectID))
	project, err := srv.store.GetProject(ctx, projectID)
	if err != nil {
		logger.Error("failed to fetch created project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	srv.loadProjectCounts(ctx, project)

	json.Write(w, http.StatusCreated, project.OAPI())
}

func (srv *ProjectsController) UpdateProject(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("updating project")

	body := oapi.UpdateProjectJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var tools pq.StringArray
	if body.Tools != nil {
		tools = pq.StringArray(*body.Tools)
	}

	update := management.ProjectUpdate{
		Name:              body.Name,
		Description:       body.Description,
		Timezone:          body.Timezone,
		Locale:            body.Locale,
		TextOptOutMessage: body.TextOptOutMessage,
		TextHelpMessage:   body.TextHelpMessage,
		LinkWrapEmail:     body.LinkWrapEmail,
		LinkWrapPush:      body.LinkWrapPush,
		Tools:             tools,
	}

	err = srv.store.UpdateProject(ctx, projectID, update)
	if err != nil {
		logger.Error("failed to update project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	project, err := srv.store.GetProject(ctx, projectID)
	if err != nil {
		logger.Error("failed to fetch updated project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	srv.loadProjectCounts(ctx, project)

	logger.Info("project updated")
	json.Write(w, http.StatusOK, project.OAPI())
}

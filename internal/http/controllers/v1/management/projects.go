package v1

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rbac/access"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/internal/webhook"
	webhookoapi "github.com/lunogram/platform/oapi"
	"github.com/lunogram/platform/pkg/modules"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

func NewProjectsController(logger *zap.Logger, managementDB, usersDB, journeyDB *sqlx.DB, webhookCaller *webhook.Caller, pub pubsub.Publisher, engine *rbac.Engine) *ProjectsController {
	return &ProjectsController{
		logger:       logger,
		managementDB: managementDB,
		store:        management.NewState(managementDB),
		journey:      journey.NewState(journeyDB),
		users:        subjects.NewState(usersDB, logger),
		webhook:      webhookCaller,
		pub:          pub,
		engine:       engine,
	}
}

type ProjectsController struct {
	logger       *zap.Logger
	managementDB *sqlx.DB
	store        *management.State
	journey      *journey.State
	users        *subjects.State
	webhook      *webhook.Caller
	pub          pubsub.Publisher
	engine       *rbac.Engine
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
	actor := rbac.FromContext(ctx)
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID))
	if err != nil {
		oapi.WriteProblem(w, err)
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

	// This endpoint answers "which projects do I belong to", which only an admin
	// can be asked: the listing joins project_admins and carries the caller's
	// role per project. A non-admin has no row there and no role, so its honest
	// answer is a 403 rather than an empty list that reads as "you have none".
	adminID, err := adminActorID(actor)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	projects, total, err := srv.store.ListProjectsForAdmin(ctx, adminID, pagination, search)
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
	actor := rbac.FromContext(ctx)
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))

	// A nil admin id drops the project_admins join, so a non-admin actor gets the
	// project without a personal role rather than the role of whichever admin
	// happens to share its id shape.
	adminID, err := optionalAdminActorID(actor)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	project, err := srv.store.GetOrganizationProject(ctx, projectID, actor.OrganizationID, adminID)
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

	// GetProject returns only the explicit project_admins role (empty when the
	// admin has no row). Resolve the effective role so org owners/admins, who
	// are project admins by inheritance, are not reported as having no role.
	if adminID != nil {
		admin, adminErr := srv.store.GetAdmin(ctx, *adminID)
		if adminErr != nil {
			logger.Error("failed to load actor for role resolution", zap.Error(adminErr))
			oapi.WriteProblem(w, adminErr)
			return
		}
		project.Role = effectiveProjectRole(admin.Role, project.Role)
	}

	logger.Info("project retrieved")
	json.Write(w, http.StatusOK, project.OAPI())
}

func (srv *ProjectsController) CreateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.OrganizationScope(actor.OrganizationID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.CreateProjectJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With()
	logger.Info("creating project", zap.String("name", body.Name))

	adminID, err := optionalAdminActorID(actor)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.managementDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("unexpected error while attempting to start a transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	defer tx.Rollback() //nolint:errcheck

	projects := management.NewProjectsStore(tx)
	subscriptions := management.NewSubscriptionsStore(tx)

	projectID, err := projects.CreateProject(ctx, management.Project{
		OrganizationID:    &actor.OrganizationID,
		Name:              body.Name,
		Description:       body.Description,
		Timezone:          body.Timezone,
		Locale:            body.Locale,
		TextOptOutMessage: body.TextOptOutMessage,
		TextHelpMessage:   body.TextHelpMessage,
	})
	if err != nil {
		logger.Error("failed to create project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Only a human caller takes a seat on the project it just created; an API
	// key is a backend credential with nobody behind it to grant one to.
	if adminID != nil {
		err = projects.AddProjectAdmin(ctx, projectID, *adminID, "admin")
		if err != nil {
			logger.Error("failed to add admin to project", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	// Create default subscriptions for each channel
	for _, channel := range []modules.Channel{modules.ChannelEmail, modules.ChannelSMS, modules.ChannelPush} {
		_, err = subscriptions.CreateSubscription(ctx, management.Subscription{
			ProjectID: projectID,
			Name:      "Default " + string(channel),
			Channel:   string(channel),
			IsPublic:  true,
		})
		if err != nil {
			logger.Error("failed to create default subscription", zap.Error(err), zap.String("channel", string(channel)))
			oapi.WriteProblem(w, err)
			return
		}
	}

	// Create default locale from the project's locale setting
	locales := management.NewLocalesStore(tx)
	localeLabel := body.Locale
	if tag, langErr := language.Parse(body.Locale); langErr == nil {
		localeLabel = display.Self.Name(tag)
	}
	_, err = locales.CreateLocale(ctx, management.Locale{
		ProjectID: projectID,
		Key:       body.Locale,
		Label:     localeLabel,
	})
	if err != nil {
		logger.Error("failed to create default locale", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if srv.pub != nil {
		err = srv.pub.Publish(ctx, schemas.ProjectEventsProcess(actor.OrganizationID), schemas.ProjectEvent{
			ID:             projectID,
			Name:           schemas.EventProjectCreated,
			OrganizationID: actor.OrganizationID,
			Data: map[string]any{
				"id":              projectID,
				"organization_id": actor.OrganizationID,
				"name":            body.Name,
				"timezone":        body.Timezone,
				"locale":          body.Locale,
			},
		})
		if err != nil {
			logger.Error("failed to publish project created event", zap.Error(err))
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

	// Write the RBAC tuples that link the project to its organization and
	// connect every resource type to the project. Without these tuples,
	// project-scoped permission checks (e.g. listing providers) will fail
	// with 403 Forbidden.
	if err := access.ProvisionProject(ctx, srv.engine, actor.OrganizationID, projectID); err != nil {
		logger.Error("failed to provision RBAC tuples for project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("project created", zap.Stringer("project_id", projectID))
	project, err := srv.store.GetOrganizationProject(ctx, projectID, actor.OrganizationID, adminID)
	if err != nil {
		logger.Error("failed to fetch created project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	srv.loadProjectCounts(ctx, project)

	err = srv.webhook.ProjectCreated(ctx, r, webhookoapi.ProjectDetails{
		Id:             project.ID,
		OrganizationId: *project.OrganizationID,
		Name:           project.Name,
		Timezone:       &project.Timezone,
		Locale:         &project.Locale,
		CreatedAt:      project.CreatedAt,
	})
	if err != nil {
		logger.Error("failed to call project created webhook", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusCreated, project.OAPI())
}

func (srv *ProjectsController) UpdateProject(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.OrganizationScope(actor.OrganizationID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("updating project")

	adminID, err := optionalAdminActorID(actor)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.UpdateProjectJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	update := management.ProjectUpdate{
		Name:              body.Name,
		Description:       body.Description,
		Timezone:          body.Timezone,
		Locale:            body.Locale,
		TextOptOutMessage: body.TextOptOutMessage,
		TextHelpMessage:   body.TextHelpMessage,
	}

	err = srv.store.UpdateProject(ctx, projectID, actor.OrganizationID, update)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("project not found", zap.Stringer("project_id", projectID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project not found")))
		return
	}

	if err != nil {
		logger.Error("failed to update project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	project, err := srv.store.GetOrganizationProject(ctx, projectID, actor.OrganizationID, adminID)
	if err != nil {
		logger.Error("failed to fetch updated project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	srv.loadProjectCounts(ctx, project)

	logger.Info("project updated")
	json.Write(w, http.StatusOK, project.OAPI())
}

func (srv *ProjectsController) DeleteProject(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.OrganizationScope(actor.OrganizationID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("deleting project")

	err = srv.store.DeleteProject(ctx, projectID, actor.OrganizationID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("project not found", zap.Stringer("project_id", projectID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project not found")))
		return
	}

	if err != nil {
		logger.Error("failed to delete project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Clean up the RBAC tuples that were created by ProvisionProject.
	if err := access.DeprovisionProject(ctx, srv.engine, actor.OrganizationID, projectID); err != nil {
		logger.Error("failed to deprovision RBAC tuples for project", zap.Error(err))
	}

	logger.Info("project deleted")
	w.WriteHeader(http.StatusNoContent)
}

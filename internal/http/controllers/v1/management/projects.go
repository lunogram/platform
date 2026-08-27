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

	actorID, err := uuid.Parse(actor.ID)
	if err != nil {
		logger.Error("failed to parse actor ID as UUID", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Scoped to the organization that scopes this session: an admin who belongs
	// to several organizations must not see another organization's projects
	// listed while this one is active.
	projects, total, err := srv.store.ListProjectsForAdmin(ctx, actor.OrganizationID, actorID, rbac.OrganizationRolesInheritingProjectAdmin(), pagination, search)
	if err != nil {
		logger.Error("failed to list projects", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	results := make([]oapi.Project, len(projects))
	for i, p := range projects {
		// The raw project_admins role is empty for a project reached purely by
		// org→project inheritance. Resolve the effective role so the console
		// gates on what the admin can actually do, not on a missing row.
		p.Project.Role = rbac.EffectiveProjectRole(p.MembershipRole, p.Project.Role)
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

	actorID, err := uuid.Parse(actor.ID)
	if err != nil {
		logger.Error("failed to parse actor ID as UUID", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// The permission above was checked against the actor's OWN organization, so
	// the load must be constrained to it as well. Reading by bare project id
	// would let any authenticated admin fetch any project by guessing its uuid;
	// a foreign project is reported as not found, identical to one that does not
	// exist, so the response does not confirm that the id is real.
	project, err := srv.store.GetProjectInOrganization(ctx, projectID, actor.OrganizationID, &actorID)
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
	project.Role, err = resolveProjectRole(ctx, srv.store, actor.OrganizationID, projectID, actorID)
	if err != nil {
		logger.Error("failed to resolve project role", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
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

	// NOTE: we add the admin as a project admin after creation if the
	// caller is a human (JWT). API key callers don't get added.
	var creatorID *uuid.UUID
	if actor.Type == rbac.ActorAdmin {
		adminID, parseErr := uuid.Parse(actor.ID)
		if parseErr != nil {
			logger.Error("failed to parse actor ID as UUID", zap.Error(parseErr))
			oapi.WriteProblem(w, parseErr)
			return
		}

		err = projects.AddProjectAdmin(ctx, projectID, adminID, rbac.ProjectAdmin)
		if err != nil {
			logger.Error("failed to add admin to project", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
		creatorID = &adminID
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

	// The creator's project_admins row needs its matching tuple. Today the
	// creator also reaches the project by org→project inheritance, so skipping
	// this appears to work — until they are demoted from organization admin and
	// silently lose a project they own on paper. Postgres and OpenFGA must agree
	// about who holds what.
	if creatorID != nil {
		if err := access.ProvisionProjectRole(ctx, srv.engine, *creatorID, projectID, rbac.ProjectAdmin); err != nil {
			logger.Error("failed to provision RBAC project role for creator", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	actorID, err := uuid.Parse(actor.ID)
	if err != nil {
		logger.Error("failed to parse actor ID as UUID", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("project created", zap.Stringer("project_id", projectID))
	project, err := srv.store.GetProject(ctx, projectID, &actorID)
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

	// Update was authorized against the actor's own organization, so the target
	// must belong to it. Without this the handler mutates any project by uuid.
	if _, err := srv.store.GetProjectInOrganization(ctx, projectID, actor.OrganizationID, nil); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Info("project not found")
			oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project not found")))
			return
		}
		logger.Error("failed to get project", zap.Error(err))
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

	err = srv.store.UpdateProject(ctx, projectID, update)
	if err != nil {
		logger.Error("failed to update project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	actorID, err := uuid.Parse(actor.ID)
	if err != nil {
		logger.Error("failed to parse actor ID as UUID", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	project, err := srv.store.GetProject(ctx, projectID, &actorID)
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

	_, err = srv.store.GetProjectInOrganization(ctx, projectID, actor.OrganizationID, nil)
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

	// Read the per-admin role grants before the project goes away; afterwards
	// there is nothing left to enumerate them from. DeprovisionProject only
	// removes the project→organization and resource→project tuples, so without
	// this every member keeps a live project:<id>#<role> tuple.
	projectRoles, err := srv.store.ListProjectAdminRoles(ctx, projectID)
	if err != nil {
		logger.Error("failed to list project admin roles", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Tuples out first, rows second — the same revocation ordering the member
	// endpoints use. This is what makes the failure recoverable: an error here
	// leaves the project and its roster fully intact, so the request is simply
	// retried. Were the project deleted first, a tuple failure would strand
	// grants that nothing left in the database can enumerate.
	//
	// These failures are therefore fatal rather than best-effort. It is tempting
	// to shrug them off because the project is going away, but the shrug is only
	// safe once the tuples are actually gone; before that it is the difference
	// between a retryable error and a permanent leak.
	grants := make([]access.ProjectRoleGrant, len(projectRoles))
	for i, pr := range projectRoles {
		grants[i] = access.ProjectRoleGrant{UserID: pr.AdminID, ProjectID: projectID, Role: pr.Role}
	}
	if err := access.DeprovisionProjectRoles(ctx, srv.engine, grants); err != nil {
		logger.Error("failed to revoke project role tuples", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to revoke project roles")))
		return
	}

	// Clean up the RBAC tuples that were created by ProvisionProject.
	if err := access.DeprovisionProject(ctx, srv.engine, actor.OrganizationID, projectID); err != nil {
		logger.Error("failed to deprovision RBAC tuples for project", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to deprovision project")))
		return
	}

	// Both row deletions commit together. Soft-deleting the project on its own
	// would make a failed roster cleanup unreplayable: the retry's
	// GetProjectInOrganization no longer sees the project and answers 404,
	// stranding the membership rows for good.
	tx, err := srv.managementDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	defer tx.Rollback() //nolint:errcheck

	txStore := management.NewState(tx)

	if err := txStore.DeleteProject(ctx, projectID); err != nil {
		logger.Error("failed to delete project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err := txStore.RemoveProjectAdmins(ctx, projectID); err != nil {
		logger.Error("failed to remove project roster", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err := tx.Commit(); err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("project deleted")
	w.WriteHeader(http.StatusNoContent)
}

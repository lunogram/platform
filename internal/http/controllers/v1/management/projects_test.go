package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rbac/access"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/lunogram/platform/internal/webhook"
	webhookoapi "github.com/lunogram/platform/oapi"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type publishedMessage struct {
	subject schemas.Subject
	data    []byte
}

type recordingPublisher struct {
	messages []publishedMessage
}

func (r *recordingPublisher) Publish(_ context.Context, subject schemas.Subject, v any, _ ...pubsub.PublishOption) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	r.messages = append(r.messages, publishedMessage{subject: subject, data: data})
	return nil
}

type failingPublisher struct{}

func (f *failingPublisher) Publish(_ context.Context, _ schemas.Subject, _ any, _ ...pubsub.PublishOption) error {
	return errors.New("publish failed")
}

func TestCreateProject(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, jrny := teststore.RunPostgreSQL(t)

	orgs := management.NewOrganizationsStore(mgmt)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	require.NoError(t, management.NewOrganizationMembersStore(mgmt).AddMember(ctx, orgID, adminID, "owner"))

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(),
		rbac.WithOrganizationID(admin.OrganizationID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "")

	projects := NewProjectsController(logger, mgmt, usrs, jrny, nil, nil, engine)

	type test struct {
		body oapi.CreateProjectJSONRequestBody
		code int
	}

	tests := map[string]test{
		"success": {
			body: oapi.CreateProjectJSONRequestBody{
				Name:     "Test Project",
				Timezone: "America/New_York",
				Locale:   "en",
			},
			code: http.StatusCreated,
		},
		"with description": {
			body: oapi.CreateProjectJSONRequestBody{
				Name:        "Test Project",
				Description: ptr.To("A test project"),
				Timezone:    "America/New_York",
				Locale:      "en",
			},
			code: http.StatusCreated,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/api/admin/projects", bytes.NewReader(bb))
			req = req.WithContext(actorCtx)

			projects.CreateProject(res, req)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == http.StatusCreated {
				var project oapi.Project
				err = json.NewDecoder(res.Body).Decode(&project)
				require.NoError(t, err)
				require.Equal(t, test.body.Name, project.Name)
				require.Equal(t, test.body.Timezone, project.Timezone)
				require.Equal(t, test.body.Locale, project.Locale)
				require.NotEqual(t, uuid.Nil, project.Id)
			}
		})
	}
}

func TestListProjects(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, jrny := teststore.RunPostgreSQL(t)

	orgs := management.NewOrganizationsStore(mgmt)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	require.NoError(t, management.NewOrganizationMembersStore(mgmt).AddMember(ctx, orgID, adminID, "owner"))

	projectStore := management.NewProjectsStore(mgmt)
	for i := 0; i < 3; i++ {
		projectID, err := projectStore.CreateProject(ctx, management.Project{
			OrganizationID: &orgID,
			Name:           "Test Project",
			Timezone:       "UTC",
			Locale:         "en",
		})
		require.NoError(t, err)

		err = projectStore.AddProjectAdmin(ctx, projectID, adminID, "admin")
		require.NoError(t, err)
	}

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(),
		rbac.WithOrganizationID(admin.OrganizationID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "")

	projects := NewProjectsController(logger, mgmt, usrs, jrny, nil, nil, engine)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects", nil)
	req = req.WithContext(actorCtx)

	limit := oapi.PaginationLimit(10)
	offset := oapi.PaginationOffset(0)
	params := oapi.ListProjectsParams{
		Limit:  &limit,
		Offset: &offset,
	}

	projects.ListProjects(res, req, params)

	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	var result oapi.ProjectList
	err = json.NewDecoder(res.Body).Decode(&result)
	require.NoError(t, err)
	require.Equal(t, 3, result.Total)
	require.Len(t, result.Results, 3)
}

func TestGetProject(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, jrny := teststore.RunPostgreSQL(t)

	orgs := management.NewOrganizationsStore(mgmt)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	require.NoError(t, management.NewOrganizationMembersStore(mgmt).AddMember(ctx, orgID, adminID, "owner"))

	projectStore := management.NewProjectsStore(mgmt)
	projectID, err := projectStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	err = projectStore.AddProjectAdmin(ctx, projectID, adminID, "admin")
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(),
		rbac.WithOrganizationID(admin.OrganizationID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "")

	projects := NewProjectsController(logger, mgmt, usrs, jrny, nil, nil, engine)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String(), nil)
	req = req.WithContext(actorCtx)

	projects.GetProject(res, req, projectID)

	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	var project oapi.Project
	err = json.NewDecoder(res.Body).Decode(&project)
	require.NoError(t, err)
	require.Equal(t, projectID, project.Id)
	require.Equal(t, "Test Project", project.Name)
}

func TestUpdateProject(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, jrny := teststore.RunPostgreSQL(t)

	orgs := management.NewOrganizationsStore(mgmt)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	require.NoError(t, management.NewOrganizationMembersStore(mgmt).AddMember(ctx, orgID, adminID, "owner"))

	projectStore := management.NewProjectsStore(mgmt)
	projectID, err := projectStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	err = projectStore.AddProjectAdmin(ctx, projectID, adminID, "admin")
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(),
		rbac.WithOrganizationID(admin.OrganizationID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "")

	projects := NewProjectsController(logger, mgmt, usrs, jrny, nil, nil, engine)

	type test struct {
		body oapi.UpdateProjectJSONRequestBody
		code int
	}

	tests := map[string]test{
		"update name": {
			body: oapi.UpdateProjectJSONRequestBody{
				Name: ptr.To("Updated Project"),
			},
			code: http.StatusOK,
		},
		"update timezone": {
			body: oapi.UpdateProjectJSONRequestBody{
				Timezone: ptr.To("America/Los_Angeles"),
			},
			code: http.StatusOK,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/api/admin/projects/"+projectID.String(), bytes.NewReader(bb))
			req = req.WithContext(actorCtx)

			projects.UpdateProject(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == http.StatusOK {
				var project oapi.Project
				err = json.NewDecoder(res.Body).Decode(&project)
				require.NoError(t, err)
				require.Equal(t, projectID, project.Id)

				if test.body.Name != nil {
					require.Equal(t, *test.body.Name, project.Name)
				}
				if test.body.Timezone != nil {
					require.Equal(t, *test.body.Timezone, project.Timezone)
				}
			}
		})
	}
}

func TestCreateProjectWebhook(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, jrny := teststore.RunPostgreSQL(t)

	orgs := management.NewOrganizationsStore(mgmt)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	var called atomic.Bool
	var receivedEvent webhookoapi.ProjectCreatedEvent

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)

		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, string(webhookoapi.ProjectCreated), r.Header.Get("X-Webhook-Event"))

		err := json.NewDecoder(r.Body).Decode(&receivedEvent)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	caller := webhook.NewCaller(logger, config.Webhook{
		ProjectCreatedURL: webhookServer.URL,
	})

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(),
		rbac.WithOrganizationID(orgID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "")

	projects := NewProjectsController(logger, mgmt, usrs, jrny, caller, nil, engine)

	body := oapi.CreateProjectJSONRequestBody{
		Name:     "Webhook Test Project",
		Timezone: "America/New_York",
		Locale:   "en",
	}

	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects", bytes.NewReader(bb))
	req = req.WithContext(actorCtx)

	projects.CreateProject(res, req)

	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())
	require.True(t, called.Load(), "webhook should have been called")

	require.Equal(t, webhookoapi.ProjectCreated, receivedEvent.Event)
	require.Equal(t, "Webhook Test Project", receivedEvent.Project.Name)
	require.Equal(t, orgID, receivedEvent.Project.OrganizationId)
	require.NotEqual(t, uuid.Nil, receivedEvent.Project.Id)
}

func TestCreateProjectPublishesNATSEvent(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, jrny := teststore.RunPostgreSQL(t)

	orgs := management.NewOrganizationsStore(mgmt)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	pub := &recordingPublisher{}

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(),
		rbac.WithOrganizationID(orgID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "")

	projects := NewProjectsController(logger, mgmt, usrs, jrny, nil, pub, engine)

	body := oapi.CreateProjectJSONRequestBody{
		Name:     "NATS Test Project",
		Timezone: "America/New_York",
		Locale:   "en",
	}

	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects", bytes.NewReader(bb))
	req = req.WithContext(actorCtx)

	projects.CreateProject(res, req)

	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())
	require.Len(t, pub.messages, 1, "expected one NATS event to be published")

	expectedSubject := schemas.ProjectEventsProcess(orgID)
	require.Equal(t, expectedSubject, pub.messages[0].subject)

	var event schemas.ProjectEvent
	err = json.Unmarshal(pub.messages[0].data, &event)
	require.NoError(t, err)
	require.Equal(t, schemas.EventProjectCreated, event.Name)
	require.Equal(t, orgID, event.OrganizationID)
	require.NotEqual(t, uuid.Nil, event.ID)
	require.Equal(t, "NATS Test Project", event.Data["name"])
	require.Equal(t, "America/New_York", event.Data["timezone"])
	require.Equal(t, "en", event.Data["locale"])
}

func TestCreateProjectRollbackOnPublishFailure(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, jrny := teststore.RunPostgreSQL(t)

	orgs := management.NewOrganizationsStore(mgmt)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	pub := &failingPublisher{}

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "")

	projects := NewProjectsController(logger, mgmt, usrs, jrny, nil, pub, engine)

	body := oapi.CreateProjectJSONRequestBody{
		Name:     "Rollback Test Project",
		Timezone: "America/New_York",
		Locale:   "en",
	}

	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects", bytes.NewReader(bb))
	req = req.WithContext(actorCtx)

	projects.CreateProject(res, req)

	require.Equal(t, http.StatusInternalServerError, res.Code, "request should fail when publish fails")

	// Verify the project was not persisted (transaction was rolled back)
	mgmtState := management.NewState(mgmt)
	_, total, err := mgmtState.ListProjects(ctx, orgID, store.Pagination{Limit: 10, Offset: 0}, "")
	require.NoError(t, err)
	require.Equal(t, 0, total, "project should not exist after rollback")
}

// projectScopeEnv builds two organizations sharing one admin, the shape that
// makes organization scoping observable.
type projectScopeEnv struct {
	controller *ProjectsController
	engine     *rbac.Engine
	state      *management.State
}

func newProjectScopeEnv(t *testing.T) projectScopeEnv {
	t.Helper()

	mgmt, usrs, jrny := teststore.RunPostgreSQL(t)
	engine := rbac.NewTestEngine(t)

	return projectScopeEnv{
		controller: NewProjectsController(zaptest.NewLogger(t), mgmt, usrs, jrny, nil, nil, engine),
		engine:     engine,
		state:      management.NewState(mgmt),
	}
}

func (env projectScopeEnv) newOrg(t *testing.T, name string) uuid.UUID {
	t.Helper()
	orgID, err := env.state.CreateOrganization(context.Background(), name)
	require.NoError(t, err)
	return orgID
}

func (env projectScopeEnv) newProject(t *testing.T, orgID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	projectID, err := env.state.CreateProject(context.Background(), management.Project{
		OrganizationID: &orgID,
		Name:           name,
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)
	require.NoError(t, access.ProvisionProject(context.Background(), env.engine, orgID, projectID))
	return projectID
}

func (env projectScopeEnv) newAdmin(t *testing.T, homeOrg uuid.UUID, email, role string) uuid.UUID {
	t.Helper()
	adminID, err := env.state.CreateAdmin(context.Background(), management.Admin{
		OrganizationID: homeOrg,
		Email:          email,
		Role:           role,
	})
	require.NoError(t, err)
	env.join(t, homeOrg, adminID, role)
	return adminID
}

func (env projectScopeEnv) join(t *testing.T, orgID, adminID uuid.UUID, role string) {
	t.Helper()
	require.NoError(t, env.state.AddMember(context.Background(), orgID, adminID, role))
	require.NoError(t, env.engine.WriteTuples(context.Background(),
		access.OrganizationRoleTuples(adminID, orgID, role)))
}

func (env projectScopeEnv) request(method string, adminID, activeOrg uuid.UUID) *http.Request {
	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(), rbac.WithOrganizationID(activeOrg))
	return httptest.NewRequest(method, "/", http.NoBody).WithContext(rbac.WithActor(context.Background(), actor))
}

// TestGetProjectRejectsForeignOrganization: reading used to authorize against
// the actor's own organization and then load the project by bare id, so any
// authenticated admin could fetch any project by guessing its uuid. A foreign
// project must be indistinguishable from one that does not exist.
func TestGetProjectRejectsForeignOrganization(t *testing.T) {
	t.Parallel()

	env := newProjectScopeEnv(t)
	orgA := env.newOrg(t, "Org A")
	orgB := env.newOrg(t, "Org B")
	secret := env.newProject(t, orgB, "Org B Secret Project")

	snooper := env.newAdmin(t, orgA, "snooper@example.com", rbac.OrganizationOwner)

	res := httptest.NewRecorder()
	env.controller.GetProject(res, env.request(http.MethodGet, snooper, orgA), secret)
	require.Equal(t, http.StatusNotFound, res.Code, res.Body.String())

	// The same id resolves once the actor's session is scoped to org B.
	env.join(t, orgB, snooper, rbac.OrganizationMember)
	res = httptest.NewRecorder()
	env.controller.GetProject(res, env.request(http.MethodGet, snooper, orgB), secret)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
}

// TestListProjectsScopedToActiveOrganization covers both list defects: projects
// of other organizations leaked into the active session, and projects reachable
// only by org→project inheritance were missing entirely.
func TestListProjectsScopedToActiveOrganization(t *testing.T) {
	t.Parallel()

	env := newProjectScopeEnv(t)
	orgA := env.newOrg(t, "Org A")
	orgB := env.newOrg(t, "Org B")

	explicit := env.newProject(t, orgA, "Explicit Project")
	inherited := env.newProject(t, orgA, "Inherited Project")
	foreign := env.newProject(t, orgB, "Foreign Project")

	// An owner of org A who also belongs to org B, and who has an explicit row on
	// only one of org A's projects.
	owner := env.newAdmin(t, orgA, "owner@example.com", rbac.OrganizationOwner)
	env.join(t, orgB, owner, rbac.OrganizationMember)
	require.NoError(t, env.state.AddAdminToProject(context.Background(), explicit, owner, rbac.ProjectEditor))
	require.NoError(t, env.state.AddAdminToProject(context.Background(), foreign, owner, rbac.ProjectEditor))

	limit := oapi.PaginationLimit(50)
	offset := oapi.PaginationOffset(0)
	params := oapi.ListProjectsParams{Limit: &limit, Offset: &offset}

	res := httptest.NewRecorder()
	env.controller.ListProjects(res, env.request(http.MethodGet, owner, orgA), params)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	var list oapi.ProjectList
	require.NoError(t, json.NewDecoder(res.Body).Decode(&list))

	roles := map[uuid.UUID]string{}
	for _, p := range list.Results {
		roles[p.Id] = p.Role
	}

	require.Len(t, list.Results, 2)
	require.Equal(t, 2, list.Total)
	require.Contains(t, roles, explicit)
	require.Contains(t, roles, inherited, "an org owner must see projects they hold by inheritance")
	require.NotContains(t, roles, foreign, "another organization's project must not leak into this session")

	// The effective role is reported, not the raw project_admins column: an org
	// owner is a project admin everywhere in their organization.
	require.Equal(t, rbac.ProjectAdmin, roles[explicit])
	require.Equal(t, rbac.ProjectAdmin, roles[inherited])

	// A plain member of org B sees only what they were explicitly granted.
	member := env.newAdmin(t, orgB, "member@example.com", rbac.OrganizationMember)
	require.NoError(t, env.state.AddAdminToProject(context.Background(), foreign, member, rbac.ProjectSupport))

	res = httptest.NewRecorder()
	env.controller.ListProjects(res, env.request(http.MethodGet, member, orgB), params)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	list = oapi.ProjectList{}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&list))
	require.Len(t, list.Results, 1)
	require.Equal(t, foreign, list.Results[0].Id)
	require.Equal(t, rbac.ProjectSupport, list.Results[0].Role)
}

// TestCreateProjectProvisionsCreatorRole: the creator's project_admins row must
// come with its tuple. It used to work only because the creator inherited
// project admin from their organization role.
func TestCreateProjectProvisionsCreatorRole(t *testing.T) {
	t.Parallel()

	env := newProjectScopeEnv(t)
	orgID := env.newOrg(t, "Creator Org")
	creator := env.newAdmin(t, orgID, "creator@example.com", rbac.OrganizationOwner)

	body, err := json.Marshal(oapi.CreateProjectJSONRequestBody{
		Name: "Fresh Project", Timezone: "UTC", Locale: "en",
	})
	require.NoError(t, err)

	req := env.request(http.MethodPost, creator, orgID)
	req.Body = io.NopCloser(bytes.NewReader(body))

	res := httptest.NewRecorder()
	env.controller.CreateProject(res, req)
	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())

	var project oapi.Project
	require.NoError(t, json.NewDecoder(res.Body).Decode(&project))

	// Checked as a direct project role, not through the organization: the whole
	// point is that the grant no longer depends on the inheritance path.
	allowed, err := env.engine.Check(context.Background(), "user:"+creator.String(),
		rbac.ProjectAdmin, rbac.ProjectScope(project.Id))
	require.NoError(t, err)
	require.True(t, allowed, "the creator's explicit project admin role must be provisioned")
}

// TestBackfillProjectRoleTuples covers the repair path for project_admins rows
// that were recorded without their RBAC tuple.
func TestBackfillProjectRoleTuples(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmt)
	engine := rbac.NewTestEngine(t)

	orgID, err := state.CreateOrganization(ctx, "Backfill Org")
	require.NoError(t, err)

	projectID, err := state.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Backfill Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)
	require.NoError(t, access.ProvisionProject(ctx, engine, orgID, projectID))

	adminID, err := state.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "stranded@example.com",
		Role:           rbac.OrganizationMember,
	})
	require.NoError(t, err)

	// A row without its tuple: the state left behind by project creation before
	// the role was provisioned.
	require.NoError(t, state.AddAdminToProject(ctx, projectID, adminID, rbac.ProjectEditor))

	allowed, err := engine.Check(ctx, "user:"+adminID.String(), "read", rbac.ProjectResourceScope("members", projectID))
	require.NoError(t, err)
	require.False(t, allowed)

	require.NoError(t, access.BackfillProjectRoleTuples(ctx, zaptest.NewLogger(t), engine, mgmt))
	// Idempotent: a second run must not fail on the tuples it just wrote.
	require.NoError(t, access.BackfillProjectRoleTuples(ctx, zaptest.NewLogger(t), engine, mgmt))

	allowed, err = engine.Check(ctx, "user:"+adminID.String(), "read", rbac.ProjectResourceScope("members", projectID))
	require.NoError(t, err)
	require.True(t, allowed, "the backfill must restore the recorded project role")
}

// TestDeleteProjectRevokesMemberRoles: deprovisioning the project removes the
// project→organization and resource→project tuples but not the per-admin role
// grants, which would otherwise outlive the project they refer to.
func TestDeleteProjectRevokesMemberRoles(t *testing.T) {
	t.Parallel()

	env := newProjectScopeEnv(t)
	orgID := env.newOrg(t, "Deleting Org")
	projectID := env.newProject(t, orgID, "Doomed Project")

	owner := env.newAdmin(t, orgID, "owner@example.com", rbac.OrganizationOwner)
	member := env.newAdmin(t, orgID, "member@example.com", rbac.OrganizationMember)
	require.NoError(t, env.state.AddAdminToProject(context.Background(), projectID, member, rbac.ProjectEditor))
	require.NoError(t, access.ProvisionProjectRole(context.Background(), env.engine, member, projectID, rbac.ProjectEditor))

	res := httptest.NewRecorder()
	env.controller.DeleteProject(res, env.request(http.MethodDelete, owner, orgID), projectID)
	require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())

	allowed, err := env.engine.Check(context.Background(), "user:"+member.String(), rbac.ProjectEditor, rbac.ProjectScope(projectID))
	require.NoError(t, err)
	require.False(t, allowed, "a deleted project must leave no role grants behind")

	// And the roster is retired, so a later organization removal has no stale
	// grant to replay.
	roles, err := env.state.ListProjectRolesInOrganization(context.Background(), orgID, member)
	require.NoError(t, err)
	require.Empty(t, roles)
}

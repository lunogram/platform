package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/rbac"

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

	admins := management.NewState(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

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

	admins := management.NewState(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

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

	admins := management.NewState(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

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

	admins := management.NewState(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

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

	admins := management.NewState(mgmt)
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

	admins := management.NewState(mgmt)
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

// TestProjectEndpointsWithAnAPIKeyActor covers the guards that used to read
// "does the actor id parse as a UUID?" as "is the caller an admin?".
//
// An API key's id is an auth-method UUID, so it parsed cleanly and the request
// continued with a key id where an admin id was expected -- querying
// project_admins for an id that can never appear there. The type is what
// actually answers the question, so it is what gets checked.
func TestProjectEndpointsWithAnAPIKeyActor(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, jrny := teststore.RunPostgreSQL(t)

	state := management.NewState(mgmt)
	orgID, err := state.CreateOrganization(ctx, "Key Organization")
	require.NoError(t, err)

	projectID, err := state.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Key Project",
	})
	require.NoError(t, err)

	// The key's id is an auth-method UUID: a perfectly well-formed UUID that is
	// not, and can never be, an admin id.
	keyActor := rbac.NewActor(rbac.ActorAPIKey, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, keyActor, "owner", "")

	projects := NewProjectsController(logger, mgmt, usrs, jrny, nil, nil, engine)

	t.Run("listing projects is refused rather than silently empty", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/admin/projects", nil).WithContext(actorCtx)

		projects.ListProjects(res, req, oapi.ListProjectsParams{})

		require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	})

	t.Run("getting a project succeeds with no personal role", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String(), nil).WithContext(actorCtx)

		projects.GetProject(res, req, projectID)

		// Authorization was already settled by the permission check; only the
		// role decoration does not apply. Previously the effective-role lookup
		// resolved the key id as an admin and turned this into a 500.
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())

		var project oapi.Project
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &project))
		require.Empty(t, project.Role, "an API key has no personal project role")
	})
}

// TestGetProjectReportsTheAdminsEffectiveRole is the counterpart: an admin still
// gets their inherited role, so the guard above did not cost the feature it
// protects.
func TestGetProjectReportsTheAdminsEffectiveRole(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, usrs, jrny := teststore.RunPostgreSQL(t)

	state := management.NewState(mgmt)
	orgID, err := state.CreateOrganization(ctx, "Role Organization")
	require.NoError(t, err)

	adminID, err := state.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "owner@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)

	projectID, err := state.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Role Project",
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(), rbac.WithOrganizationID(orgID))
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "")

	projects := NewProjectsController(logger, mgmt, usrs, jrny, nil, nil, engine)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String(), nil).WithContext(actorCtx)

	projects.GetProject(res, req, projectID)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	var project oapi.Project
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &project))
	require.NotEmpty(t, project.Role, "an org owner is a project admin by inheritance")
}

package v1

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const testMaxUploadSize = 10485760 // 10MB

//go:embed test/users/valid.csv
var validUsersCSV string

//go:embed test/users/no-external-id.csv
var noExternalIDCSV string

//go:embed test/users/out-of-order.csv
var outOfOrderCSV string

func TestListCreation(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	lists := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	type test struct {
		body oapi.CreateListJSONRequestBody
		code int
	}

	tests := map[string]test{
		"static list": {
			body: oapi.CreateListJSONRequestBody{
				Name: "Static Test List",
				Type: oapi.CreateListTypeStatic,
			},
			code: 201,
		},
		"dynamic list": {
			body: oapi.CreateListJSONRequestBody{
				Name: "Dynamic Test List",
				Type: oapi.CreateListTypeDynamic,
			},
			code: 201,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/lists", bytes.NewReader(bb))
			req = req.WithContext(actorCtx)
			lists.CreateList(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 201 {
				var response oapi.List
				err := json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, test.body.Name, response.Name)
				require.Equal(t, string(test.body.Type), string(response.Type))
			}
		})
	}
}

func TestListLists(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	listsStore := subjects.NewListsStore(usrs)

	testLists := []subjects.List{
		{
			ProjectID: projectID,
			Name:      "Test List 1",
			Type:      "static",
		},
		{
			ProjectID: projectID,
			Name:      "Test List 2",
			Type:      "static",
		},
		{
			ProjectID: projectID,
			Name:      "Test List 3",
			Type:      "static",
		},
	}

	for _, list := range testLists {
		_, err := listsStore.CreateList(ctx, list)
		require.NoError(t, err)
	}

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	type test struct {
		limit  int
		offset int
		total  int
		result int
	}

	tests := map[string]test{
		"default": {
			limit:  10,
			offset: 0,
			total:  3,
			result: 3,
		},
		"with limit": {
			limit:  2,
			offset: 0,
			total:  3,
			result: 2,
		},
		"with offset": {
			limit:  10,
			offset: 1,
			total:  3,
			result: 2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			limit := oapi.Limit(test.limit)
			offset := oapi.Offset(test.offset)

			params := oapi.ListListsParams{
				Limit:  &limit,
				Offset: &offset,
			}

			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/lists", nil)
			req = req.WithContext(actorCtx)
			controller.ListLists(res, req, projectID, params)

			require.Equal(t, 200, res.Code, res.Body.String())

			var response oapi.ListListResponse
			err := json.Unmarshal(res.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, test.total, response.Total)
			require.Equal(t, test.result, len(response.Results))
		})
	}
}

func TestGetList(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	listsStore := subjects.NewListsStore(usrs)
	listID, err := listsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "static",
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/lists/"+listID.String(), nil)
	req = req.WithContext(actorCtx)
	controller.GetList(res, req, projectID, listID)

	require.Equal(t, 200, res.Code, res.Body.String())

	var response oapi.List
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, listID, response.Id)
	require.Equal(t, "Test List", response.Name)
}

func TestUpdateList(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	listsStore := subjects.NewListsStore(usrs)
	listID, err := listsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "dynamic",
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	body := oapi.UpdateListJSONRequestBody{
		Name: "Updated List",
	}

	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/v1/lists/"+listID.String(), bytes.NewReader(bb))
	req = req.WithContext(actorCtx)
	controller.UpdateList(res, req, projectID, listID)

	require.Equal(t, 200, res.Code, res.Body.String())

	var response oapi.List
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "Updated List", response.Name)
}

func TestDeleteList(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	listsStore := subjects.NewListsStore(usrs)
	listID, err := listsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "static",
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/lists/"+listID.String(), nil)
	req = req.WithContext(actorCtx)
	controller.DeleteList(res, req, projectID, listID)

	require.Equal(t, 204, res.Code, res.Body.String())

	_, err = listsStore.GetList(ctx, projectID, listID)
	require.Error(t, err)
}

func TestDuplicateList(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	listsStore := subjects.NewListsStore(usrs)
	listID, err := listsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Original List",
		Type:      "static",
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/lists/"+listID.String()+"/duplicate", nil)
	req = req.WithContext(actorCtx)
	controller.DuplicateList(res, req, projectID, listID)

	require.Equal(t, 201, res.Code, res.Body.String())

	var response oapi.List
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "Copy of Original List", response.Name)
	require.NotEqual(t, listID, response.Id)
	require.Equal(t, 0, response.Version)
}

func TestImportListUsers(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	type test struct {
		csv   string
		code  int
		users int
	}

	tests := map[string]test{
		"successful-import": {
			csv:   validUsersCSV,
			code:  204,
			users: 3,
		},
		"missing-external-id-column": {
			csv:   noExternalIDCSV,
			code:  400,
			users: 0,
		},
		"out-of-order-columns": {
			csv:   outOfOrderCSV,
			code:  204,
			users: 3,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mgmt, usrs, _ := teststore.RunPostgreSQL(t)
			ctx := graceful.NewContext(t.Context())
			cfg := config.Node{
				Nats: config.Nats{
					URL: container.RunNATS(t),
				},
			}

			jet, err := pubsub.New(ctx, cfg)
			require.NoError(t, err)

			err = consumer.Bootstrap(ctx, logger, jet, "")
			require.NoError(t, err)

			pub := pubsub.NewPublisher(jet, "")

			projects := management.NewProjectsStore(mgmt)
			projectID, err := projects.CreateProject(ctx, DefaultProject)
			require.NoError(t, err)

			usersStore := subjects.NewUsersStore(usrs)
			listsStore := subjects.NewListsStore(usrs)

			list := subjects.List{
				ProjectID: projectID,
				Name:      "Import Test List",
				Type:      subjects.ListTypeStatic,
			}

			listID, err := listsStore.CreateList(ctx, list)
			require.NoError(t, err)

			actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
				rbac.WithOrganizationID(uuid.New()),
				rbac.WithProjectID(projectID),
			)
			engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

			controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			header := textproto.MIMEHeader{}
			header.Set("Content-Disposition", `form-data; name="file"; filename="users.csv"`)
			header.Set("Content-Type", "text/csv")

			part, err := writer.CreatePart(header)
			require.NoError(t, err)

			_, err = part.Write([]byte(test.csv))
			require.NoError(t, err)

			err = writer.Close()
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", fmt.Sprintf("/v1/lists/%s/users", listID), body)
			req = req.WithContext(actorCtx)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			controller.ImportListUsers(res, req, projectID, listID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			users, total, err := usersStore.ListUsers(ctx, projectID, store.Pagination{Limit: 100, Offset: 0}, "")
			require.NoError(t, err)
			require.Equal(t, test.users, total, "expected %d users in total", test.users)
			require.Len(t, users, test.users, "expected %d users to be returned", test.users)
		})
	}
}

func TestCreateListWithOrganizationEvents(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	// Create a rule with both user events and organization events
	// This tests that events with the same name but different subject_types are handled correctly
	rule := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeWrapper,
			Group:    rules.RuleGroupParent,
			Operator: rules.OperatorAnd,
			Children: []rules.Rule{
				{
					Type:  rules.RuleTypeWrapper,
					Group: rules.RuleGroupEvent,
					Value: "purchase.completed", // user event
				},
				{
					Type:  rules.RuleTypeWrapper,
					Group: rules.RuleGroupOrganizationEvent,
					Value: "purchase.completed", // organization event - same name but different subject_type
				},
				{
					Type:  rules.RuleTypeWrapper,
					Group: rules.RuleGroupOrganizationEvent,
					Value: "subscription.upgraded", // another organization event
				},
			},
		},
	}

	body := oapi.CreateListJSONRequestBody{
		Name: "Dynamic List With Org Events",
		Type: oapi.CreateListTypeDynamic,
		Rule: &rule,
	}

	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/lists", bytes.NewReader(bb))
	req = req.WithContext(actorCtx)
	controller.CreateList(res, req, projectID)

	require.Equal(t, 201, res.Code, res.Body.String())

	var response oapi.List
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, body.Name, response.Name)
	require.NotNil(t, response.DraftRule, "draft rule should be created")

	// Verify events are created with correct subject types
	eventsStore := subjects.NewEventsStore(usrs)

	// Check user events
	userEvents, err := eventsStore.ListEventSchemas(ctx, projectID, subjects.SubjectTypeUser)
	require.NoError(t, err)
	require.Len(t, userEvents, 1, "should have 1 user event")
	require.Equal(t, "purchase.completed", userEvents[0].Name)
	require.Equal(t, subjects.SubjectTypeUser, userEvents[0].SubjectType)

	// Check organization events
	orgEvents, err := eventsStore.ListEventSchemas(ctx, projectID, subjects.SubjectTypeOrganization)
	require.NoError(t, err)
	require.Len(t, orgEvents, 2, "should have 2 organization events")

	orgEventNames := make(map[string]bool)
	for _, e := range orgEvents {
		orgEventNames[e.Name] = true
		require.Equal(t, subjects.SubjectTypeOrganization, e.SubjectType)
	}
	require.True(t, orgEventNames["purchase.completed"], "should have purchase.completed org event")
	require.True(t, orgEventNames["subscription.upgraded"], "should have subscription.upgraded org event")

	// Verify rules_events dependencies are created correctly by getting the draft version's rule
	listsStore := subjects.NewListsStore(usrs)
	draftVersion, err := listsStore.GetDraftVersion(ctx, response.Id)
	require.NoError(t, err)
	require.NotNil(t, draftVersion.RuleID, "draft version should have a rule_id")

	rulesStore := subjects.NewRulesStore(usrs)
	ruleData, err := rulesStore.GetRule(ctx, projectID, *draftVersion.RuleID)
	require.NoError(t, err)
	require.Len(t, ruleData.Events, 3, "should have 3 event dependencies (1 user + 2 org)")

	// Verify that the events are correctly linked (both user and org events with same name should have different IDs)
	eventIDSet := make(map[string]bool)
	for _, eventID := range ruleData.Events {
		eventIDSet[eventID.String()] = true
	}
	require.Len(t, eventIDSet, 3, "all 3 event IDs should be unique")
}

func TestUpdateListWithOrganizationEvents(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	// Create an initial dynamic list without a rule
	listsStore := subjects.NewListsStore(usrs)
	listID, err := listsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Test Dynamic List",
		Type:      "dynamic",
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	// Update the list with a rule containing organization events
	rule := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeWrapper,
			Group:    rules.RuleGroupParent,
			Operator: rules.OperatorOr,
			Children: []rules.Rule{
				{
					Type:  rules.RuleTypeWrapper,
					Group: rules.RuleGroupOrganizationEvent,
					Value: "org.plan.upgraded",
				},
				{
					Type:  rules.RuleTypeWrapper,
					Group: rules.RuleGroupEvent,
					Value: "user.premium.activated",
				},
			},
		},
	}

	body := oapi.UpdateListJSONRequestBody{
		Name: "Updated Dynamic List",
		Rule: &rule,
	}

	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/v1/lists/"+listID.String(), bytes.NewReader(bb))
	req = req.WithContext(actorCtx)
	controller.UpdateList(res, req, projectID, listID)

	require.Equal(t, 200, res.Code, res.Body.String())

	var response oapi.List
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.NotNil(t, response.DraftRule, "draft rule should be created")

	// Verify events are created with correct subject types
	eventsStore := subjects.NewEventsStore(usrs)

	// Check user events
	userEvents, err := eventsStore.ListEventSchemas(ctx, projectID, subjects.SubjectTypeUser)
	require.NoError(t, err)
	require.Len(t, userEvents, 1, "should have 1 user event")
	require.Equal(t, "user.premium.activated", userEvents[0].Name)

	// Check organization events
	orgEvents, err := eventsStore.ListEventSchemas(ctx, projectID, subjects.SubjectTypeOrganization)
	require.NoError(t, err)
	require.Len(t, orgEvents, 1, "should have 1 organization event")
	require.Equal(t, "org.plan.upgraded", orgEvents[0].Name)

	// Verify rules_events dependencies via the draft version
	listsStoreForUpdate := subjects.NewListsStore(usrs)
	draftVersionForUpdate, err := listsStoreForUpdate.GetDraftVersion(ctx, listID)
	require.NoError(t, err)
	require.NotNil(t, draftVersionForUpdate.RuleID, "draft version should have a rule_id")

	rulesStore := subjects.NewRulesStore(usrs)
	ruleData, err := rulesStore.GetRule(ctx, projectID, *draftVersionForUpdate.RuleID)
	require.NoError(t, err)
	require.Len(t, ruleData.Events, 2, "should have 2 event dependencies")
}

func TestCreateDynamicListWithRule_IsDraft(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	rule := rules.RuleSet{
		Rule: rules.Rule{
			UUID:     uuid.New(),
			Path:     "user.data.age",
			Group:    rules.RuleGroupUser,
			Type:     rules.RuleTypeNumber,
			Operator: rules.OperatorGreaterEqual,
			Value:    float64(18),
		},
	}

	body := oapi.CreateListJSONRequestBody{
		Name: "Dynamic Draft List",
		Type: oapi.CreateListTypeDynamic,
		Rule: &rule,
	}

	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/lists", bytes.NewReader(bb))
	req = req.WithContext(actorCtx)
	controller.CreateList(res, req, projectID)

	require.Equal(t, 201, res.Code, res.Body.String())

	var response oapi.List
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)

	// New dynamic lists with a rule should start as draft
	require.Equal(t, oapi.ListStateDraft, response.State)
	require.NotNil(t, response.DraftRule, "draft_rule should be populated")
	require.Nil(t, response.Rule, "published rule should be nil for draft list")
}

func TestUpdateListSavesRuleToDraft(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	// Create a dynamic list without a rule
	listsStore := subjects.NewListsStore(usrs)
	listID, err := listsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Dynamic List",
		Type:      "dynamic",
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	// Update with a rule — should save to draft version
	rule := rules.RuleSet{
		Rule: rules.Rule{
			UUID:     uuid.New(),
			Path:     "user.data.name",
			Group:    rules.RuleGroupUser,
			Type:     rules.RuleTypeString,
			Operator: rules.OperatorEquals,
			Value:    "Alice",
		},
	}

	body := oapi.UpdateListJSONRequestBody{
		Name: "Updated Dynamic List",
		Rule: &rule,
	}

	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/v1/lists/"+listID.String(), bytes.NewReader(bb))
	req = req.WithContext(actorCtx)
	controller.UpdateList(res, req, projectID, listID)

	require.Equal(t, 200, res.Code, res.Body.String())

	var response oapi.List
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)

	require.Equal(t, oapi.ListStateDraft, response.State)
	require.NotNil(t, response.DraftRule, "draft_rule should be populated after update with rule")
	require.Nil(t, response.Rule, "published rule should be nil — not published yet")
}

func TestUpdateListPublish(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	// Create a dynamic list with a rule (starts as draft)
	rule := rules.RuleSet{
		Rule: rules.Rule{
			UUID:     uuid.New(),
			Path:     "user.data.age",
			Group:    rules.RuleGroupUser,
			Type:     rules.RuleTypeNumber,
			Operator: rules.OperatorGreaterEqual,
			Value:    float64(21),
		},
	}

	createBody := oapi.CreateListJSONRequestBody{
		Name: "Publishable List",
		Type: oapi.CreateListTypeDynamic,
		Rule: &rule,
	}

	bb, err := json.Marshal(createBody)
	require.NoError(t, err)

	createRes := httptest.NewRecorder()
	createReq := httptest.NewRequest("POST", "/v1/lists", bytes.NewReader(bb))
	createReq = createReq.WithContext(actorCtx)
	controller.CreateList(createRes, createReq, projectID)

	require.Equal(t, 201, createRes.Code, createRes.Body.String())

	var created oapi.List
	err = json.Unmarshal(createRes.Body.Bytes(), &created)
	require.NoError(t, err)
	require.Equal(t, oapi.ListStateDraft, created.State)

	// Now publish the list
	updateBody := oapi.UpdateListJSONRequestBody{
		Name:      "Publishable List",
		Published: ptr(true),
	}

	bb, err = json.Marshal(updateBody)
	require.NoError(t, err)

	updateRes := httptest.NewRecorder()
	updateReq := httptest.NewRequest("PATCH", "/v1/lists/"+created.Id.String(), bytes.NewReader(bb))
	updateReq = updateReq.WithContext(actorCtx)
	controller.UpdateList(updateRes, updateReq, projectID, created.Id)

	require.Equal(t, 200, updateRes.Code, updateRes.Body.String())

	var published oapi.List
	err = json.Unmarshal(updateRes.Body.Bytes(), &published)
	require.NoError(t, err)

	require.Equal(t, oapi.ListStateReady, published.State, "state should be 'ready' after publish")
	require.NotNil(t, published.Rule, "published rule should be populated after publish")
	require.Nil(t, published.DraftRule, "draft_rule should be nil after publish (no pending draft)")
}

func TestPreviewListUsers(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	// Create users with different data
	usersStore := subjects.NewUsersStore(usrs)
	_, err = usersStore.CreateUser(ctx, projectID, ptr("alice@example.com"), nil, []byte(`{"name":"Alice","age":30}`), nil, nil, []subjects.ExternalIDParam{{Source: "default", ExternalID: "alice"}})
	require.NoError(t, err)

	_, err = usersStore.CreateUser(ctx, projectID, ptr("bob@example.com"), nil, []byte(`{"name":"Bob","age":17}`), nil, nil, []subjects.ExternalIDParam{{Source: "default", ExternalID: "bob"}})
	require.NoError(t, err)

	_, err = usersStore.CreateUser(ctx, projectID, ptr("carol@example.com"), nil, []byte(`{"name":"Carol","age":25}`), nil, nil, []subjects.ExternalIDParam{{Source: "default", ExternalID: "carol"}})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	// Create a dynamic list with a rule: age >= 18
	rule := rules.RuleSet{
		Rule: rules.Rule{
			UUID:     uuid.New(),
			Path:     "user.data.age",
			Group:    rules.RuleGroupUser,
			Type:     rules.RuleTypeNumber,
			Operator: rules.OperatorGreaterEqual,
			Value:    float64(18),
		},
	}

	createBody := oapi.CreateListJSONRequestBody{
		Name: "Preview Test List",
		Type: oapi.CreateListTypeDynamic,
		Rule: &rule,
	}

	bb, err := json.Marshal(createBody)
	require.NoError(t, err)

	createRes := httptest.NewRecorder()
	createReq := httptest.NewRequest("POST", "/v1/lists", bytes.NewReader(bb))
	createReq = createReq.WithContext(actorCtx)
	controller.CreateList(createRes, createReq, projectID)

	require.Equal(t, 201, createRes.Code, createRes.Body.String())

	var created oapi.List
	err = json.Unmarshal(createRes.Body.Bytes(), &created)
	require.NoError(t, err)
	require.Equal(t, oapi.ListStateDraft, created.State)
	require.NotNil(t, created.DraftRule)

	t.Run("returns matching users for draft rule", func(t *testing.T) {
		limit := oapi.Limit(25)
		params := oapi.PreviewListUsersParams{
			Limit: &limit,
		}

		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", fmt.Sprintf("/v1/lists/%s/users/preview", created.Id), nil)
		req = req.WithContext(actorCtx)
		controller.PreviewListUsers(res, req, projectID, created.Id, params)

		require.Equal(t, 200, res.Code, res.Body.String())

		var response oapi.UserList
		err := json.Unmarshal(res.Body.Bytes(), &response)
		require.NoError(t, err)
		require.Equal(t, 2, response.Total, "should match Alice and Carol (age >= 18)")
		require.Len(t, response.Results, 2)

		extIDs := make(map[string]bool)
		for _, u := range response.Results {
			for _, ident := range u.Identifier {
				if ident.Source == "default" {
					extIDs[ident.ExternalId] = true
				}
			}
		}
		require.True(t, extIDs["alice"])
		require.True(t, extIDs["carol"])
	})

	t.Run("fails for static list", func(t *testing.T) {
		staticBody := oapi.CreateListJSONRequestBody{
			Name: "Static List",
			Type: oapi.CreateListTypeStatic,
		}

		bb, err := json.Marshal(staticBody)
		require.NoError(t, err)

		staticRes := httptest.NewRecorder()
		staticReq := httptest.NewRequest("POST", "/v1/lists", bytes.NewReader(bb))
		staticReq = staticReq.WithContext(actorCtx)
		controller.CreateList(staticRes, staticReq, projectID)
		require.Equal(t, 201, staticRes.Code)

		var staticList oapi.List
		err = json.Unmarshal(staticRes.Body.Bytes(), &staticList)
		require.NoError(t, err)

		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", fmt.Sprintf("/v1/lists/%s/users/preview", staticList.Id), nil)
		req = req.WithContext(actorCtx)
		controller.PreviewListUsers(res, req, projectID, staticList.Id, oapi.PreviewListUsersParams{})

		require.Equal(t, 400, res.Code, "should reject preview for static list")
	})

	t.Run("fails for list with no draft rule", func(t *testing.T) {
		noDraftBody := oapi.CreateListJSONRequestBody{
			Name: "No Draft Rule",
			Type: oapi.CreateListTypeDynamic,
		}

		bb, err := json.Marshal(noDraftBody)
		require.NoError(t, err)

		noDraftRes := httptest.NewRecorder()
		noDraftReq := httptest.NewRequest("POST", "/v1/lists", bytes.NewReader(bb))
		noDraftReq = noDraftReq.WithContext(actorCtx)
		controller.CreateList(noDraftRes, noDraftReq, projectID)
		require.Equal(t, 201, noDraftRes.Code)

		var noDraftList oapi.List
		err = json.Unmarshal(noDraftRes.Body.Bytes(), &noDraftList)
		require.NoError(t, err)

		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", fmt.Sprintf("/v1/lists/%s/users/preview", noDraftList.Id), nil)
		req = req.WithContext(actorCtx)
		controller.PreviewListUsers(res, req, projectID, noDraftList.Id, oapi.PreviewListUsersParams{})

		require.Equal(t, 400, res.Code, "should reject preview when no draft rule exists")
	})

	t.Run("returns 404 for non-existent list", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/v1/lists/"+uuid.New().String()+"/users/preview", nil)
		req = req.WithContext(actorCtx)
		controller.PreviewListUsers(res, req, projectID, uuid.New(), oapi.PreviewListUsersParams{})

		require.Equal(t, 404, res.Code, "should return 404 for non-existent list")
	})
}

func TestDuplicatePublishedList_CreatesDraft(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewListsController(logger, usrs, projects, pub, testMaxUploadSize, engine)

	// Create and publish a dynamic list
	rule := rules.RuleSet{
		Rule: rules.Rule{
			UUID:     uuid.New(),
			Path:     "user.data.score",
			Group:    rules.RuleGroupUser,
			Type:     rules.RuleTypeNumber,
			Operator: rules.OperatorGreaterEqual,
			Value:    float64(100),
		},
	}

	createBody := oapi.CreateListJSONRequestBody{
		Name: "Original Published List",
		Type: oapi.CreateListTypeDynamic,
		Rule: &rule,
	}

	bb, err := json.Marshal(createBody)
	require.NoError(t, err)

	createRes := httptest.NewRecorder()
	createReq := httptest.NewRequest("POST", "/v1/lists", bytes.NewReader(bb))
	createReq = createReq.WithContext(actorCtx)
	controller.CreateList(createRes, createReq, projectID)
	require.Equal(t, 201, createRes.Code, createRes.Body.String())

	var created oapi.List
	err = json.Unmarshal(createRes.Body.Bytes(), &created)
	require.NoError(t, err)

	// Publish the list
	publishBody := oapi.UpdateListJSONRequestBody{
		Name:      "Original Published List",
		Published: ptr(true),
	}

	bb, err = json.Marshal(publishBody)
	require.NoError(t, err)

	publishRes := httptest.NewRecorder()
	publishReq := httptest.NewRequest("PATCH", "/v1/lists/"+created.Id.String(), bytes.NewReader(bb))
	publishReq = publishReq.WithContext(actorCtx)
	controller.UpdateList(publishRes, publishReq, projectID, created.Id)
	require.Equal(t, 200, publishRes.Code, publishRes.Body.String())

	// Duplicate the published list
	dupRes := httptest.NewRecorder()
	dupReq := httptest.NewRequest("POST", "/v1/lists/"+created.Id.String()+"/duplicate", nil)
	dupReq = dupReq.WithContext(actorCtx)
	controller.DuplicateList(dupRes, dupReq, projectID, created.Id)

	require.Equal(t, 201, dupRes.Code, dupRes.Body.String())

	var duplicated oapi.List
	err = json.Unmarshal(dupRes.Body.Bytes(), &duplicated)
	require.NoError(t, err)

	require.NotEqual(t, created.Id, duplicated.Id)
	require.Equal(t, "Copy of Original Published List", duplicated.Name)
	// Duplicated list should start as draft
	require.Equal(t, oapi.ListStateDraft, duplicated.State, "duplicated list should be draft")
	require.NotNil(t, duplicated.DraftRule, "duplicated list should have a draft_rule (copied from published)")
	require.Nil(t, duplicated.Rule, "duplicated list should not have a published rule")
}

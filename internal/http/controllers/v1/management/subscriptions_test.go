package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestSubscriptionCreation(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := management.Config{
		URI: container.RunPostgreSQL(t),
	}

	err := management.Migrate(config)
	require.NoError(t, err)

	db, err := management.New(ctx, logger, config)
	require.NoError(t, err)

	projects := management.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	subs := NewSubscriptionsController(logger, db)

	isPublicTrue := true
	isPublicFalse := false

	type test struct {
		body oapi.CreateSubscriptionJSONRequestBody
		code int
	}

	tests := map[string]test{
		"email-subscription": {
			body: oapi.CreateSubscriptionJSONRequestBody{
				Name:     "Marketing Newsletter",
				Channel:  oapi.Channel("email"),
				IsPublic: &isPublicTrue,
			},
			code: 201,
		},
		"sms-subscription": {
			body: oapi.CreateSubscriptionJSONRequestBody{
				Name:     "Order Updates",
				Channel:  oapi.Channel("text"),
				IsPublic: &isPublicTrue,
			},
			code: 201,
		},
		"private-subscription": {
			body: oapi.CreateSubscriptionJSONRequestBody{
				Name:     "Internal Alerts",
				Channel:  oapi.Channel("push"),
				IsPublic: &isPublicFalse,
			},
			code: 201,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/subscriptions", bytes.NewReader(bb))
			subs.CreateSubscription(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 201 {
				var result oapi.Subscription
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, test.body.Name, result.Name)
				require.Equal(t, test.body.Channel, result.Channel)
				require.Equal(t, projectID, result.ProjectId)
				require.NotEqual(t, uuid.Nil, result.Id)
				if test.body.IsPublic != nil {
					require.Equal(t, *test.body.IsPublic, result.IsPublic)
				}
			}
		})
	}
}

func TestListSubscriptions(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := management.Config{
		URI: container.RunPostgreSQL(t),
	}

	err := management.Migrate(config)
	require.NoError(t, err)

	db, err := management.New(ctx, logger, config)
	require.NoError(t, err)

	projects := management.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	subs := NewSubscriptionsController(logger, db)

	// Create some test subscriptions
	isPublic := true
	for i, name := range []string{"Newsletter", "Updates", "Alerts"} {
		body := oapi.CreateSubscriptionJSONRequestBody{
			Name:     name,
			Channel:  oapi.Channel("email"),
			IsPublic: &isPublic,
		}

		bb, err := json.Marshal(body)
		require.NoError(t, err)

		res := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/v1/subscriptions", bytes.NewReader(bb))
		subs.CreateSubscription(res, req, projectID)
		require.Equal(t, 201, res.Code, "failed to create subscription %d", i)
	}

	// List subscriptions
	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/subscriptions?limit=10&offset=0", nil)

	params := oapi.ListSubscriptionsParams{
		Limit:  ptr(oapi.PaginationLimit(10)),
		Offset: ptr(oapi.PaginationOffset(0)),
	}

	subs.ListSubscriptions(res, req, projectID, params)
	require.Equal(t, 200, res.Code, res.Body.String())

	var result oapi.SubscriptionListResponse
	err = json.Unmarshal(res.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Len(t, result.Results, 3)
	require.Equal(t, 3, result.Total)
}

func TestUpdateSubscription(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := management.Config{
		URI: container.RunPostgreSQL(t),
	}

	err := management.Migrate(config)
	require.NoError(t, err)

	db, err := management.New(ctx, logger, config)
	require.NoError(t, err)

	projects := management.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	subs := NewSubscriptionsController(logger, db)

	// Create a subscription
	isPublic := true
	createBody := oapi.CreateSubscriptionJSONRequestBody{
		Name:     "Original Name",
		Channel:  oapi.Channel("email"),
		IsPublic: &isPublic,
	}

	bb, err := json.Marshal(createBody)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/subscriptions", bytes.NewReader(bb))
	subs.CreateSubscription(res, req, projectID)
	require.Equal(t, 201, res.Code)

	var created oapi.Subscription
	err = json.Unmarshal(res.Body.Bytes(), &created)
	require.NoError(t, err)

	// Update the subscription
	updateBody := oapi.UpdateSubscriptionJSONRequestBody{
		Name:     "Updated Name",
		IsPublic: false,
	}

	bb, err = json.Marshal(updateBody)
	require.NoError(t, err)

	res = httptest.NewRecorder()
	req = httptest.NewRequest("PATCH", "/v1/subscriptions/"+created.Id.String(), bytes.NewReader(bb))
	subs.UpdateSubscription(res, req, projectID, created.Id)
	require.Equal(t, 200, res.Code, res.Body.String())

	var updated oapi.Subscription
	err = json.Unmarshal(res.Body.Bytes(), &updated)
	require.NoError(t, err)
	require.Equal(t, "Updated Name", updated.Name)
	require.Equal(t, false, updated.IsPublic)
	require.Equal(t, created.Id, updated.Id)
}

func TestGetSubscription(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := management.Config{
		URI: container.RunPostgreSQL(t),
	}

	err := management.Migrate(config)
	require.NoError(t, err)

	db, err := management.New(ctx, logger, config)
	require.NoError(t, err)

	projects := management.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	subs := NewSubscriptionsController(logger, db)

	// Create a subscription
	isPublic := true
	createBody := oapi.CreateSubscriptionJSONRequestBody{
		Name:     "Test Subscription",
		Channel:  oapi.Channel("email"),
		IsPublic: &isPublic,
	}

	bb, err := json.Marshal(createBody)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/subscriptions", bytes.NewReader(bb))
	subs.CreateSubscription(res, req, projectID)
	require.Equal(t, 201, res.Code)

	var created oapi.Subscription
	err = json.Unmarshal(res.Body.Bytes(), &created)
	require.NoError(t, err)

	// Get the subscription
	res = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/v1/subscriptions/"+created.Id.String(), nil)
	subs.GetSubscription(res, req, projectID, created.Id)
	require.Equal(t, 200, res.Code, res.Body.String())

	var retrieved oapi.Subscription
	err = json.Unmarshal(res.Body.Bytes(), &retrieved)
	require.NoError(t, err)
	require.Equal(t, created.Id, retrieved.Id)
	require.Equal(t, created.Name, retrieved.Name)
	require.Equal(t, created.Channel, retrieved.Channel)

	// Test not found
	res = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/v1/subscriptions/"+uuid.New().String(), nil)
	subs.GetSubscription(res, req, projectID, uuid.New())
	require.Equal(t, 404, res.Code)
}

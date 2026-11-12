//go:build enterprise

package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// broadcastTestEnv holds all state needed to exercise broadcast endpoints.
type broadcastTestEnv struct {
	controller      *BroadcastsController
	projectID       uuid.UUID
	campaignID      uuid.UUID
	listID          uuid.UUID
	broadcastsStore *management.BroadcastsStore
	mgmtState       *management.State
	actorCtx        func(r *http.Request) *http.Request
}

func newBroadcastTestEnv(t *testing.T) broadcastTestEnv {
	t.Helper()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	natsURL := container.RunNATS(t)
	mgmtDB, usrsDB, _ := teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Nats: config.Nats{
			URL: natsURL,
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	mgmtState := management.NewState(mgmtDB)
	usrsState := subjects.NewState(usrsDB, zap.NewNop())

	orgID, err := mgmtState.OrganizationsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := mgmtState.ProjectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	// A provider must exist for the channel so the broadcast can resolve one
	// at send time; campaigns no longer reference a provider directly.
	_, err = mgmtState.ProvidersStore.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "test",
		Channels:  management.Channels{"email"},
		Data:      json.RawMessage(`{}`),
		Name:      "Test Provider",
	})
	require.NoError(t, err)

	campaignID, err := mgmtState.CampaignsStore.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Broadcast Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	listID, err := usrsState.ListsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Broadcast List",
		Type:      "static",
	})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		email := ptr.To("user" + uuid.New().String() + "@test.com")
		_, err := usrsState.UsersStore.CreateUser(ctx, projectID, email, nil, json.RawMessage(`{}`), nil, nil, nil)
		require.NoError(t, err)
	}

	ns := consumer.Namespace("test_broadcasts")

	err = consumer.Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewBroadcastsController(logger, mgmtDB, usrsDB, pub, jet, engine, ns)

	return broadcastTestEnv{
		controller:      controller,
		projectID:       projectID,
		campaignID:      campaignID,
		listID:          listID,
		broadcastsStore: mgmtState.BroadcastsStore,
		mgmtState:       mgmtState,
		actorCtx: func(r *http.Request) *http.Request {
			return r.WithContext(actorCtx)
		},
	}
}

func TestCreateBroadcast(t *testing.T) {
	t.Parallel()
	env := newBroadcastTestEnv(t)

	tests := map[string]struct {
		body oapi.CreateBroadcastJSONRequestBody
		code int
	}{
		"success pending": {
			body: oapi.CreateBroadcastJSONRequestBody{
				CampaignId: env.campaignID,
				ListId:     env.listID,
			},
			code: 201,
		},
		"success scheduled": {
			body: oapi.CreateBroadcastJSONRequestBody{
				CampaignId:  env.campaignID,
				ListId:      env.listID,
				ScheduledAt: ptr.To(time.Now().Add(24 * time.Hour)),
			},
			code: 201,
		},
		"scheduled_at in the past": {
			body: oapi.CreateBroadcastJSONRequestBody{
				CampaignId:  env.campaignID,
				ListId:      env.listID,
				ScheduledAt: ptr.To(time.Now().Add(-1 * time.Hour)),
			},
			code: 400,
		},
		"campaign not found": {
			body: oapi.CreateBroadcastJSONRequestBody{
				CampaignId: uuid.New(),
				ListId:     env.listID,
			},
			code: 404,
		},
		"list not found": {
			body: oapi.CreateBroadcastJSONRequestBody{
				CampaignId: env.campaignID,
				ListId:     uuid.New(),
			},
			code: 404,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(tc.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/broadcasts", bytes.NewReader(bb))
			req = env.actorCtx(req)
			env.controller.CreateBroadcast(res, req, env.projectID)

			require.Equal(t, tc.code, res.Code, res.Body.String())

			if tc.code == 201 {
				var broadcast oapi.Broadcast
				err := json.Unmarshal(res.Body.Bytes(), &broadcast)
				require.NoError(t, err)

				require.Equal(t, env.projectID, broadcast.ProjectId)
				require.Equal(t, env.campaignID, broadcast.CampaignId)
				require.Equal(t, env.listID, broadcast.ListId)
				require.NotEqual(t, uuid.Nil, broadcast.Id)

				if tc.body.ScheduledAt != nil {
					require.Equal(t, oapi.BroadcastState("scheduled"), broadcast.State)
					require.NotNil(t, broadcast.ScheduledAt)
				} else {
					require.Equal(t, oapi.BroadcastState("pending"), broadcast.State)
				}

				// Verify campaign information is included.
				require.NotNil(t, broadcast.Campaign)
				require.NotNil(t, broadcast.Campaign.Name)
				require.Equal(t, "Broadcast Campaign", *broadcast.Campaign.Name)
			}
		})
	}
}

// TestCreateBroadcastCampaignWithoutProvider verifies that a broadcast can be
// created for a campaign that has no provider configured. Providers are owned
// by the project and resolved per message at dispatch time, so provider
// configuration is not a precondition for creating a broadcast.
func TestCreateBroadcastCampaignWithoutProvider(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	natsURL := container.RunNATS(t)
	mgmtDB, usrsDB, _ := teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Nats: config.Nats{
			URL: natsURL,
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	mgmtState := management.NewState(mgmtDB)
	usrsState := subjects.NewState(usrsDB, zap.NewNop())

	orgID, err := mgmtState.OrganizationsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := mgmtState.ProjectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	// Campaign without a provider.
	campaignID, err := mgmtState.CampaignsStore.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "No Provider Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	listID, err := usrsState.ListsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "static",
	})
	require.NoError(t, err)

	ns := consumer.Namespace("test_no_provider")
	pub := pubsub.NewPublisher(jet, string(ns))

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewBroadcastsController(logger, mgmtDB, usrsDB, pub, jet, engine, ns)

	body := oapi.CreateBroadcastJSONRequestBody{
		CampaignId: campaignID,
		ListId:     listID,
	}
	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/broadcasts", bytes.NewReader(bb))
	req = req.WithContext(actorCtx)
	controller.CreateBroadcast(res, req, projectID)

	require.Equal(t, 201, res.Code, res.Body.String())

	var broadcast oapi.Broadcast
	err = json.Unmarshal(res.Body.Bytes(), &broadcast)
	require.NoError(t, err)

	require.Equal(t, projectID, broadcast.ProjectId)
	require.Equal(t, campaignID, broadcast.CampaignId)
	require.Equal(t, listID, broadcast.ListId)
	require.Equal(t, oapi.BroadcastState("pending"), broadcast.State)
}

func TestListBroadcasts(t *testing.T) {
	t.Parallel()
	env := newBroadcastTestEnv(t)

	ctx := t.Context()

	for i := 0; i < 3; i++ {
		_, err := env.broadcastsStore.CreateBroadcast(ctx, management.Broadcast{
			ProjectID:  env.projectID,
			CampaignID: env.campaignID,
			ListID:     env.listID,
			ListName:   "Broadcast List",
			ListType:   "static",
		})
		require.NoError(t, err)
	}

	tests := map[string]struct {
		limit  int
		offset int
		total  int
		count  int
	}{
		"default": {
			limit: 20, offset: 0, total: 3, count: 3,
		},
		"with limit": {
			limit: 2, offset: 0, total: 3, count: 2,
		},
		"with offset": {
			limit: 20, offset: 1, total: 3, count: 2,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			limit := oapi.Limit(tc.limit)
			offset := oapi.Offset(tc.offset)

			params := oapi.ListBroadcastsParams{
				Limit:  &limit,
				Offset: &offset,
			}

			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/broadcasts", nil)
			req = env.actorCtx(req)
			env.controller.ListBroadcasts(res, req, env.projectID, params)

			require.Equal(t, 200, res.Code, res.Body.String())

			var response oapi.BroadcastListResponse
			err := json.Unmarshal(res.Body.Bytes(), &response)
			require.NoError(t, err)

			require.Equal(t, tc.total, response.Total)
			require.Len(t, response.Results, tc.count)
		})
	}
}

func TestGetBroadcast(t *testing.T) {
	t.Parallel()
	env := newBroadcastTestEnv(t)

	ctx := t.Context()
	broadcast, err := env.broadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  env.projectID,
		CampaignID: env.campaignID,
		ListID:     env.listID,
		ListName:   "Broadcast List",
		ListType:   "static",
	})
	require.NoError(t, err)

	tests := map[string]struct {
		id   uuid.UUID
		code int
	}{
		"found": {
			id:   broadcast.ID,
			code: 200,
		},
		"not found": {
			id:   uuid.New(),
			code: 404,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/broadcasts/"+tc.id.String(), nil)
			req = env.actorCtx(req)
			env.controller.GetBroadcast(res, req, env.projectID, tc.id)

			require.Equal(t, tc.code, res.Code, res.Body.String())

			if tc.code == 200 {
				var result oapi.Broadcast
				err := json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, broadcast.ID, result.Id)
				require.Equal(t, oapi.BroadcastState("pending"), result.State)
			}
		})
	}
}

func TestSendBroadcast(t *testing.T) {
	t.Parallel()
	env := newBroadcastTestEnv(t)

	ctx := t.Context()

	// Create a pending broadcast that can be sent.
	broadcast, err := env.broadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  env.projectID,
		CampaignID: env.campaignID,
		ListID:     env.listID,
		ListName:   "Broadcast List",
		ListType:   "static",
	})
	require.NoError(t, err)

	// Create a second broadcast that we'll cancel so it's in a non-pending state.
	cancelledBroadcast, err := env.broadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  env.projectID,
		CampaignID: env.campaignID,
		ListID:     env.listID,
		ListName:   "Broadcast List",
		ListType:   "static",
	})
	require.NoError(t, err)
	_, err = env.mgmtState.CancelBroadcast(ctx, env.projectID, cancelledBroadcast.ID)
	require.NoError(t, err)

	tests := map[string]struct {
		id   uuid.UUID
		code int
	}{
		"success": {
			id:   broadcast.ID,
			code: 200,
		},
		"not pending": {
			id:   cancelledBroadcast.ID,
			code: 400,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/broadcasts/"+tc.id.String()+"/send", nil)
			req = env.actorCtx(req)
			env.controller.SendBroadcast(res, req, env.projectID, tc.id)

			require.Equal(t, tc.code, res.Code, res.Body.String())

			if tc.code == 200 {
				var result oapi.Broadcast
				err := json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, oapi.BroadcastState("sending"), result.State)
			}
		})
	}
}

// TestSendBroadcastDuplicatePrevented verifies that a broadcast can only be
// sent once — a second send attempt must fail.
func TestSendBroadcastDuplicatePrevented(t *testing.T) {
	t.Parallel()
	env := newBroadcastTestEnv(t)

	ctx := t.Context()
	broadcast, err := env.broadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  env.projectID,
		CampaignID: env.campaignID,
		ListID:     env.listID,
		ListName:   "Broadcast List",
		ListType:   "static",
	})
	require.NoError(t, err)

	// First send — should succeed.
	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/broadcasts/"+broadcast.ID.String()+"/send", nil)
	req = env.actorCtx(req)
	env.controller.SendBroadcast(res, req, env.projectID, broadcast.ID)
	require.Equal(t, 200, res.Code, res.Body.String())

	// Second send — should fail because it's already in sending state.
	res2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/v1/broadcasts/"+broadcast.ID.String()+"/send", nil)
	req2 = env.actorCtx(req2)
	env.controller.SendBroadcast(res2, req2, env.projectID, broadcast.ID)
	require.Equal(t, 400, res2.Code, res2.Body.String())
}

func TestUpdateBroadcast(t *testing.T) {
	t.Parallel()
	env := newBroadcastTestEnv(t)

	ctx := t.Context()

	pendingBroadcast, err := env.broadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  env.projectID,
		CampaignID: env.campaignID,
		ListID:     env.listID,
		ListName:   "Broadcast List",
		ListType:   "static",
	})
	require.NoError(t, err)

	// Create and cancel a broadcast so it's not in an updatable state.
	cancelledBroadcast, err := env.broadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  env.projectID,
		CampaignID: env.campaignID,
		ListID:     env.listID,
		ListName:   "Broadcast List",
		ListType:   "static",
	})
	require.NoError(t, err)
	_, err = env.mgmtState.CancelBroadcast(ctx, env.projectID, cancelledBroadcast.ID)
	require.NoError(t, err)

	futureTime := time.Now().Add(48 * time.Hour)

	tests := map[string]struct {
		id   uuid.UUID
		body oapi.UpdateBroadcastJSONRequestBody
		code int
	}{
		"set schedule": {
			id:   pendingBroadcast.ID,
			body: oapi.UpdateBroadcastJSONRequestBody{ScheduledAt: &futureTime},
			code: 200,
		},
		"not updatable": {
			id:   cancelledBroadcast.ID,
			body: oapi.UpdateBroadcastJSONRequestBody{ScheduledAt: &futureTime},
			code: 400,
		},
		"scheduled_at in the past": {
			id:   pendingBroadcast.ID,
			body: oapi.UpdateBroadcastJSONRequestBody{ScheduledAt: ptr.To(time.Now().Add(-1 * time.Hour))},
			code: 400,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(tc.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/v1/broadcasts/"+tc.id.String(), bytes.NewReader(bb))
			req = env.actorCtx(req)
			env.controller.UpdateBroadcast(res, req, env.projectID, tc.id)

			require.Equal(t, tc.code, res.Code, res.Body.String())

			if tc.code == 200 {
				var result oapi.Broadcast
				err := json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.NotNil(t, result.ScheduledAt)
				require.Equal(t, oapi.BroadcastState("scheduled"), result.State)
			}
		})
	}
}

func TestCancelBroadcast(t *testing.T) {
	t.Parallel()
	env := newBroadcastTestEnv(t)

	ctx := t.Context()

	pendingBroadcast, err := env.broadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  env.projectID,
		CampaignID: env.campaignID,
		ListID:     env.listID,
		ListName:   "Broadcast List",
		ListType:   "static",
	})
	require.NoError(t, err)

	// Create and send a broadcast so it's in sending state (not cancellable).
	sendingBroadcast, err := env.broadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  env.projectID,
		CampaignID: env.campaignID,
		ListID:     env.listID,
		ListName:   "Broadcast List",
		ListType:   "static",
	})
	require.NoError(t, err)
	err = env.broadcastsStore.TransitionPendingBroadcastToSending(ctx, sendingBroadcast.ID)
	require.NoError(t, err)

	tests := map[string]struct {
		id   uuid.UUID
		code int
	}{
		"success": {
			id:   pendingBroadcast.ID,
			code: 200,
		},
		"not cancellable (sending)": {
			id:   sendingBroadcast.ID,
			code: 400,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/broadcasts/"+tc.id.String()+"/cancel", nil)
			req = env.actorCtx(req)
			env.controller.CancelBroadcast(res, req, env.projectID, tc.id)

			require.Equal(t, tc.code, res.Code, res.Body.String())

			if tc.code == 200 {
				var result oapi.Broadcast
				err := json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, oapi.BroadcastState("cancelled"), result.State)
			}
		})
	}
}

func TestGetBroadcastUsers(t *testing.T) {
	t.Parallel()
	env := newBroadcastTestEnv(t)

	ctx := t.Context()

	broadcast, err := env.broadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  env.projectID,
		CampaignID: env.campaignID,
		ListID:     env.listID,
		ListName:   "Broadcast List",
		ListType:   "static",
	})
	require.NoError(t, err)

	tests := map[string]struct {
		id   uuid.UUID
		code int
	}{
		"found": {
			id:   broadcast.ID,
			code: 200,
		},
		"broadcast not found": {
			id:   uuid.New(),
			code: 404,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			limit := oapi.Limit(10)
			offset := oapi.Offset(0)

			params := oapi.GetBroadcastUsersParams{
				Limit:  &limit,
				Offset: &offset,
			}

			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/broadcasts/"+tc.id.String()+"/users", nil)
			req = env.actorCtx(req)
			env.controller.GetBroadcastUsers(res, req, env.projectID, tc.id, params)

			require.Equal(t, tc.code, res.Code, res.Body.String())

			if tc.code == 200 {
				var response map[string]any
				err := json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Contains(t, response, "total")
				require.Contains(t, response, "limit")
				require.Contains(t, response, "offset")
				require.Contains(t, response, "results")
			}
		})
	}
}

func TestIsTerminalState(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		state    management.BroadcastState
		terminal bool
	}{
		"pending":   {management.BroadcastStatePending, false},
		"scheduled": {management.BroadcastStateScheduled, false},
		"sending":   {management.BroadcastStateSending, false},
		"completed": {management.BroadcastStateCompleted, true},
		"failed":    {management.BroadcastStateFailed, true},
		"cancelled": {management.BroadcastStateCancelled, true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.terminal, isTerminalState(tc.state))
		})
	}
}

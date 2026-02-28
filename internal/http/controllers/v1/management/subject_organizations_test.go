package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/claim"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type testSubjectOrganizationsController struct {
	controller *SubjectOrganizationsController
	projectID  uuid.UUID
	usersDB    *subjects.State
}

func setupSubjectOrganizationsController(t *testing.T) *testSubjectOrganizationsController {
	t.Helper()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	gracefulCtx := graceful.NewContext(ctx)
	mgmtDB, usrsDB, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(gracefulCtx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(gracefulCtx, logger, jet)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)

	orgsStore := management.NewOrganizationsStore(mgmtDB)
	orgID, err := orgsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectsStore := management.NewProjectsStore(mgmtDB)
	projectID, err := projectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           DefaultProject.Name,
		Timezone:       DefaultProject.Timezone,
		Locale:         DefaultProject.Locale,
	})
	require.NoError(t, err)

	controller := NewSubjectOrganizationsController(logger, usrsDB, pub)
	usersState := subjects.NewState(usrsDB)

	return &testSubjectOrganizationsController{
		controller: controller,
		projectID:  projectID,
		usersDB:    usersState,
	}
}

func TestListOrganizations(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
			ExternalID: "org_" + uuid.New().String(),
			Name:       ptr("Test Org"),
		})
		require.NoError(t, err)
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.ListOrganizations(res, req, tc.projectID, oapi.ListOrganizationsParams{})

	require.Equal(t, 200, res.Code)

	var response oapi.OrganizationList
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 5, response.Total)
	require.Len(t, response.Results, 5)
}

func TestListOrganizationsPagination(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
			ExternalID: "org_" + uuid.New().String(),
			Name:       ptr("Test Org"),
		})
		require.NoError(t, err)
	}

	limit := 3
	offset := 2

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.ListOrganizations(res, req, tc.projectID, oapi.ListOrganizationsParams{
		Limit:  (*oapi.Limit)(&limit),
		Offset: (*oapi.Offset)(&offset),
	})

	require.Equal(t, 200, res.Code)

	var response oapi.OrganizationList
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 10, response.Total)
	require.Len(t, response.Results, 3)
	require.Equal(t, 3, response.Limit)
	require.Equal(t, 2, response.Offset)
}

func TestListOrganizationsUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations", nil)

	tc.controller.ListOrganizations(res, req, tc.projectID, oapi.ListOrganizationsParams{})

	require.Equal(t, 401, res.Code)
}

func TestUpsertOrganization(t *testing.T) {
	t.Parallel()

	type test struct {
		body       oapi.UpsertOrganization
		statusCode int
	}

	tests := map[string]test{
		"create with all fields": {
			body: oapi.UpsertOrganization{
				ExternalId: "org_123",
				Name:       ptr("Acme Corp"),
				Data: &map[string]any{
					"industry": "technology",
					"size":     "enterprise",
				},
			},
			statusCode: 200,
		},
		"create with minimal data": {
			body: oapi.UpsertOrganization{
				ExternalId: "org_456",
			},
			statusCode: 200,
		},
		"create with name only": {
			body: oapi.UpsertOrganization{
				ExternalId: "org_789",
				Name:       ptr("Simple Corp"),
			},
			statusCode: 200,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tc := setupSubjectOrganizationsController(t)

			body, err := json.Marshal(tt.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/api/admin/projects/"+tc.projectID.String()+"/organizations", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(claim.WithSession(req.Context(), validSession()))

			tc.controller.UpsertOrganization(res, req, tc.projectID)

			require.Equal(t, tt.statusCode, res.Code, res.Body.String())

			if res.Code == 200 {
				var response oapi.Organization
				err = json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)
				require.NotEmpty(t, response.Id)
				require.Equal(t, tt.body.ExternalId, response.ExternalId)
			}
		})
	}
}

func TestUpsertOrganizationUpdate(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)

	// First upsert - create
	body1 := oapi.UpsertOrganization{
		ExternalId: "org_123",
		Name:       ptr("Original Name"),
		Data: &map[string]any{
			"plan": "basic",
		},
	}

	bodyBytes1, err := json.Marshal(body1)
	require.NoError(t, err)

	res1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("POST", "/api/admin/projects/"+tc.projectID.String()+"/organizations", bytes.NewReader(bodyBytes1))
	req1.Header.Set("Content-Type", "application/json")
	req1 = req1.WithContext(claim.WithSession(req1.Context(), validSession()))

	tc.controller.UpsertOrganization(res1, req1, tc.projectID)
	require.Equal(t, 200, res1.Code)

	var response1 oapi.Organization
	err = json.Unmarshal(res1.Body.Bytes(), &response1)
	require.NoError(t, err)
	orgID := response1.Id

	// Second upsert - update
	body2 := oapi.UpsertOrganization{
		ExternalId: "org_123",
		Name:       ptr("Updated Name"),
		Data: &map[string]any{
			"plan": "enterprise",
		},
	}

	bodyBytes2, err := json.Marshal(body2)
	require.NoError(t, err)

	res2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/admin/projects/"+tc.projectID.String()+"/organizations", bytes.NewReader(bodyBytes2))
	req2.Header.Set("Content-Type", "application/json")
	req2 = req2.WithContext(claim.WithSession(req2.Context(), validSession()))

	tc.controller.UpsertOrganization(res2, req2, tc.projectID)
	require.Equal(t, 200, res2.Code)

	var response2 oapi.Organization
	err = json.Unmarshal(res2.Body.Bytes(), &response2)
	require.NoError(t, err)

	require.Equal(t, orgID, response2.Id)
	require.Equal(t, "Updated Name", *response2.Name)
}

func TestUpsertOrganizationUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)

	body := oapi.UpsertOrganization{
		ExternalId: "org_123",
		Name:       ptr("Test Org"),
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects/"+tc.projectID.String()+"/organizations", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	tc.controller.UpsertOrganization(res, req, tc.projectID)

	require.Equal(t, 401, res.Code)
}

func TestUpsertOrganizationInvalidRequest(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects/"+tc.projectID.String()+"/organizations", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.UpsertOrganization(res, req, tc.projectID)

	require.Equal(t, 400, res.Code)
}

func TestGetOrganization(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Test Org"),
		Data: map[string]any{
			"industry": "technology",
		},
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String(), nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.GetOrganization(res, req, tc.projectID, orgID)

	require.Equal(t, 200, res.Code)

	var response oapi.Organization
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, orgID, response.Id)
	require.Equal(t, "org_123", response.ExternalId)
	require.Equal(t, "Test Org", *response.Name)
}

func TestGetOrganizationNotFound(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	nonExistentID := uuid.New()

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+nonExistentID.String(), nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.GetOrganization(res, req, tc.projectID, nonExistentID)

	require.Equal(t, 404, res.Code)
}

func TestGetOrganizationUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Test Org"),
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String(), nil)

	tc.controller.GetOrganization(res, req, tc.projectID, orgID)

	require.Equal(t, 401, res.Code)
}

func TestUpdateOrganization(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Original Name"),
	})
	require.NoError(t, err)

	dataBytes := json.RawMessage(`{"plan": "premium"}`)
	body := oapi.UpdateOrganization{
		Name: ptr("Updated Name"),
		Data: &dataBytes,
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.UpdateOrganization(res, req, tc.projectID, orgID)

	require.Equal(t, 200, res.Code)

	var response oapi.Organization
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, orgID, response.Id)
	require.Equal(t, "Updated Name", *response.Name)
}

func TestUpdateOrganizationNotFound(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	nonExistentID := uuid.New()

	body := oapi.UpdateOrganization{
		Name: ptr("Updated Name"),
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+nonExistentID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.UpdateOrganization(res, req, tc.projectID, nonExistentID)

	require.Equal(t, 404, res.Code)
}

func TestUpdateOrganizationUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Original Name"),
	})
	require.NoError(t, err)

	body := oapi.UpdateOrganization{
		Name: ptr("Updated Name"),
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	tc.controller.UpdateOrganization(res, req, tc.projectID, orgID)

	require.Equal(t, 401, res.Code)
}

func TestDeleteOrganization(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Test Org"),
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String(), nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.DeleteOrganization(res, req, tc.projectID, orgID)

	require.Equal(t, 204, res.Code)

	// Verify organization is deleted
	getRes := httptest.NewRecorder()
	getReq := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String(), nil)
	getReq = getReq.WithContext(claim.WithSession(getReq.Context(), validSession()))

	tc.controller.GetOrganization(getRes, getReq, tc.projectID, orgID)
	require.Equal(t, 404, getRes.Code)
}

func TestDeleteOrganizationNotFound(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	nonExistentID := uuid.New()

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+nonExistentID.String(), nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.DeleteOrganization(res, req, tc.projectID, nonExistentID)

	require.Equal(t, 404, res.Code)
}

func TestDeleteOrganizationUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Test Org"),
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String(), nil)

	tc.controller.DeleteOrganization(res, req, tc.projectID, orgID)

	require.Equal(t, 401, res.Code)
}

func TestListOrganizationMembers(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Test Org"),
	})
	require.NoError(t, err)

	// Create users and add them to the organization
	for i := 0; i < 3; i++ {
		userID, err := tc.usersDB.UsersStore.CreateUser(ctx, subjects.User{
			ProjectID:   tc.projectID,
			AnonymousID: ptr(uuid.New().String()),
			Data:        json.RawMessage(`{}`),
		})
		require.NoError(t, err)

		_, err = tc.usersDB.OrganizationsStore.UpsertAndGetOrganizationMember(ctx, orgID, userID, map[string]any{"role": "member"})
		require.NoError(t, err)
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String()+"/members", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.ListOrganizationMembers(res, req, tc.projectID, orgID, oapi.ListOrganizationMembersParams{})

	require.Equal(t, 200, res.Code)

	var response oapi.OrganizationMemberList
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 3, response.Total)
	require.Len(t, response.Results, 3)
}

func TestListOrganizationMembersOrganizationNotFound(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	nonExistentID := uuid.New()

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+nonExistentID.String()+"/members", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.ListOrganizationMembers(res, req, tc.projectID, nonExistentID, oapi.ListOrganizationMembersParams{})

	require.Equal(t, 404, res.Code)
}

func TestListOrganizationMembersUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Test Org"),
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String()+"/members", nil)

	tc.controller.ListOrganizationMembers(res, req, tc.projectID, orgID, oapi.ListOrganizationMembersParams{})

	require.Equal(t, 401, res.Code)
}

func TestAddOrganizationMember(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Test Org"),
	})
	require.NoError(t, err)

	userID, err := tc.usersDB.UsersStore.CreateUser(ctx, subjects.User{
		ProjectID:   tc.projectID,
		AnonymousID: ptr(uuid.New().String()),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	body := oapi.AddOrganizationMember{
		UserId: userID,
		Data: &map[string]any{
			"role":       "admin",
			"department": "engineering",
		},
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String()+"/members", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.AddOrganizationMember(res, req, tc.projectID, orgID)

	require.Equal(t, 200, res.Code)

	// Verify user was added
	listRes := httptest.NewRecorder()
	listReq := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String()+"/members", nil)
	listReq = listReq.WithContext(claim.WithSession(listReq.Context(), validSession()))

	tc.controller.ListOrganizationMembers(listRes, listReq, tc.projectID, orgID, oapi.ListOrganizationMembersParams{})
	require.Equal(t, 200, listRes.Code)

	var listResponse oapi.OrganizationMemberList
	err = json.Unmarshal(listRes.Body.Bytes(), &listResponse)
	require.NoError(t, err)
	require.Equal(t, 1, listResponse.Total)
}

func TestAddOrganizationMemberOrganizationNotFound(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	userID, err := tc.usersDB.UsersStore.CreateUser(ctx, subjects.User{
		ProjectID:   tc.projectID,
		AnonymousID: ptr(uuid.New().String()),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	nonExistentOrgID := uuid.New()

	body := oapi.AddOrganizationMember{
		UserId: userID,
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+nonExistentOrgID.String()+"/members", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.AddOrganizationMember(res, req, tc.projectID, nonExistentOrgID)

	require.Equal(t, 404, res.Code)
}

func TestAddOrganizationMemberUserNotFound(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Test Org"),
	})
	require.NoError(t, err)

	nonExistentUserID := uuid.New()

	body := oapi.AddOrganizationMember{
		UserId: nonExistentUserID,
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String()+"/members", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.AddOrganizationMember(res, req, tc.projectID, orgID)

	require.Equal(t, 404, res.Code)
}

func TestAddOrganizationMemberUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Test Org"),
	})
	require.NoError(t, err)

	body := oapi.AddOrganizationMember{
		UserId: uuid.New(),
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String()+"/members", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	tc.controller.AddOrganizationMember(res, req, tc.projectID, orgID)

	require.Equal(t, 401, res.Code)
}

func TestRemoveOrganizationMember(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Test Org"),
	})
	require.NoError(t, err)

	userID, err := tc.usersDB.UsersStore.CreateUser(ctx, subjects.User{
		ProjectID:   tc.projectID,
		AnonymousID: ptr(uuid.New().String()),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	_, err = tc.usersDB.OrganizationsStore.UpsertAndGetOrganizationMember(ctx, orgID, userID, nil)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String()+"/members/"+userID.String(), nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.RemoveOrganizationMember(res, req, tc.projectID, orgID, userID)

	require.Equal(t, 204, res.Code)

	// Verify user was removed
	listRes := httptest.NewRecorder()
	listReq := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String()+"/members", nil)
	listReq = listReq.WithContext(claim.WithSession(listReq.Context(), validSession()))

	tc.controller.ListOrganizationMembers(listRes, listReq, tc.projectID, orgID, oapi.ListOrganizationMembersParams{})
	require.Equal(t, 200, listRes.Code)

	var listResponse oapi.OrganizationMemberList
	err = json.Unmarshal(listRes.Body.Bytes(), &listResponse)
	require.NoError(t, err)
	require.Equal(t, 0, listResponse.Total)
}

func TestRemoveOrganizationMemberOrganizationNotFound(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	nonExistentOrgID := uuid.New()
	userID := uuid.New()

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+nonExistentOrgID.String()+"/members/"+userID.String(), nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.RemoveOrganizationMember(res, req, tc.projectID, nonExistentOrgID, userID)

	require.Equal(t, 404, res.Code)
}

func TestRemoveOrganizationMemberUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	orgID, err := tc.usersDB.OrganizationsStore.UpsertOrganization(ctx, tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_123",
		Name:       ptr("Test Org"),
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/projects/"+tc.projectID.String()+"/organizations/"+orgID.String()+"/members/"+uuid.New().String(), nil)

	tc.controller.RemoveOrganizationMember(res, req, tc.projectID, orgID, uuid.New())

	require.Equal(t, 401, res.Code)
}

func TestListOrganizationSchemas(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	paths := rules.Paths{
		{Path: ".plan", Type: "string"},
		{Path: ".seats", Type: "number"},
		{Path: ".active", Type: "boolean"},
		{Path: ".metadata", Type: "object"},
		{Path: ".metadata.industry", Type: "string"},
	}
	err := tc.usersDB.OrganizationsStore.UpsertOrganizationSchema(ctx, tc.projectID, paths)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/schema", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.ListOrganizationSchemas(res, req, tc.projectID)

	require.Equal(t, 200, res.Code)

	var response struct {
		Results []oapi.SchemaPath `json:"results"`
	}
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Results, 5)

	pathMap := make(map[string][]string)
	for _, schema := range response.Results {
		pathMap[schema.Path] = schema.Types
	}

	require.Contains(t, pathMap[".plan"], "string")
	require.Contains(t, pathMap[".seats"], "number")
	require.Contains(t, pathMap[".active"], "boolean")
	require.Contains(t, pathMap[".metadata"], "object")
	require.Contains(t, pathMap[".metadata.industry"], "string")
}

func TestListOrganizationSchemasEmpty(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/schema", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.ListOrganizationSchemas(res, req, tc.projectID)

	require.Equal(t, 200, res.Code)

	var response struct {
		Results []oapi.SchemaPath `json:"results"`
	}
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Empty(t, response.Results)
}

func TestListOrganizationSchemasUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/schema", nil)

	tc.controller.ListOrganizationSchemas(res, req, tc.projectID)

	require.Equal(t, 401, res.Code)
}

func TestListOrganizationSchemasWithMultipleTypes(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	paths := rules.Paths{
		{Path: ".seats", Type: "number"},
		{Path: ".seats", Type: "string"},
		{Path: ".active", Type: "boolean"},
		{Path: ".active", Type: "string"},
		{Path: ".tags", Type: "array"},
		{Path: ".tags", Type: "string"},
		{Path: ".metadata", Type: "object"},
		{Path: ".plan", Type: "string"},
	}
	err := tc.usersDB.OrganizationsStore.UpsertOrganizationSchema(ctx, tc.projectID, paths)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/schema", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.ListOrganizationSchemas(res, req, tc.projectID)

	require.Equal(t, 200, res.Code)

	var response struct {
		Results []oapi.SchemaPath `json:"results"`
	}
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Results, 5)

	pathMap := make(map[string][]string)
	for _, schema := range response.Results {
		pathMap[schema.Path] = schema.Types
	}

	require.Contains(t, pathMap, ".seats")
	require.Len(t, pathMap[".seats"], 2)
	require.Contains(t, pathMap[".seats"], "number")
	require.Contains(t, pathMap[".seats"], "string")

	require.Contains(t, pathMap, ".active")
	require.Len(t, pathMap[".active"], 2)
	require.Contains(t, pathMap[".active"], "boolean")
	require.Contains(t, pathMap[".active"], "string")

	require.Contains(t, pathMap, ".tags")
	require.Len(t, pathMap[".tags"], 2)
	require.Contains(t, pathMap[".tags"], "array")
	require.Contains(t, pathMap[".tags"], "string")

	require.Contains(t, pathMap, ".metadata")
	require.Len(t, pathMap[".metadata"], 1)
	require.Contains(t, pathMap[".metadata"], "object")

	require.Contains(t, pathMap, ".plan")
	require.Len(t, pathMap[".plan"], 1)
	require.Contains(t, pathMap[".plan"], "string")
}

func TestListOrganizationMemberSchemas(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	paths := rules.Paths{
		{Path: ".role", Type: "string"},
		{Path: ".level", Type: "number"},
		{Path: ".permissions", Type: "array"},
		{Path: ".metadata", Type: "object"},
		{Path: ".metadata.department", Type: "string"},
	}
	err := tc.usersDB.OrganizationsStore.UpsertOrganizationUserSchema(ctx, tc.projectID, paths)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/members/schema", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.ListOrganizationMemberSchemas(res, req, tc.projectID)

	require.Equal(t, 200, res.Code)

	var response struct {
		Results []oapi.SchemaPath `json:"results"`
	}
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Results, 5)

	pathMap := make(map[string][]string)
	for _, schema := range response.Results {
		pathMap[schema.Path] = schema.Types
	}

	require.Contains(t, pathMap[".role"], "string")
	require.Contains(t, pathMap[".level"], "number")
	require.Contains(t, pathMap[".permissions"], "array")
	require.Contains(t, pathMap[".metadata"], "object")
	require.Contains(t, pathMap[".metadata.department"], "string")
}

func TestListOrganizationMemberSchemasEmpty(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/members/schema", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.ListOrganizationMemberSchemas(res, req, tc.projectID)

	require.Equal(t, 200, res.Code)

	var response struct {
		Results []oapi.SchemaPath `json:"results"`
	}
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Empty(t, response.Results)
}

func TestListOrganizationMemberSchemasUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/members/schema", nil)

	tc.controller.ListOrganizationMemberSchemas(res, req, tc.projectID)

	require.Equal(t, 401, res.Code)
}

func TestListOrganizationMemberSchemasWithMultipleTypes(t *testing.T) {
	t.Parallel()

	tc := setupSubjectOrganizationsController(t)
	ctx := context.Background()

	paths := rules.Paths{
		{Path: ".level", Type: "number"},
		{Path: ".level", Type: "string"},
		{Path: ".active", Type: "boolean"},
		{Path: ".active", Type: "string"},
		{Path: ".permissions", Type: "array"},
		{Path: ".permissions", Type: "string"},
		{Path: ".metadata", Type: "object"},
		{Path: ".role", Type: "string"},
	}
	err := tc.usersDB.OrganizationsStore.UpsertOrganizationUserSchema(ctx, tc.projectID, paths)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/organizations/members/schema", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	tc.controller.ListOrganizationMemberSchemas(res, req, tc.projectID)

	require.Equal(t, 200, res.Code)

	var response struct {
		Results []oapi.SchemaPath `json:"results"`
	}
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Results, 5)

	pathMap := make(map[string][]string)
	for _, schema := range response.Results {
		pathMap[schema.Path] = schema.Types
	}

	require.Contains(t, pathMap, ".level")
	require.Len(t, pathMap[".level"], 2)
	require.Contains(t, pathMap[".level"], "number")
	require.Contains(t, pathMap[".level"], "string")

	require.Contains(t, pathMap, ".active")
	require.Len(t, pathMap[".active"], 2)
	require.Contains(t, pathMap[".active"], "boolean")
	require.Contains(t, pathMap[".active"], "string")

	require.Contains(t, pathMap, ".permissions")
	require.Len(t, pathMap[".permissions"], 2)
	require.Contains(t, pathMap[".permissions"], "array")
	require.Contains(t, pathMap[".permissions"], "string")

	require.Contains(t, pathMap, ".metadata")
	require.Len(t, pathMap[".metadata"], 1)
	require.Contains(t, pathMap[".metadata"], "object")

	require.Contains(t, pathMap, ".role")
	require.Len(t, pathMap[".role"], 1)
	require.Contains(t, pathMap[".role"], "string")
}

package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCreateSenderIdentity(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	providerStore := management.NewProvidersStore(mgmt)
	emailProviderID, err := providerStore.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "test",
		Channels:  management.Channels{"email"},
		Name:      "Test Email Provider",
		Data:      json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	smsProviderID, err := providerStore.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "test",
		Channels:  management.Channels{"sms"},
		Name:      "Test SMS Provider",
		Data:      json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewSenderIdentitiesController(logger, mgmt, engine)

	type test struct {
		body oapi.CreateSenderIdentityJSONRequestBody
		code int
	}

	tests := map[string]test{
		"email identity": {
			body: oapi.CreateSenderIdentityJSONRequestBody{
				Channel:    oapi.CreateSenderIdentityChannelEmail,
				ProviderId: emailProviderID,
				Traits:     map[string]interface{}{"address": "hello@example.com"},
			},
			code: 201,
		},
		"email identity with name": {
			body: oapi.CreateSenderIdentityJSONRequestBody{
				Channel:    oapi.CreateSenderIdentityChannelEmail,
				ProviderId: emailProviderID,
				Traits:     map[string]interface{}{"address": "support@example.com", "name": "Acme Support"},
			},
			code: 201,
		},
		"sms identity": {
			body: oapi.CreateSenderIdentityJSONRequestBody{
				Channel:    oapi.CreateSenderIdentityChannelSms,
				ProviderId: smsProviderID,
				Traits:     map[string]interface{}{"address": "+15551234567"},
			},
			code: 201,
		},
		"missing address": {
			body: oapi.CreateSenderIdentityJSONRequestBody{
				Channel:    oapi.CreateSenderIdentityChannelEmail,
				ProviderId: emailProviderID,
				Traits:     map[string]interface{}{"address": ""},
			},
			code: 400,
		},
		"invalid channel": {
			body: oapi.CreateSenderIdentityJSONRequestBody{
				Channel:    "push",
				ProviderId: emailProviderID,
				Traits:     map[string]interface{}{"address": "hello@example.com"},
			},
			code: 400,
		},
		"provider not found": {
			body: oapi.CreateSenderIdentityJSONRequestBody{
				Channel:    oapi.CreateSenderIdentityChannelEmail,
				ProviderId: uuid.New(),
				Traits:     map[string]interface{}{"address": "hello@example.com"},
			},
			code: 404,
		},
		"channel mismatch with provider": {
			body: oapi.CreateSenderIdentityJSONRequestBody{
				Channel:    oapi.CreateSenderIdentityChannelEmail,
				ProviderId: smsProviderID,
				Traits:     map[string]interface{}{"address": "hello@example.com"},
			},
			code: 400,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/sender-identities", bytes.NewReader(bb))
			req = req.WithContext(actorCtx)
			controller.CreateSenderIdentity(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 201 {
				var result oapi.SenderIdentity
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, test.body.Traits["address"], result.Traits["address"])
				require.Equal(t, oapi.SenderIdentityChannel(test.body.Channel), result.Channel)
				require.Equal(t, test.body.ProviderId, result.ProviderId)
				require.Equal(t, projectID, result.ProjectId)
				require.NotEqual(t, uuid.Nil, result.Id)
			}
		})
	}
}

func TestCreateSenderIdentityProjectNotFound(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	invalidProjectID := uuid.New()

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(invalidProjectID),
	)
	engine, actorCtx := rbac.TestSetup(t, t.Context(), actor, "owner", "admin")

	controller := NewSenderIdentitiesController(logger, mgmt, engine)

	body := oapi.CreateSenderIdentityJSONRequestBody{
		Channel:    oapi.CreateSenderIdentityChannelEmail,
		ProviderId: uuid.New(),
		Traits:     map[string]interface{}{"address": "hello@example.com"},
	}

	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/sender-identities", bytes.NewReader(bb))
	req = req.WithContext(actorCtx)
	controller.CreateSenderIdentity(res, req, invalidProjectID)

	require.Equal(t, 404, res.Code, res.Body.String())
}

func TestListSenderIdentities(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	providerStore := management.NewProvidersStore(mgmt)
	emailProviderID, err := providerStore.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "test",
		Channels:  management.Channels{"email"},
		Name:      "Test Email Provider",
		Data:      json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	smsProviderID, err := providerStore.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "test",
		Channels:  management.Channels{"sms"},
		Name:      "Test SMS Provider",
		Data:      json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	// Create some sender identities via the store directly
	identitiesStore := management.NewSenderIdentitiesStore(mgmt)
	for i, addr := range []string{"one@example.com", "two@example.com", "three@example.com"} {
		provID := emailProviderID
		ch := "email"
		if i == 2 {
			provID = smsProviderID
			ch = "sms"
		}
		_, err := identitiesStore.CreateSenderIdentity(ctx, management.SenderIdentity{
			ProjectID:  projectID,
			ProviderID: provID,
			Channel:    ch,
			Traits:     json.RawMessage(`{"address":"` + addr + `"}`),
		})
		require.NoError(t, err)
	}

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewSenderIdentitiesController(logger, mgmt, engine)

	type test struct {
		params   oapi.ListSenderIdentitiesParams
		expected int
		total    int
	}

	emailChannel := oapi.ListSenderIdentitiesParamsChannelEmail
	smsChannel := oapi.ListSenderIdentitiesParamsChannelSms

	tests := map[string]test{
		"list-all": {
			params: oapi.ListSenderIdentitiesParams{
				Limit:  ptr(oapi.Limit(20)),
				Offset: ptr(oapi.Offset(0)),
			},
			expected: 3,
			total:    3,
		},
		"with-pagination": {
			params: oapi.ListSenderIdentitiesParams{
				Limit:  ptr(oapi.Limit(2)),
				Offset: ptr(oapi.Offset(0)),
			},
			expected: 2,
			total:    3,
		},
		"with-offset": {
			params: oapi.ListSenderIdentitiesParams{
				Limit:  ptr(oapi.Limit(20)),
				Offset: ptr(oapi.Offset(2)),
			},
			expected: 1,
			total:    3,
		},
		"filter-by-email-channel": {
			params: oapi.ListSenderIdentitiesParams{
				Limit:   ptr(oapi.Limit(20)),
				Offset:  ptr(oapi.Offset(0)),
				Channel: &emailChannel,
			},
			expected: 2,
			total:    2,
		},
		"filter-by-sms-channel": {
			params: oapi.ListSenderIdentitiesParams{
				Limit:   ptr(oapi.Limit(20)),
				Offset:  ptr(oapi.Offset(0)),
				Channel: &smsChannel,
			},
			expected: 1,
			total:    1,
		},
		"filter-by-provider": {
			params: oapi.ListSenderIdentitiesParams{
				Limit:      ptr(oapi.Limit(20)),
				Offset:     ptr(oapi.Offset(0)),
				ProviderId: &emailProviderID,
			},
			expected: 2,
			total:    2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/sender-identities", nil)
			req = req.WithContext(actorCtx)
			controller.ListSenderIdentities(res, req, projectID, test.params)

			require.Equal(t, 200, res.Code, res.Body.String())

			var result struct {
				Results []oapi.SenderIdentity `json:"results"`
				Total   int                   `json:"total"`
				Limit   int                   `json:"limit"`
				Offset  int                   `json:"offset"`
			}
			err := json.Unmarshal(res.Body.Bytes(), &result)
			require.NoError(t, err)
			require.Equal(t, test.expected, len(result.Results))
			require.Equal(t, test.total, result.Total)
		})
	}
}

func TestListSenderIdentitiesProjectNotFound(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	invalidProjectID := uuid.New()

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(invalidProjectID),
	)
	engine, actorCtx := rbac.TestSetup(t, t.Context(), actor, "owner", "admin")

	controller := NewSenderIdentitiesController(logger, mgmt, engine)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/sender-identities", nil)
	req = req.WithContext(actorCtx)
	controller.ListSenderIdentities(res, req, invalidProjectID, oapi.ListSenderIdentitiesParams{
		Limit:  ptr(oapi.Limit(20)),
		Offset: ptr(oapi.Offset(0)),
	})

	require.Equal(t, 404, res.Code, res.Body.String())
}

func TestGetSenderIdentity(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	providerStore := management.NewProvidersStore(mgmt)
	emailProviderID, err := providerStore.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "test",
		Channels:  management.Channels{"email"},
		Name:      "Test Email Provider",
		Data:      json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	identitiesStore := management.NewSenderIdentitiesStore(mgmt)
	identityID, err := identitiesStore.CreateSenderIdentity(ctx, management.SenderIdentity{
		ProjectID:  projectID,
		ProviderID: emailProviderID,
		Channel:    "email",
		Traits:     json.RawMessage(`{"address":"hello@example.com","name":"Acme"}`),
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewSenderIdentitiesController(logger, mgmt, engine)

	type test struct {
		identityID uuid.UUID
		code       int
	}

	tests := map[string]test{
		"existing-identity": {
			identityID: identityID,
			code:       200,
		},
		"non-existing-identity": {
			identityID: uuid.New(),
			code:       404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", fmt.Sprintf("/v1/sender-identities/%s", test.identityID), nil)
			req = req.WithContext(actorCtx)
			controller.GetSenderIdentity(res, req, projectID, test.identityID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result oapi.SenderIdentity
				err := json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, test.identityID, result.Id)
				require.Equal(t, "hello@example.com", result.Traits["address"])
				require.Equal(t, oapi.SenderIdentityChannel("email"), result.Channel)
				require.Equal(t, emailProviderID, result.ProviderId)
				require.Equal(t, projectID, result.ProjectId)
				require.Equal(t, "Acme", result.Traits["name"])
			}
		})
	}
}

func TestGetSenderIdentityProjectNotFound(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	invalidProjectID := uuid.New()

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(invalidProjectID),
	)
	engine, actorCtx := rbac.TestSetup(t, t.Context(), actor, "owner", "admin")

	controller := NewSenderIdentitiesController(logger, mgmt, engine)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/sender-identities/%s", uuid.New()), nil)
	req = req.WithContext(actorCtx)
	controller.GetSenderIdentity(res, req, invalidProjectID, uuid.New())

	require.Equal(t, 404, res.Code, res.Body.String())
}

func TestDeleteSenderIdentity(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	providerStore := management.NewProvidersStore(mgmt)
	emailProviderID, err := providerStore.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "test",
		Channels:  management.Channels{"email"},
		Name:      "Test Email Provider",
		Data:      json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	identitiesStore := management.NewSenderIdentitiesStore(mgmt)
	identityID, err := identitiesStore.CreateSenderIdentity(ctx, management.SenderIdentity{
		ProjectID:  projectID,
		ProviderID: emailProviderID,
		Channel:    "email",
		Traits:     json.RawMessage(`{"address":"delete-me@example.com"}`),
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewSenderIdentitiesController(logger, mgmt, engine)

	type test struct {
		identityID uuid.UUID
		code       int
	}

	tests := map[string]test{
		"successful-delete": {
			identityID: identityID,
			code:       204,
		},
		"non-existing-identity": {
			identityID: uuid.New(),
			code:       204, // Idempotent — deleting a non-existing identity succeeds
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", fmt.Sprintf("/v1/sender-identities/%s", test.identityID), nil)
			req = req.WithContext(actorCtx)
			controller.DeleteSenderIdentity(res, req, projectID, test.identityID)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}

	// Verify the identity is actually gone
	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/sender-identities/%s", identityID), nil)
	req = req.WithContext(actorCtx)
	controller.GetSenderIdentity(res, req, projectID, identityID)
	require.Equal(t, http.StatusNotFound, res.Code, res.Body.String())
}

func TestDeleteSenderIdentityProjectNotFound(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	invalidProjectID := uuid.New()

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(invalidProjectID),
	)
	engine, actorCtx := rbac.TestSetup(t, t.Context(), actor, "owner", "admin")

	controller := NewSenderIdentitiesController(logger, mgmt, engine)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/v1/sender-identities/%s", uuid.New()), nil)
	req = req.WithContext(actorCtx)
	controller.DeleteSenderIdentity(res, req, invalidProjectID, uuid.New())

	require.Equal(t, 404, res.Code, res.Body.String())
}

func TestCreateAndGetSenderIdentityRoundTrip(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	providerStore := management.NewProvidersStore(mgmt)
	emailProviderID, err := providerStore.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "test",
		Channels:  management.Channels{"email"},
		Name:      "Test Email Provider",
		Data:      json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewSenderIdentitiesController(logger, mgmt, engine)

	// Create
	createBody := oapi.CreateSenderIdentityJSONRequestBody{
		Channel:    oapi.CreateSenderIdentityChannelEmail,
		ProviderId: emailProviderID,
		Traits:     map[string]interface{}{"address": "roundtrip@example.com", "name": "Round Trip"},
	}

	bb, err := json.Marshal(createBody)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/sender-identities", bytes.NewReader(bb))
	req = req.WithContext(actorCtx)
	controller.CreateSenderIdentity(res, req, projectID)
	require.Equal(t, 201, res.Code, res.Body.String())

	var created oapi.SenderIdentity
	err = json.Unmarshal(res.Body.Bytes(), &created)
	require.NoError(t, err)

	// Get
	res = httptest.NewRecorder()
	req = httptest.NewRequest("GET", fmt.Sprintf("/v1/sender-identities/%s", created.Id), nil)
	req = req.WithContext(actorCtx)
	controller.GetSenderIdentity(res, req, projectID, created.Id)
	require.Equal(t, 200, res.Code, res.Body.String())

	var retrieved oapi.SenderIdentity
	err = json.Unmarshal(res.Body.Bytes(), &retrieved)
	require.NoError(t, err)

	require.Equal(t, created.Id, retrieved.Id)
	require.Equal(t, created.Traits["address"], retrieved.Traits["address"])
	require.Equal(t, created.Channel, retrieved.Channel)
	require.Equal(t, created.ProviderId, retrieved.ProviderId)
	require.Equal(t, created.ProjectId, retrieved.ProjectId)
	require.Equal(t, "Round Trip", retrieved.Traits["name"])

	// List should include it
	res = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/v1/sender-identities", nil)
	req = req.WithContext(actorCtx)
	controller.ListSenderIdentities(res, req, projectID, oapi.ListSenderIdentitiesParams{
		Limit:  ptr(oapi.Limit(20)),
		Offset: ptr(oapi.Offset(0)),
	})
	require.Equal(t, 200, res.Code, res.Body.String())

	var listResult struct {
		Results []oapi.SenderIdentity `json:"results"`
		Total   int                   `json:"total"`
	}
	err = json.Unmarshal(res.Body.Bytes(), &listResult)
	require.NoError(t, err)
	require.Equal(t, 1, listResult.Total)
	require.Len(t, listResult.Results, 1)
	require.Equal(t, created.Id, listResult.Results[0].Id)

	// Delete
	res = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", fmt.Sprintf("/v1/sender-identities/%s", created.Id), nil)
	req = req.WithContext(actorCtx)
	controller.DeleteSenderIdentity(res, req, projectID, created.Id)
	require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())

	// Verify gone
	res = httptest.NewRecorder()
	req = httptest.NewRequest("GET", fmt.Sprintf("/v1/sender-identities/%s", created.Id), nil)
	req = req.WithContext(actorCtx)
	controller.GetSenderIdentity(res, req, projectID, created.Id)
	require.Equal(t, http.StatusNotFound, res.Code, res.Body.String())
}

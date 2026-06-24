package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrgEndpoints_OwnData_Forbidden proves the organization/cross-subject
// endpoints fail closed (403) for an own-data actor across organizations.go and
// scheduled.go. The guard runs right after the permission check, so a valid
// body is unnecessary — the request never reaches decoding.
func TestOrgEndpoints_OwnData_Forbidden(t *testing.T) {
	t.Parallel()

	type call func(c *testClientController, r *http.Request, w http.ResponseWriter)

	withBody := func(method, path string) *http.Request {
		body, _ := json.Marshal(map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "org_1"}},
			"name":       "x",
		})
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		return req
	}

	tests := map[string]struct {
		req  func() *http.Request
		call call
	}{
		"UpsertOrganizationClient": {
			req: func() *http.Request { return withBody(http.MethodPost, "/api/client/organizations") },
			call: func(c *testClientController, r *http.Request, w http.ResponseWriter) {
				c.UpsertOrganizationClient(w, r, uuid.Nil)
			},
		},
		"DeleteOrganizationClient": {
			req: func() *http.Request { return withBody(http.MethodDelete, "/api/client/organizations") },
			call: func(c *testClientController, r *http.Request, w http.ResponseWriter) {
				c.DeleteOrganizationClient(w, r, uuid.Nil)
			},
		},
		"AddOrganizationUserClient": {
			req: func() *http.Request { return withBody(http.MethodPost, "/api/client/organizations/users") },
			call: func(c *testClientController, r *http.Request, w http.ResponseWriter) {
				c.AddOrganizationUserClient(w, r, uuid.Nil)
			},
		},
		"RemoveOrganizationUserClient": {
			req: func() *http.Request { return withBody(http.MethodDelete, "/api/client/organizations/users") },
			call: func(c *testClientController, r *http.Request, w http.ResponseWriter) {
				c.RemoveOrganizationUserClient(w, r, uuid.Nil)
			},
		},
		"UpsertOrganizationScheduledClient": {
			req: func() *http.Request { return withBody(http.MethodPost, "/api/client/organizations/scheduled") },
			call: func(c *testClientController, r *http.Request, w http.ResponseWriter) {
				c.UpsertOrganizationScheduledClient(w, r, uuid.Nil)
			},
		},
		"DeleteOrganizationScheduledClient": {
			req: func() *http.Request { return withBody(http.MethodDelete, "/api/client/organizations/scheduled") },
			call: func(c *testClientController, r *http.Request, w http.ResponseWriter) {
				c.DeleteOrganizationScheduledClient(w, r, uuid.Nil)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := setupClientController(t)
			c.withCapturingPublisher()
			orgID, projectID := c.newProject(t)
			actor := c.ownDataActor(t, orgID, projectID, verifiedSubject, verifiedSubjectSource)

			req := tc.req().WithContext(rbac.WithActor(t.Context(), actor))
			w := httptest.NewRecorder()
			tc.call(c, req, w)

			assert.Equal(t, http.StatusForbidden, w.Code)
		})
	}
}

// TestUpsertUserClient_OwnData_BindsToVerifiedSubject proves user upsert binds
// to the verified subject: an own-data actor supplying ANOTHER user's
// identifier persists/returns the verified subject, not the supplied one.
func TestUpsertUserClient_OwnData_BindsToVerifiedSubject(t *testing.T) {
	t.Parallel()

	c := setupClientController(t)
	c.withCapturingPublisher()
	orgID, projectID := c.newProject(t)
	actor := c.ownDataActor(t, orgID, projectID, verifiedSubject, verifiedSubjectSource)

	body, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "crm", "external_id": "someone-else"}},
		"email":      "victim@example.com",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/client/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	c.UpsertUserClient(w, req, uuid.Nil)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Identifier []struct {
			Source     string `json:"source"`
			ExternalId string `json:"external_id"`
		} `json:"identifier"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Identifier, 1)
	assert.Equal(t, verifiedSubject, resp.Identifier[0].ExternalId,
		"user must be created for the verified subject, not the supplied identifier")
	assert.Equal(t, verifiedSubjectSource, resp.Identifier[0].Source)
}

// TestUpsertUserClient_AllData_Passthrough proves a verified all-data user keeps
// the client-supplied identifier when upserting.
func TestUpsertUserClient_AllData_Passthrough(t *testing.T) {
	t.Parallel()

	c := setupClientController(t)
	c.withCapturingPublisher()
	orgID, projectID := c.newProject(t)
	actor := c.allDataActor(t, orgID, projectID)

	body, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "crm", "external_id": "user-555"}},
		"email":      "real@example.com",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/client/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	c.UpsertUserClient(w, req, uuid.Nil)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Identifier []struct {
			Source     string `json:"source"`
			ExternalId string `json:"external_id"`
		} `json:"identifier"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Identifier, 1)
	assert.Equal(t, "user-555", resp.Identifier[0].ExternalId)
	assert.Equal(t, "crm", resp.Identifier[0].Source)
}

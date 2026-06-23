package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inboxStateMessages filters captured publishes down to InboxStateEvent payloads.
func inboxStateMessages(t *testing.T, pub *capturingPublisher) []schemas.InboxStateEvent {
	t.Helper()
	var out []schemas.InboxStateEvent
	for _, m := range pub.captured() {
		if e, ok := m.Value.(schemas.InboxStateEvent); ok {
			out = append(out, e)
		}
	}
	return out
}

// TestPostUserInboxRead_OwnData_BindsToVerifiedSubject proves an own-data actor
// cannot mark ANOTHER user's messages read: the supplied target is overridden
// with the verified subject (publishUserInboxStateEvents binding).
func TestPostUserInboxRead_OwnData_BindsToVerifiedSubject(t *testing.T) {
	t.Parallel()

	c := setupClientController(t)
	pub := c.withCapturingPublisher()
	orgID, projectID := c.newProject(t)
	actor := c.ownDataActor(t, orgID, projectID, verifiedSubject, verifiedSubjectSource)

	body, err := json.Marshal([]map[string]any{
		{
			"message_id": uuid.New().String(),
			"target":     []map[string]any{{"source": "crm", "external_id": "someone-else"}},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/client/users/inbox/read", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	c.PostUserInboxRead(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)

	msgs := inboxStateMessages(t, pub)
	require.Len(t, msgs, 1)
	assert.Equal(t, []subjects.ExternalIDParam{
		{Source: verifiedSubjectSource, ExternalID: verifiedSubject},
	}, msgs[0].Identifiers)
}

// TestPostUserInboxRead_AllData_Passthrough proves a verified all-data user
// keeps the supplied target.
func TestPostUserInboxRead_AllData_Passthrough(t *testing.T) {
	t.Parallel()

	c := setupClientController(t)
	pub := c.withCapturingPublisher()
	orgID, projectID := c.newProject(t)
	actor := c.allDataActor(t, orgID, projectID)

	body, err := json.Marshal([]map[string]any{
		{
			"message_id": uuid.New().String(),
			"target":     []map[string]any{{"source": "crm", "external_id": "user-777"}},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/client/users/inbox/read", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	c.PostUserInboxRead(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)

	msgs := inboxStateMessages(t, pub)
	require.Len(t, msgs, 1)
	assert.Equal(t, []subjects.ExternalIDParam{
		{Source: "crm", ExternalID: "user-777"},
	}, msgs[0].Identifiers)
}

// TestOrgInbox_OwnData_Forbidden proves every organization inbox endpoint fails
// closed (403) for an own-data actor: "own data" has no meaning for an
// organization, so a confined end user may never act across one.
func TestOrgInbox_OwnData_Forbidden(t *testing.T) {
	t.Parallel()

	read := func(handler func(*InboxController, *testClientController, *rbac.Actor) *httptest.ResponseRecorder) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			c := setupClientController(t)
			c.withCapturingPublisher()
			orgID, projectID := c.newProject(t)
			actor := c.ownDataActor(t, orgID, projectID, verifiedSubject, verifiedSubjectSource)
			w := handler(c.InboxController, c, actor)
			assert.Equal(t, http.StatusForbidden, w.Code)
		}
	}

	orgEvent := func() []byte {
		body, _ := json.Marshal([]map[string]any{
			{
				"message_id": uuid.New().String(),
				"target":     []map[string]any{{"source": "default", "external_id": "org_1"}},
			},
		})
		return body
	}

	t.Run("GetOrganizationInbox", read(func(ic *InboxController, c *testClientController, actor *rbac.Actor) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/client/organizations/inbox", nil)
		req = req.WithContext(rbac.WithActor(req.Context(), actor))
		w := httptest.NewRecorder()
		ic.GetOrganizationInbox(w, req, oapi.GetOrganizationInboxParams{})
		return w
	}))

	t.Run("GetOrganizationInboxCount", read(func(ic *InboxController, c *testClientController, actor *rbac.Actor) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/client/organizations/inbox/count", nil)
		req = req.WithContext(rbac.WithActor(req.Context(), actor))
		w := httptest.NewRecorder()
		ic.GetOrganizationInboxCount(w, req, oapi.GetOrganizationInboxCountParams{})
		return w
	}))

	t.Run("PostOrganizationInboxRead", read(func(ic *InboxController, c *testClientController, actor *rbac.Actor) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/client/organizations/inbox/read", bytes.NewReader(orgEvent()))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(rbac.WithActor(req.Context(), actor))
		w := httptest.NewRecorder()
		ic.PostOrganizationInboxRead(w, req)
		return w
	}))

	t.Run("PostOrganizationInboxArchived", read(func(ic *InboxController, c *testClientController, actor *rbac.Actor) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/client/organizations/inbox/archived", bytes.NewReader(orgEvent()))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(rbac.WithActor(req.Context(), actor))
		w := httptest.NewRecorder()
		ic.PostOrganizationInboxArchived(w, req)
		return w
	}))
}

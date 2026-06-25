package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	verifiedSubject       = "verified-user-123"
	verifiedSubjectSource = "https://idp.example"
)

// postEvents marshals events and invokes PostUserEvents with the given actor.
func postEvents(t *testing.T, c *testClientController, actor *rbac.Actor, events []map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/client/users/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()
	c.PostUserEvents(w, req, uuid.Nil)
	return w
}

// userEventMessages filters the captured publishes down to UserEvent payloads.
func userEventMessages(t *testing.T, pub *capturingPublisher) []schemas.UserEvent {
	t.Helper()
	var out []schemas.UserEvent
	for _, m := range pub.captured() {
		if ue, ok := m.Value.(schemas.UserEvent); ok {
			out = append(out, ue)
		}
	}
	return out
}

// TestPostUserEvents_OwnData_BindsToVerifiedSubject proves the core bypass
// guard: an own-data actor that supplies ANOTHER user's identifier still emits
// an event attributed to its own verified subject, never the supplied one.
func TestPostUserEvents_OwnData_BindsToVerifiedSubject(t *testing.T) {
	t.Parallel()

	c := setupClientController(t)
	pub := c.withCapturingPublisher()
	orgID, projectID := c.newProject(t)
	actor := c.ownDataActor(t, orgID, projectID, verifiedSubject, verifiedSubjectSource)

	w := postEvents(t, c, actor, []map[string]any{
		{
			"name":       "purchase_completed",
			"identifier": []map[string]any{{"source": "crm", "external_id": "someone-else"}},
		},
	})

	require.Equal(t, http.StatusAccepted, w.Code)

	msgs := userEventMessages(t, pub)
	require.Len(t, msgs, 1)
	// The client-supplied "someone-else" identifier must be discarded and
	// replaced with the verified subject.
	assert.Equal(t, []subjects.ExternalIDParam{
		{Source: verifiedSubjectSource, ExternalID: verifiedSubject},
	}, msgs[0].Identifiers)
}

// TestPostUserEvents_OwnData_NoIdentifierStillBound proves an own-data actor
// that supplies no identifier at all is still bound to its verified subject.
func TestPostUserEvents_OwnData_NoIdentifierStillBound(t *testing.T) {
	t.Parallel()

	c := setupClientController(t)
	pub := c.withCapturingPublisher()
	orgID, projectID := c.newProject(t)
	actor := c.ownDataActor(t, orgID, projectID, verifiedSubject, verifiedSubjectSource)

	w := postEvents(t, c, actor, []map[string]any{{"name": "page_viewed"}})

	require.Equal(t, http.StatusAccepted, w.Code)

	msgs := userEventMessages(t, pub)
	require.Len(t, msgs, 1)
	assert.Equal(t, []subjects.ExternalIDParam{
		{Source: verifiedSubjectSource, ExternalID: verifiedSubject},
	}, msgs[0].Identifiers)
}

// TestPostUserEvents_OwnData_MatchForbidden proves an own-data actor may not
// emit match-based (attribute fan-out) events: those target other users.
func TestPostUserEvents_OwnData_MatchForbidden(t *testing.T) {
	t.Parallel()

	c := setupClientController(t)
	pub := c.withCapturingPublisher()
	orgID, projectID := c.newProject(t)
	actor := c.ownDataActor(t, orgID, projectID, verifiedSubject, verifiedSubjectSource)

	w := postEvents(t, c, actor, []map[string]any{
		{
			"name":  "promo",
			"match": map[string]any{"plan": "enterprise"},
		},
	})

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Empty(t, pub.captured(), "no event should be published when match is rejected")
}

// TestPostUserEvents_AllData_Passthrough proves a verified end user scoped to
// all data keeps the client-supplied identifier (trusted-integration behaviour).
func TestPostUserEvents_AllData_Passthrough(t *testing.T) {
	t.Parallel()

	c := setupClientController(t)
	pub := c.withCapturingPublisher()
	orgID, projectID := c.newProject(t)
	actor := c.allDataActor(t, orgID, projectID)

	w := postEvents(t, c, actor, []map[string]any{
		{
			"name":       "purchase_completed",
			"identifier": []map[string]any{{"source": "crm", "external_id": "user-999"}},
		},
	})

	require.Equal(t, http.StatusAccepted, w.Code)

	msgs := userEventMessages(t, pub)
	require.Len(t, msgs, 1)
	assert.Equal(t, []subjects.ExternalIDParam{
		{Source: "crm", ExternalID: "user-999"},
	}, msgs[0].Identifiers)
}

// TestPostUserEvents_NonOwn_MutualExclusivity preserves the match/identifier
// validation for non-own callers (here a verified all-data user). An own-data
// actor never reaches this branch (match is rejected up front).
func TestPostUserEvents_NonOwn_MutualExclusivity(t *testing.T) {
	t.Parallel()

	c := setupClientController(t)
	c.withCapturingPublisher()
	orgID, projectID := c.newProject(t)
	actor := c.allDataActor(t, orgID, projectID)

	w := postEvents(t, c, actor, []map[string]any{
		{
			"name":       "promo",
			"match":      map[string]any{"plan": "enterprise"},
			"identifier": []map[string]any{{"source": "crm", "external_id": "user-1"}},
		},
	})

	assert.Equal(t, http.StatusBadRequest, w.Code, "match and identifier are mutually exclusive")
}

// TestPostUserEvents_NonOwn_IdentifierRequired preserves the "one of match or
// identifier is required" validation for non-own callers.
func TestPostUserEvents_NonOwn_IdentifierRequired(t *testing.T) {
	t.Parallel()

	c := setupClientController(t)
	c.withCapturingPublisher()
	orgID, projectID := c.newProject(t)
	actor := c.allDataActor(t, orgID, projectID)

	w := postEvents(t, c, actor, []map[string]any{{"name": "promo"}})

	assert.Equal(t, http.StatusBadRequest, w.Code, "one of match or identifier is required")
}

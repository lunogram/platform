package v1

import (
	"bytes"
	"context"
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
	"go.uber.org/zap/zaptest"
)

// newInboxController wires an InboxController over an in-memory RBAC engine that
// grants the actor the "client" project role (which carries inbox-create) and a
// capturing publisher. It returns the controller, the publisher, and the
// project the actor is scoped to.
func newInboxController(t *testing.T, actor *rbac.Actor) (*InboxController, *capturingPublisher, context.Context) {
	t.Helper()

	// "member" org + "client" project role: the client role grants inbox create.
	engine, ctx := rbac.TestSetup(t, context.Background(), actor, "member", "client")

	pub := &capturingPublisher{}
	client := NewClientController(zaptest.NewLogger(t), nil, nil, nil, pub, engine)
	return NewInboxController(client), pub, ctx
}

// postInboxMessage issues PostUserInboxMessages with a single message addressed
// to the given target external id (the client-supplied recipient).
func postInboxMessage(t *testing.T, srv *InboxController, ctx context.Context, suppliedTarget string) *httptest.ResponseRecorder {
	t.Helper()

	body := []map[string]any{{
		"target":     []map[string]any{{"external_id": suppliedTarget}},
		"identifier": map[string]any{"external_id": "message_1"},
		"channel":    "push",
		"content":    map[string]any{"title": "Hello"},
	}}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/client/users/inbox", bytes.NewReader(data)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.PostUserInboxMessages(w, req, uuid.Nil)
	return w
}

func publishedIdentifiers(t *testing.T, pub *capturingPublisher) []subjects.ExternalIDParam {
	t.Helper()
	captured := pub.captured()
	require.Len(t, captured, 1, "exactly one inbox message should be published")
	msg, ok := captured[0].Value.(schemas.InboxMessage)
	require.True(t, ok, "published value should be a schemas.InboxMessage")
	return msg.Identifiers
}

// TestPostUserInboxMessages_OwnDataOverridesTarget is the controller-level
// regression test for the inbox IDOR fix: an own-data end user (DataScopeOwn)
// must not be able to address an inbox message to another user by supplying a
// different target — the recipient is overridden to the verified subject.
func TestPostUserInboxMessages_OwnDataOverridesTarget(t *testing.T) {
	t.Parallel()

	actor := rbac.NewActor(rbac.ActorEndUser, uuid.NewString(),
		rbac.WithProjectID(uuid.New()),
		rbac.WithSubject("verified-user", "https://idp.example"),
		rbac.WithScope(rbac.DataScopeOwn),
	)
	srv, pub, ctx := newInboxController(t, actor)

	// Client tries to address the message to a different user ("victim").
	w := postInboxMessage(t, srv, ctx, "victim")
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())

	// The published recipient is bound to the verified subject, not "victim".
	assert.Equal(t, []subjects.ExternalIDParam{
		{Source: "https://idp.example", ExternalID: "verified-user"},
	}, publishedIdentifiers(t, pub))
}

// TestPostUserInboxMessages_AllDataKeepsTarget asserts the inverse: a trusted
// caller acting across users (DataScopeAll) keeps the client-supplied target, so
// backend integrations can still address any user.
func TestPostUserInboxMessages_AllDataKeepsTarget(t *testing.T) {
	t.Parallel()

	actor := rbac.NewActor(rbac.ActorEndUser, uuid.NewString(),
		rbac.WithProjectID(uuid.New()),
		rbac.WithSubject("verified-user", "https://idp.example"),
		// No DataScopeOwn: acts across subjects.
	)
	srv, pub, ctx := newInboxController(t, actor)

	w := postInboxMessage(t, srv, ctx, "any-user")
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())

	// The supplied target is preserved (source defaults to "default").
	got := publishedIdentifiers(t, pub)
	require.Len(t, got, 1)
	assert.Equal(t, "any-user", got[0].ExternalID)
}

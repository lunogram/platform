package v1

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
)

// capturedMessage records a single Publish call so tests can assert on the
// subject and payload that a handler emitted.
type capturedMessage struct {
	Subject schemas.Subject
	Value   any
}

// capturingPublisher is a pubsub.Publisher test double that records every
// published message in memory instead of sending it to a broker. It lets the
// own-data tests inspect exactly which identifiers a handler attached to an
// event, which is where the subject-binding enforcement actually lands.
type capturingPublisher struct {
	mu       sync.Mutex
	messages []capturedMessage
}

func (p *capturingPublisher) Publish(_ context.Context, subject schemas.Subject, v any, _ ...pubsub.PublishOption) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages = append(p.messages, capturedMessage{Subject: subject, Value: v})
	return nil
}

func (p *capturingPublisher) captured() []capturedMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]capturedMessage, len(p.messages))
	copy(out, p.messages)
	return out
}

// withCapturingPublisher swaps the controller's publisher for a capturing double
// and returns it. The handlers under test only need a Publisher, so replacing
// the real NATS publisher keeps these tests fast and lets us assert on payloads
// without standing up a consumer.
func (tc *testClientController) withCapturingPublisher() *capturingPublisher {
	pub := &capturingPublisher{}
	tc.UsersController.ClientController.pubsub = pub
	return pub
}

// ownDataActor builds a verified end-user actor confined to its own data
// (DataScopeOwn) and writes the relationship tuples that let it pass the
// "client" project-role permission checks. It mirrors actorContext but for the
// own-data end-user case the PR exists to constrain.
//
// subject/source are the verified external identity the request must be bound
// to regardless of any client-supplied identifier.
func (tc *testClientController) ownDataActor(t *testing.T, orgID, projectID uuid.UUID, subject, source string) *rbac.Actor {
	t.Helper()

	actor := rbac.NewActor(rbac.ActorEndUser, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
		rbac.WithSubject(subject, source),
		rbac.WithScope(rbac.DataScopeOwn),
	)

	engine, _ := rbac.TestSetup(t, t.Context(), actor, "member", "client")
	tc.UsersController.ClientController.engine = engine

	return actor
}

// allDataActor builds a verified end-user actor whose method is scoped to all
// data (the default DataScopeAll) — a trusted integration that may act across
// subjects with client-supplied identifiers. It is the passthrough counterpart
// to ownDataActor.
func (tc *testClientController) allDataActor(t *testing.T, orgID, projectID uuid.UUID) *rbac.Actor {
	t.Helper()

	actor := rbac.NewActor(rbac.ActorEndUser, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
		rbac.WithSubject("trusted-subject", "https://idp.example"),
	)

	engine, _ := rbac.TestSetup(t, t.Context(), actor, "member", "client")
	tc.UsersController.ClientController.engine = engine

	return actor
}

// newProject creates an organization and project and returns the project ID,
// the convenience the own-data tests repeatedly need.
func (tc *testClientController) newProject(t *testing.T) (orgID, projectID uuid.UUID) {
	t.Helper()

	orgID, err := tc.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	if err != nil {
		t.Fatalf("create organization: %v", err)
	}

	projectID, err = tc.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return orgID, projectID
}

package rbac

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewActorAppliesOptions(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	projectID := uuid.New()

	actor := NewActor(ActorAPIKey, "key-1",
		WithOrganizationID(orgID),
		WithProjectID(projectID),
	)

	assert.Equal(t, ActorAPIKey, actor.Type)
	assert.Equal(t, "key-1", actor.ID)
	assert.Equal(t, orgID, actor.OrganizationID)
	assert.Equal(t, projectID, actor.ProjectID)
}

func TestUserKeyFormatsCorrectly(t *testing.T) {
	t.Parallel()

	actor := NewActor(ActorAdmin, "abc-123")
	assert.Equal(t, "user:abc-123", actor.UserKey())
}

func TestOrganizationScopeFormatsObject(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	scope := OrganizationScope(id)
	assert.Equal(t, "organization:11111111-1111-1111-1111-111111111111", scope)
}

func TestProjectScopeFormatsObject(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	scope := ProjectScope(id)
	assert.Equal(t, "project:22222222-2222-2222-2222-222222222222", scope)
}

func TestProjectResourceScopeFormatsObject(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	scope := ProjectResourceScope("users", id)
	assert.Equal(t, "users:22222222-2222-2222-2222-222222222222", scope)
}

func TestActorRoundTripsViaContext(t *testing.T) {
	t.Parallel()

	actor := NewActor(ActorAdmin, "ctx-user")
	ctx := WithActor(context.Background(), actor)

	got := FromContext(ctx)
	assert.Same(t, actor, got)
}

func TestFromContextReturnsNilWhenAbsent(t *testing.T) {
	t.Parallel()

	assert.Nil(t, FromContext(context.Background()))
}

func TestFromContextReturnsNilOnWrongValueType(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), actorContextKey{}, "not-an-actor")
	assert.Nil(t, FromContext(ctx))
}

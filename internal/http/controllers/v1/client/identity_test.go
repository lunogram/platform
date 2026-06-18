package v1

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/stretchr/testify/assert"
)

func TestBoundUserIdentifiers(t *testing.T) {
	t.Parallel()

	supplied := []subjects.ExternalIDParam{{Source: "crm", ExternalID: "client-supplied"}}

	t.Run("own-data end user is bound to the verified subject, ignoring client input", func(t *testing.T) {
		actor := rbac.NewActor(rbac.ActorEndUser, uuid.NewString(),
			rbac.WithSubject("verified-user", "https://idp.example"),
			rbac.WithOwnData(true))
		ctx := rbac.WithActor(context.Background(), actor)

		got := boundUserIdentifiers(ctx, supplied)
		assert.Equal(t, []subjects.ExternalIDParam{{Source: "https://idp.example", ExternalID: "verified-user"}}, got)
		assert.True(t, isOwnDataScoped(ctx))
	})

	t.Run("all-data end user acts across users with client-supplied identifiers", func(t *testing.T) {
		// A verified end user whose method is scoped to "all" data (e.g. an
		// IdP-authenticated support tool) is not confined to its own subject.
		actor := rbac.NewActor(rbac.ActorEndUser, uuid.NewString(),
			rbac.WithSubject("verified-user", "https://idp.example"))
		ctx := rbac.WithActor(context.Background(), actor)

		assert.Equal(t, supplied, boundUserIdentifiers(ctx, supplied))
		assert.False(t, isOwnDataScoped(ctx))
	})

	t.Run("api key uses the client-supplied identifiers", func(t *testing.T) {
		actor := rbac.NewActor(rbac.ActorAPIKey, uuid.NewString())
		ctx := rbac.WithActor(context.Background(), actor)

		assert.Equal(t, supplied, boundUserIdentifiers(ctx, supplied))
		assert.False(t, isOwnDataScoped(ctx))
	})

	t.Run("no actor falls back to client-supplied identifiers", func(t *testing.T) {
		assert.Equal(t, supplied, boundUserIdentifiers(context.Background(), supplied))
		assert.False(t, isOwnDataScoped(context.Background()))
	})
}

func TestRequireCrossSubjectAccess(t *testing.T) {
	t.Parallel()

	t.Run("own-data end user is denied cross-subject access", func(t *testing.T) {
		actor := rbac.NewActor(rbac.ActorEndUser, uuid.NewString(),
			rbac.WithSubject("verified-user", "https://idp.example"),
			rbac.WithOwnData(true))
		ctx := rbac.WithActor(context.Background(), actor)

		assert.Error(t, requireCrossSubjectAccess(ctx))
	})

	t.Run("all-data end user is allowed cross-subject access", func(t *testing.T) {
		actor := rbac.NewActor(rbac.ActorEndUser, uuid.NewString(),
			rbac.WithSubject("verified-user", "https://idp.example"))
		ctx := rbac.WithActor(context.Background(), actor)

		assert.NoError(t, requireCrossSubjectAccess(ctx))
	})

	t.Run("api key is allowed cross-subject access", func(t *testing.T) {
		ctx := rbac.WithActor(context.Background(), rbac.NewActor(rbac.ActorAPIKey, uuid.NewString()))
		assert.NoError(t, requireCrossSubjectAccess(ctx))
	})
}

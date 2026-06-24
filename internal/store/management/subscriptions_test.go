package management

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionsStore(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	t.Run("creates subscription", func(t *testing.T) {
		subID, err := db.CreateSubscription(ctx, Subscription{
			ProjectID: projectID,
			Name:      "Newsletter",
			Channel:   "email",
			IsPublic:  true,
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, subID)

		sub, err := db.GetSubscription(ctx, projectID, subID)
		require.NoError(t, err)
		assert.Equal(t, "Newsletter", sub.Name)
		assert.Equal(t, "email", sub.Channel)
		assert.True(t, sub.IsPublic)
	})

	t.Run("lists subscriptions", func(t *testing.T) {
		_, err := db.CreateSubscription(ctx, Subscription{
			ProjectID: projectID,
			Name:      "SMS Alerts",
			Channel:   "sms",
			IsPublic:  false,
		})
		require.NoError(t, err)

		subs, total, err := db.ListSubscriptions(ctx, projectID, store.Pagination{Limit: 10, Offset: 0})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 1)
		assert.GreaterOrEqual(t, len(subs), 1)
	})

	t.Run("IsUserUnsubscribed", func(t *testing.T) {
		subID, err := db.CreateSubscription(ctx, Subscription{
			ProjectID: projectID,
			Name:      "Promotions",
			Channel:   "email",
			IsPublic:  true,
		})
		require.NoError(t, err)

		userID := uuid.New()

		t.Run("returns false when no record exists", func(t *testing.T) {
			unsubscribed, err := db.IsUserUnsubscribed(ctx, userID, subID)
			require.NoError(t, err)
			assert.False(t, unsubscribed)
		})

		t.Run("returns true after unsubscribe", func(t *testing.T) {
			require.NoError(t, db.Unsubscribe(ctx, userID, subID))

			unsubscribed, err := db.IsUserUnsubscribed(ctx, userID, subID)
			require.NoError(t, err)
			assert.True(t, unsubscribed)
		})

		t.Run("returns false after re-subscribe", func(t *testing.T) {
			require.NoError(t, db.Subscribe(ctx, userID, subID))

			unsubscribed, err := db.IsUserUnsubscribed(ctx, userID, subID)
			require.NoError(t, err)
			assert.False(t, unsubscribed)
		})
	})
}

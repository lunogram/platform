package consumer

import (
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/stretchr/testify/require"
)

func TestBuildRenderDataIncludesUnsubscribeURLWhenSubscriptionLinked(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	campaignID := uuid.New()
	subscriptionID := uuid.New()
	userID := uuid.New()

	campaign := &management.Campaign{
		ID:             campaignID,
		ProjectID:      projectID,
		Transactional:  false,
		SubscriptionID: &subscriptionID,
	}
	user := &subjects.User{ID: userID}

	data := buildRenderData("https://example.com/", user, campaign, nil)

	rawUnsubscribeURL, ok := data["unsubscribe_url"]
	require.True(t, ok)

	unsubscribeURL, ok := rawUnsubscribeURL.(string)
	require.True(t, ok)

	parsedURL, err := url.Parse(unsubscribeURL)
	require.NoError(t, err)
	require.Equal(t, "/unsubscribe/email", parsedURL.Path)

	link := parsedURL.Query().Get("link")
	require.NotEmpty(t, link)

	parsedLink, err := url.Parse(link)
	require.NoError(t, err)
	require.Equal(t, userID.String(), parsedLink.Query().Get("u"))
	require.Equal(t, campaignID.String(), parsedLink.Query().Get("c"))
}

func TestBuildRenderDataOmitsUnsubscribeURLWhenNotEligible(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	userID := uuid.New()
	user := &subjects.User{ID: userID}

	tests := map[string]*management.Campaign{
		"transactional campaign": {
			ID:             uuid.New(),
			ProjectID:      projectID,
			Transactional:  true,
			SubscriptionID: func() *uuid.UUID { id := uuid.New(); return &id }(),
		},
		"no linked subscription": {
			ID:             uuid.New(),
			ProjectID:      projectID,
			Transactional:  false,
			SubscriptionID: nil,
		},
	}

	for name, campaign := range tests {
		campaign := campaign
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data := buildRenderData("https://example.com", user, campaign, nil)
			_, ok := data["unsubscribe_url"]
			require.False(t, ok)
		})
	}
}

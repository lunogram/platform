package journeys

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/require"
)

// A campaign step resolves its variant against the journey context and hands
// the send a static selector, because the context it reads no longer exists at
// render time. An empty result is still a decision - the step asked for house
// branding, or its expression matched nothing - and has to reach the send as a
// pinned default rather than falling through to the campaign's own selector.
func TestHandleCampaignVariant(t *testing.T) {
	t.Parallel()

	campaignID := uuid.New()

	tests := map[string]struct {
		variant *oapi.VariantSelector
		data    map[string]any
		want    *management.VariantSelector
	}{
		"no selector defers to the campaign": {
			variant: nil,
			want:    nil,
		},
		"static key is pinned as written": {
			variant: &oapi.VariantSelector{Type: "static", Key: ptr.To("acme")},
			want:    &management.VariantSelector{Type: management.VariantSelectorStatic, Key: "acme"},
		},
		"empty static key pins the default variant": {
			variant: &oapi.VariantSelector{Type: "static", Key: ptr.To("")},
			want:    &management.VariantSelector{Type: management.VariantSelectorStatic, Key: ""},
		},
		"expression resolves against the journey context": {
			variant: &oapi.VariantSelector{Type: "expression", Expression: ptr.To("{{ user.data.tenant }}")},
			data:    map[string]any{"user": map[string]any{"data": map[string]any{"tenant": "acme"}}},
			want:    &management.VariantSelector{Type: management.VariantSelectorStatic, Key: "acme"},
		},
		"expression matching nothing still pins the default": {
			variant: &oapi.VariantSelector{Type: "expression", Expression: ptr.To("{{ user.data.tenant }}")},
			data:    map[string]any{"user": map[string]any{"data": map[string]any{}}},
			want:    &management.VariantSelector{Type: management.VariantSelectorStatic, Key: ""},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(oapi.CampaignStepData{
				CampaignId: campaignID,
				Variant:    test.variant,
			})
			require.NoError(t, err)

			pub := &mockPublisher{}
			hctx := HandlerContext{
				Context:   context.Background(),
				Publisher: pub,
				ProjectID: uuid.New(),
				UserID:    uuid.New(),
				Data:      test.data,
			}

			_, _, err = HandleCampaign(hctx, journey.JourneyVersionStep{Data: data}, journey.JourneyUserState{})
			require.NoError(t, err)
			require.Len(t, pub.publishedEvents, 1)

			msg, ok := pub.publishedEvents[0].data.(schemas.SendCampaign)
			require.True(t, ok)

			if test.want == nil {
				require.Nil(t, msg.Variant)
				return
			}
			require.Equal(t, *test.want, *msg.Variant)
		})
	}
}

package consumer

import (
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func template(variant, locale string) management.Template {
	return management.Template{ID: uuid.New(), Variant: variant, Locale: locale}
}

func TestSelectTemplate(t *testing.T) {
	t.Parallel()

	house := template("", "en")
	houseNL := template("", "nl")
	acme := template("acme", "en")
	acmeNL := template("acme", "nl")

	project := &management.Project{Locale: "en"}

	tests := []struct {
		name      string
		templates management.Templates
		variant   string
		user      *subjects.User
		want      management.Template
	}{
		{
			name:      "picks the requested variant over the default",
			templates: management.Templates{house, acme},
			variant:   "acme",
			user:      &subjects.User{Locale: ptr.To("en")},
			want:      acme,
		},
		{
			name:      "applies locale within the requested variant",
			templates: management.Templates{house, houseNL, acme, acmeNL},
			variant:   "acme",
			user:      &subjects.User{Locale: ptr.To("nl")},
			want:      acmeNL,
		},
		{
			name:      "falls back to the project locale within the variant",
			templates: management.Templates{house, acme, acmeNL},
			variant:   "acme",
			user:      &subjects.User{Locale: ptr.To("de")},
			want:      acme,
		},
		{
			name:      "falls back to the default variant when the variant has no template",
			templates: management.Templates{house, houseNL},
			variant:   "acme",
			user:      &subjects.User{Locale: ptr.To("nl")},
			want:      houseNL,
		},
		{
			// The single-template shortcut must not run before the variant
			// filter, or a campaign holding one variant template would answer
			// every send with it.
			name:      "does not hand a lone variant template to a default send",
			templates: management.Templates{acme, house},
			variant:   "",
			user:      nil,
			want:      house,
		},
		{
			// No on-brand answer exists. Sending the wrong branding beats
			// dropping the message; the caller counts this as a fallback.
			name:      "sends something when neither the variant nor the default has a template",
			templates: management.Templates{acme},
			variant:   "globex",
			user:      nil,
			want:      acme,
		},
		{
			name:      "uses the only template when it is the default variant",
			templates: management.Templates{house},
			variant:   "",
			user:      nil,
			want:      house,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := selectTemplate(test.templates, test.variant, test.user, project)
			require.NoError(t, err)
			require.Equal(t, test.want.ID, got.ID)
		})
	}
}

func TestSelectTemplateWithoutTemplates(t *testing.T) {
	t.Parallel()

	_, err := selectTemplate(nil, "", nil, &management.Project{Locale: "en"})
	require.Error(t, err)
}

func TestResolveVariant(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	data := map[string]any{
		"user": map[string]any{
			"data": map[string]any{"tenant": "acme"},
		},
	}

	tests := []struct {
		name     string
		selector *string
		event    schemas.SendCampaign
		want     string
	}{
		{
			name:     "explicit variant on the event wins over the selector",
			selector: ptr.To("{{ user.data.tenant }}"),
			event:    schemas.SendCampaign{Variant: ptr.To("globex")},
			want:     "globex",
		},
		{
			name:     "renders the campaign selector when the event names nothing",
			selector: ptr.To("{{ user.data.tenant }}"),
			want:     "acme",
		},
		{
			name:     "trims whitespace a Liquid expression leaves behind",
			selector: ptr.To("  {{ user.data.tenant }}  "),
			want:     "acme",
		},
		{
			name:     "resolves to the default variant when the selector matches nothing",
			selector: ptr.To("{{ user.data.missing }}"),
			want:     "",
		},
		{
			name: "resolves to the default variant when no selector is configured",
			want: "",
		},
		{
			name:     "a broken expression resolves to the default variant rather than failing the send",
			selector: ptr.To("{{ user.data.tenant | }}"),
			want:     "",
		},
		{
			// A selector with no Liquid in it is a static pin, which is how a
			// campaign sends every message under one brand.
			name:     "a literal selector is used as the variant key",
			selector: ptr.To("acme"),
			want:     "acme",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			campaign := &management.Campaign{VariantSelector: test.selector}
			require.Equal(t, test.want, resolveVariant(logger, campaign, test.event, data))
		})
	}
}

func TestCampaignVariantsHas(t *testing.T) {
	t.Parallel()

	variants := management.CampaignVariants{{Key: "acme"}, {Key: "globex"}}

	require.True(t, variants.Has("acme"))
	require.True(t, variants.Has(""), "the default variant always exists")
	require.False(t, variants.Has("initech"))
	require.True(t, management.CampaignVariants{}.Has(""))
}

package consumer

import (
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store"
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

	expression := func(value string) *management.VariantSelector {
		return &management.VariantSelector{
			Type:       management.VariantSelectorExpression,
			Expression: value,
		}
	}
	static := func(key string) *management.VariantSelector {
		return &management.VariantSelector{
			Type: management.VariantSelectorStatic,
			Key:  key,
		}
	}

	tests := []struct {
		name     string
		campaign *management.VariantSelector
		event    *management.VariantSelector
		want     string
	}{
		{
			name:     "the event selector wins over the campaign selector",
			campaign: expression("{{ user.data.tenant }}"),
			event:    static("globex"),
			want:     "globex",
		},
		{
			name:     "renders the campaign expression when the event carries nothing",
			campaign: expression("{{ user.data.tenant }}"),
			want:     "acme",
		},
		{
			// Pinning the default variant has to beat the campaign's selector
			// rather than read as "nothing set" and fall through to it - this
			// is what forces one send back to house branding inside an
			// otherwise white-labelled campaign.
			name:     "an event pinning the default variant overrides the campaign expression",
			campaign: expression("{{ user.data.tenant }}"),
			event:    static(""),
			want:     "",
		},
		{
			name:     "trims whitespace an expression leaves behind",
			campaign: expression("  {{ user.data.tenant }}  "),
			want:     "acme",
		},
		{
			name:     "a static campaign selector pins every recipient",
			campaign: static("acme"),
			want:     "acme",
		},
		{
			name:     "an expression matching nothing resolves to the default variant",
			campaign: expression("{{ user.data.missing }}"),
			want:     "",
		},
		{
			name: "no selector anywhere resolves to the default variant",
			want: "",
		},
		{
			name:     "a broken expression resolves to the default variant rather than failing the send",
			campaign: expression("{{ user.data.tenant | }}"),
			want:     "",
		},
		{
			// Static never renders, so a key that happens to look like Liquid
			// is still used verbatim rather than evaluated.
			name:     "a static key is never rendered",
			campaign: static("{{ user.data.tenant }}"),
			want:     "{{ user.data.tenant }}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			campaign := &management.Campaign{
				Variants: store.JSONB[management.CampaignVariants]{
					Data: management.CampaignVariants{Selector: test.campaign},
				},
			}
			event := schemas.SendCampaign{Variant: test.event}
			require.Equal(t, test.want, resolveVariant(logger, campaign, event, data))
		})
	}
}

func TestCampaignVariantsHas(t *testing.T) {
	t.Parallel()

	variants := management.CampaignVariants{
		Options: []management.CampaignVariant{{Key: "acme"}, {Key: "globex"}},
	}

	require.True(t, variants.Has("acme"))
	require.True(t, variants.Has(""), "the default variant always exists")
	require.False(t, variants.Has("initech"))
	require.True(t, management.CampaignVariants{}.Has(""))
}

func TestVariantSelectorValidate(t *testing.T) {
	t.Parallel()

	variants := management.CampaignVariants{
		Options: []management.CampaignVariant{{Key: "acme"}},
	}

	tests := map[string]struct {
		selector management.VariantSelector
		wantErr  bool
	}{
		"declared static key": {
			selector: management.VariantSelector{Type: management.VariantSelectorStatic, Key: "acme"},
		},
		"undeclared static key": {
			selector: management.VariantSelector{Type: management.VariantSelectorStatic, Key: "globex"},
			wantErr:  true,
		},
		"static without a key pins the default variant": {
			selector: management.VariantSelector{Type: management.VariantSelectorStatic},
		},
		"expression": {
			selector: management.VariantSelector{Type: management.VariantSelectorExpression, Expression: "{{ user.data.tenant }}"},
		},
		"expression without an expression": {
			selector: management.VariantSelector{Type: management.VariantSelectorExpression},
			wantErr:  true,
		},
		"unknown type": {
			selector: management.VariantSelector{Type: "whatever"},
			wantErr:  true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := test.selector.Validate(variants)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

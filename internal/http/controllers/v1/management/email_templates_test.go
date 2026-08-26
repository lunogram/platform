package v1

import (
	"testing"

	"github.com/lunogram/platform/internal/gallery"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToEmailTemplateList(t *testing.T) {
	t.Parallel()

	t.Run("maps every field", func(t *testing.T) {
		blocks := map[string]any{"root": "value"}
		listing := &gallery.Listing{
			Total:  1,
			Limit:  10,
			Offset: 20,
			Results: []gallery.Template{{
				ID:          "welcome",
				Label:       "Welcome",
				Description: ptr.To("A welcome email"),
				HTML:        ptr.To("<p>hi</p>"),
				Text:        ptr.To("hi"),
				Thumbnail:   ptr.To("https://cdn.example.com/welcome.png"),
				Blocks:      &blocks,
			}},
		}

		got := toEmailTemplateList(listing)

		assert.Equal(t, 1, got.Total)
		assert.Equal(t, 10, got.Limit)
		assert.Equal(t, 20, got.Offset)
		require.Len(t, got.Results, 1)

		result := got.Results[0]
		assert.Equal(t, "welcome", result.Id)
		assert.Equal(t, "Welcome", result.Label)
		assert.Equal(t, "A welcome email", *result.Description)
		assert.Equal(t, "<p>hi</p>", *result.Html)
		assert.Equal(t, "hi", *result.Text)
		assert.Equal(t, "https://cdn.example.com/welcome.png", *result.Thumbnail)
		require.NotNil(t, result.Blocks)
		assert.Equal(t, "value", (*result.Blocks)["root"])
	})

	t.Run("an empty gallery serialises as an empty array, not null", func(t *testing.T) {
		got := toEmailTemplateList(&gallery.Listing{})
		assert.NotNil(t, got.Results)
		assert.Empty(t, got.Results)
	})
}

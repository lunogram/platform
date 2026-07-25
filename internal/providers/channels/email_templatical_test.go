package channels

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingCaller stands in for the Deno renderer, capturing what the backend
// asked it to compile so tests can assert on the source selection.
type recordingCaller struct {
	compiledSource string
	html           string
	plainText      string
}

func (c *recordingCaller) Call(_ context.Context, subject schemas.Subject, v any) ([]byte, error) {
	switch {
	case strings.HasPrefix(string(subject), "email.compile."):
		req, ok := v.(schemas.CompileEmail)
		if !ok {
			return nil, assertUnexpected("compile payload")
		}
		c.compiledSource = req.Source
		return json.Marshal(schemas.CompileEmailResponse{CompiledJS: `{"kind":"templatical"}`})

	case strings.HasPrefix(string(subject), "email.render."):
		return json.Marshal(schemas.RenderEmailResponse{HTML: c.html, PlainText: c.plainText})
	}
	return nil, assertUnexpected(string(subject))
}

type assertUnexpected string

func (e assertUnexpected) Error() string { return "unexpected call: " + string(e) }

func TestComposeEmailTemplateData_Templatical(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	document := `{"blocks":[{"type":"title","content":"Hi {{ user.first_name }}"}],"settings":{}}`

	data, err := json.Marshal(map[string]any{
		"subject": "Welcome",
		"type":    TemplateTypeTemplatical,
		"blocks":  json.RawMessage(document),
	})
	require.NoError(t, err)

	caller := &recordingCaller{
		html:      "<html>Hi {{ user.first_name }}</html>",
		plainText: "Hi {{ user.first_name }}",
	}
	renderer := pubsub.NewEmailRenderer(caller)

	out, err := ComposeEmailTemplateData(context.Background(), renderer, projectID, data, nil)
	require.NoError(t, err)

	// The visual document is what gets compiled, not the (absent) JSX source.
	assert.JSONEq(t, document, caller.compiledSource)

	var result map[string]any
	require.NoError(t, json.Unmarshal(out, &result))

	assert.Equal(t, caller.html, result["html"])
	assert.Equal(t, caller.plainText, result["text"])

	// The document must not survive into the payload: the downstream Liquid
	// pass runs over this JSON and would try to evaluate the {{ }} inside it.
	// `code` remains as an empty object — encoding/json's omitempty does not
	// drop zero-valued structs — but carries no source or bundle.
	assert.NotContains(t, result, "blocks")
	assert.Empty(t, result["code"])
	assert.NotContains(t, string(out), document)
}

func TestComposeEmailTemplateData_ReactEmailUnchanged(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	source := "export default function Email() { return null }"

	data, err := json.Marshal(map[string]any{
		"subject": "Welcome",
		"code":    map[string]any{"source": source},
	})
	require.NoError(t, err)

	caller := &recordingCaller{html: "<html></html>", plainText: ""}
	renderer := pubsub.NewEmailRenderer(caller)

	out, err := ComposeEmailTemplateData(context.Background(), renderer, projectID, data, nil)
	require.NoError(t, err)

	// A template with no explicit type still compiles its JSX, exactly as
	// before the type field existed.
	assert.Equal(t, source, caller.compiledSource)

	var result map[string]any
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, caller.html, result["html"])
	assert.Empty(t, result["code"])
	assert.NotContains(t, string(out), source)
}

func TestComposeEmailTemplateData_NothingToCompile(t *testing.T) {
	t.Parallel()

	data := []byte(`{"subject":"Plain","html":"<p>already rendered</p>"}`)

	caller := &recordingCaller{}
	out, err := ComposeEmailTemplateData(
		context.Background(), pubsub.NewEmailRenderer(caller), uuid.New(), data, nil,
	)
	require.NoError(t, err)

	assert.Empty(t, caller.compiledSource)
	assert.JSONEq(t, string(data), string(out))
}

func TestEmailTemplateData_CompileSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data EmailTemplateData
		want string
	}{
		{
			name: "templatical uses the document",
			data: EmailTemplateData{
				Type:   TemplateTypeTemplatical,
				Blocks: json.RawMessage(`{"blocks":[]}`),
				Code:   EmailCodeData{Source: "ignored"},
			},
			want: `{"blocks":[]}`,
		},
		{
			name: "explicit react-email uses the JSX",
			data: EmailTemplateData{
				Type: TemplateTypeReactEmail,
				Code: EmailCodeData{Source: "jsx"},
			},
			want: "jsx",
		},
		{
			name: "absent type falls back to the JSX",
			data: EmailTemplateData{Code: EmailCodeData{Source: "jsx"}},
			want: "jsx",
		},
		{
			name: "templatical without a document compiles nothing",
			data: EmailTemplateData{Type: TemplateTypeTemplatical},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.data.CompileSource())
		})
	}
}

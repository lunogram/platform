package webhook

import (
	"embed"
	"fmt"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/lunogram/platform/internal/configfile"
)

//go:embed templates/*.jsonnet
var defaultTemplates embed.FS

// Template is a parsed JSONNet body template.
//
// The template is a JSONNet function of one argument — the event context — as
// Ory's web hooks are:
//
//	function(ctx) {
//	  event: ctx.event,
//	  project: ctx.payload.project,
//	}
//
// Parsing happens once, at configuration load, so a syntax error surfaces at
// boot. Evaluation happens per dispatch against a fresh VM, because a jsonnet
// VM carries per-evaluation state and is not safe to share across concurrent
// dispatches; the AST, which is the expensive half, is what gets reused.
type Template struct {
	name string
	node ast.Node
}

// ParseTemplate resolves a template reference and parses it.
//
// ref is a base64:// payload, a file:// URL or an inline JSONNet snippet.
// Relative file paths are resolved against baseDir, which is normally the
// directory holding the configuration file, so a config and its templates can
// be shipped together and moved together.
func ParseTemplate(name, ref, baseDir string) (*Template, error) {
	raw, err := configfile.Resolve("template "+name, ref, baseDir)
	if err != nil {
		return nil, err
	}

	snippet := string(raw)
	filename := name
	if path, ok := strings.CutPrefix(ref, configfile.FileScheme); ok {
		filename = path
	}

	if strings.TrimSpace(snippet) == "" {
		return nil, fmt.Errorf("template %s: is empty", name)
	}

	node, parseErr := jsonnet.SnippetToAST(filename, snippet)
	if parseErr != nil {
		return nil, fmt.Errorf("template %s: %w", name, parseErr)
	}

	return &Template{name: name, node: node}, nil
}

// defaultTemplate returns the embedded template shipped for an event, so a
// deployment that configures a hook without a body still produces the
// documented wire shape for that event.
func defaultTemplate(event string) (*Template, error) {
	raw, err := defaultTemplates.ReadFile("templates/" + event + ".jsonnet")
	if err != nil {
		return nil, fmt.Errorf("no default body template for event %q", event)
	}
	node, parseErr := jsonnet.SnippetToAST(event+".jsonnet", string(raw))
	if parseErr != nil {
		return nil, fmt.Errorf("default template %s: %w", event, parseErr)
	}
	return &Template{name: event + " (default)", node: node}, nil
}

// Render evaluates the template against the JSON-encoded event context and
// returns the request body.
func (t *Template) Render(ctxJSON []byte) ([]byte, error) {
	vm := jsonnet.MakeVM()
	// The engine renders only operator-supplied templates, but an importer that
	// can reach the filesystem would turn a template into an arbitrary file
	// read. Templates get their data through ctx, not through imports.
	vm.Importer(&jsonnet.MemoryImporter{})
	vm.TLACode("ctx", string(ctxJSON))

	out, err := vm.Evaluate(t.node)
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", t.name, err)
	}
	return []byte(out), nil
}

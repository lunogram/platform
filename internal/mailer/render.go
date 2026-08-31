package mailer

import (
	"embed"
	"fmt"
	"html/template"
	"net/url"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/lunogram/platform/internal/configfile"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// Renderer turns an auth event into a [Message]. It is safe for concurrent use.
//
// Every template is resolved and parsed once, when the renderer is built, so a
// broken override stops the service at boot rather than at the first
// registration. An override that is simply absent is not broken: the embedded
// default takes its place.
type Renderer struct {
	layoutHTML *template.Template
	layoutText *texttemplate.Template

	kinds map[string]*kindTemplates

	productName string
	baseURL     string
}

// kindTemplates is one message's copy, parsed.
type kindTemplates struct {
	subject     *texttemplate.Template
	heading     *texttemplate.Template
	actionLabel *texttemplate.Template
	footer      *texttemplate.Template
	body        []*texttemplate.Template
}

// layoutData is what the layout is evaluated against: the message context plus
// the copy, already rendered.
//
// The copy arrives as plain strings and the HTML layout escapes them. An
// operator who wants markup in the body overrides the layout, which is the
// template that is allowed to emit HTML; keeping the copy escaped means a value
// that reaches it from a request -- an address, a product name -- cannot inject
// markup into somebody's mailbox.
type layoutData struct {
	TemplateData
	Subject     string
	Heading     string
	Body        []string
	ActionLabel string
	Footer      string
}

// NewRenderer resolves the configured templates over the embedded defaults.
//
// baseDir is the directory the configuration file was read from; relative
// file:// references resolve against it.
func NewRenderer(config Config, baseURL, baseDir string) (*Renderer, error) {
	productName := config.ProductName
	if productName == "" {
		productName = "Lunogram"
	}

	r := &Renderer{
		kinds:       make(map[string]*kindTemplates, len(defaultContent)),
		productName: productName,
		baseURL:     strings.TrimRight(baseURL, "/"),
	}

	var err error
	if r.layoutHTML, err = r.layoutHTMLTemplate(config.Templates.Layout.HTML, baseDir); err != nil {
		return nil, err
	}
	if r.layoutText, err = r.layoutTextTemplate(config.Templates.Layout.Text, baseDir); err != nil {
		return nil, err
	}

	for kind, fallback := range defaultContent {
		parsed, err := parseContent(kind, config.Templates.override(kind), fallback, baseDir)
		if err != nil {
			return nil, err
		}
		r.kinds[kind] = parsed
	}

	return r, nil
}

func (r *Renderer) layoutHTMLTemplate(ref, baseDir string) (*template.Template, error) {
	raw, err := resolveOrEmbedded("mail.templates.layout.html", ref, "templates/layout.html.tmpl", baseDir)
	if err != nil {
		return nil, err
	}
	parsed, err := template.New("layout.html").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("mail.templates.layout.html: %w", err)
	}
	return parsed, nil
}

func (r *Renderer) layoutTextTemplate(ref, baseDir string) (*texttemplate.Template, error) {
	raw, err := resolveOrEmbedded("mail.templates.layout.text", ref, "templates/layout.txt.tmpl", baseDir)
	if err != nil {
		return nil, err
	}
	parsed, err := texttemplate.New("layout.txt").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("mail.templates.layout.text: %w", err)
	}
	return parsed, nil
}

// resolveOrEmbedded resolves an operator reference, or reads the embedded
// default when none was configured.
//
// The asymmetry is the point: an absent override is a deployment that did not
// ask for anything, and it gets what ships. A present one that cannot be
// resolved is a deployment that asked for something specific and got it wrong,
// and falling back there would send the wrong mail with nothing in the logs to
// explain why.
func resolveOrEmbedded(name, ref, embedded, baseDir string) ([]byte, error) {
	if strings.TrimSpace(ref) == "" {
		return templateFS.ReadFile(embedded)
	}
	return configfile.Resolve(name, ref, baseDir)
}

// parseContent parses one message's copy, taking each field from the operator
// when they set it and from the embedded default when they did not.
func parseContent(kind string, override, fallback Content, baseDir string) (*kindTemplates, error) {
	parsed := &kindTemplates{}

	fields := []struct {
		name     string
		override string
		fallback string
		into     **texttemplate.Template
	}{
		{"subject", override.Subject, fallback.Subject, &parsed.subject},
		{"heading", override.Heading, fallback.Heading, &parsed.heading},
		{"action_label", override.ActionLabel, fallback.ActionLabel, &parsed.actionLabel},
		{"footer", override.Footer, fallback.Footer, &parsed.footer},
	}

	for _, field := range fields {
		name := fmt.Sprintf("mail.templates.%s.%s", kind, field.name)
		tmpl, err := parseField(name, field.override, field.fallback, baseDir)
		if err != nil {
			return nil, err
		}
		*field.into = tmpl
	}

	body := override.Body
	if len(body) == 0 {
		body = fallback.Body
	}
	for i, paragraph := range body {
		name := fmt.Sprintf("mail.templates.%s.body[%d]", kind, i)
		tmpl, err := parseField(name, paragraph, "", baseDir)
		if err != nil {
			return nil, err
		}
		parsed.body = append(parsed.body, tmpl)
	}

	return parsed, nil
}

func parseField(name, override, fallback, baseDir string) (*texttemplate.Template, error) {
	source := fallback
	if strings.TrimSpace(override) != "" {
		raw, err := configfile.Resolve(name, override, baseDir)
		if err != nil {
			return nil, err
		}
		source = string(raw)
	}

	parsed, err := texttemplate.New(name).Parse(source)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

// BaseURL is the public origin every link in an auth message is built from.
func (r *Renderer) BaseURL() string { return r.baseURL }

// link builds an absolute console URL carrying a single-use token.
func (r *Renderer) link(path, token string) string {
	return r.baseURL + path + "?token=" + url.QueryEscape(token)
}

func (r *Renderer) VerifyEmail(to, token string, ttl time.Duration) Message {
	return r.render(KindVerifyEmail, to, r.link("/verify-email", token), ttl)
}

func (r *Renderer) PasswordReset(to, token string, ttl time.Duration) Message {
	return r.render(KindPasswordReset, to, r.link("/reset-password", token), ttl)
}

func (r *Renderer) AccountExists(to, resetToken string, ttl time.Duration) Message {
	return r.render(KindAccountExists, to, r.link("/reset-password", resetToken), ttl)
}

func (r *Renderer) PasswordChanged(to string) Message {
	return r.render(KindPasswordChanged, to, "", 0)
}

func (r *Renderer) render(kind, to, actionURL string, ttl time.Duration) Message {
	data := TemplateData{
		Kind:        kind,
		ProductName: r.productName,
		Recipient:   to,
		ActionURL:   actionURL,
		BaseURL:     r.baseURL,
	}
	if ttl > 0 {
		data.ExpiresIn = humaniseTTL(ttl)
	}

	templates := r.kinds[kind]
	layout := layoutData{
		TemplateData: data,
		Subject:      execute(templates.subject, data),
		Heading:      execute(templates.heading, data),
		ActionLabel:  execute(templates.actionLabel, data),
		Footer:       execute(templates.footer, data),
	}
	for _, paragraph := range templates.body {
		layout.Body = append(layout.Body, execute(paragraph, data))
	}

	var html, text strings.Builder
	// Both layouts were parsed at construction and every value is a plain
	// string, so execution cannot fail for a reason the caller could act on. A
	// partial body is still better than no mail at all.
	_ = r.layoutHTML.Execute(&html, layout)
	_ = r.layoutText.Execute(&text, layout)

	return Message{
		To:        to,
		Kind:      kind,
		Subject:   layout.Subject,
		HTML:      html.String(),
		Text:      text.String(),
		ActionURL: actionURL,
	}
}

func execute(tmpl *texttemplate.Template, data TemplateData) string {
	var out strings.Builder
	_ = tmpl.Execute(&out, data)
	return out.String()
}

func humaniseTTL(ttl time.Duration) string {
	switch {
	case ttl >= 48*time.Hour:
		return fmt.Sprintf("%d days", int(ttl.Hours()/24))
	case ttl >= 2*time.Hour:
		return fmt.Sprintf("%d hours", int(ttl.Hours()))
	case ttl >= time.Hour:
		return "one hour"
	default:
		return fmt.Sprintf("%d minutes", int(ttl.Minutes()))
	}
}

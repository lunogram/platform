# Subscription Preference Templates

This directory contains the server-side rendered HTML templates for subscription preferences management.

## Structure

- `unsubscribe.html` - Email unsubscribe confirmation page
- `preferences.html` - Subscription preferences management page
- `input.css` - Tailwind CSS input file
- `tailwind.config.js` - Tailwind configuration
- `templates.go` - Go template package with embedded files
- `generate.go` - Go generate directives for building CSS
- `static/styles.css` - Generated CSS file (auto-generated, not committed)

## Building

The templates use Tailwind CSS for styling. To build the CSS:

```bash
# Generate CSS using go generate
cd services/nexus
go generate ./internal/http/controllers/v1/public/templates

# Or manually with tailwindcss CLI
cd services/nexus/internal/http/controllers/v1/public/templates
tailwindcss -i ./input.css -o ./static/styles.css --minify
```

The generated `static/styles.css` file is embedded in the Go binary at compile time using `//go:embed`.

## Templates

### Unsubscribe Template

Shows a confirmation message when a user unsubscribes from an email campaign via an unsubscribe link.

**URL**: `/unsubscribe/email?user_id={uuid}&campaign_id={uuid}`

**Localization**: Supports English, Spanish, French, German, Portuguese, and Italian based on user's locale.

### Preferences Template

Shows a form where users can manage their subscription preferences across all public subscriptions in a project.

**URL**: `/preferences/{userID}?project_id={uuid}`

**Features**:
- Lists all public subscriptions
- Checkboxes to opt in/out of each subscription
- Success message after updating preferences
- HTMX integration for dynamic updates (ready for future enhancement)
- Responsive design with Tailwind CSS

## Localization

The templates support multiple languages:
- English (en)
- Spanish (es)
- French (fr)
- German (de)
- Portuguese (pt)
- Italian (it)

Strings are provided by the `GetStrings()` function in `templates.go` based on the user's locale.

## Development

When modifying templates:

1. Edit the HTML files (`unsubscribe.html`, `preferences.html`)
2. Update Tailwind classes as needed
3. Regenerate CSS: `go generate ./internal/http/controllers/v1/public/templates`
4. Rebuild the Go binary to embed the new templates

The CSS is automatically minified during the build process.

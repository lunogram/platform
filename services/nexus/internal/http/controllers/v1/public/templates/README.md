# Subscription Preference Templates

This directory contains the server-side rendered HTML templates for subscription preferences management.

## Building

The templates use Tailwind CSS for styling. To build:

```bash
make generate
```

This will regenerate the CSS and run all go:generate directives. The generated `static/styles.css` file is embedded in the Go binary at compile time using `//go:embed`.

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

The templates support multiple languages. Strings are provided by the `GetStrings()` function in `templates.go` based on the user's locale.

## Development

When modifying templates:

1. Edit the HTML files (`unsubscribe.html`, `preferences.html`)
2. Update Tailwind classes as needed
3. Run `make generate` to regenerate CSS and templates
4. Rebuild the Go binary to embed the new templates

The CSS is automatically minified during the build process.

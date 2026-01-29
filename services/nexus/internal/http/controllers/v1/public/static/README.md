# Static Assets

This directory contains static assets for the subscription management UI.

## Tailwind CSS

The `styles.css` file is compiled from `input.css` using Tailwind CSS.

### Rebuilding CSS

To rebuild the CSS file after making changes to templates or Tailwind configuration:

```bash
# From the services/nexus directory
npx tailwindcss@latest -i ./internal/http/controllers/v1/public/static/input.css \
  -o ./internal/http/controllers/v1/public/static/styles.css \
  --config ./internal/http/controllers/v1/public/static/tailwind.config.js \
  --minify
```

Or use the standalone Tailwind CLI binary:

```bash
# Download the binary if not already present
curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
chmod +x tailwindcss-linux-x64

# Build CSS
./tailwindcss-linux-x64 -i ./internal/http/controllers/v1/public/static/input.css \
  -o ./internal/http/controllers/v1/public/static/styles.css \
  --config ./internal/http/controllers/v1/public/static/tailwind.config.js \
  --minify
```

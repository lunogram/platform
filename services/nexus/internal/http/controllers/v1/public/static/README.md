# Static Assets

This directory contains static assets for the subscription management UI.

## Tailwind CSS

The `styles.css` file is compiled from `input.css` using Tailwind CSS.

### Automatic Rebuilding

The CSS is automatically rebuilt when you run:

```bash
make generate
```

This command:
1. Installs npm dependencies (including tailwindcss) via `pnpm install`
2. Runs `go generate` which invokes `pnpm exec tailwindcss` to compile the CSS

### Manual Rebuilding

If you need to manually rebuild the CSS:

```bash
# From the repository root
pnpm exec tailwindcss -i ./services/nexus/internal/http/controllers/v1/public/static/input.css \
  -o ./services/nexus/internal/http/controllers/v1/public/static/styles.css \
  --config ./services/nexus/internal/http/controllers/v1/public/static/tailwind.config.js \
  --minify
```

### Requirements

- Node.js and pnpm must be installed
- Tailwind CSS is included as a dev dependency in the root `package.json`
- The CSS compilation is integrated into the `make generate` toolchain

# Modules

> 🚧 **Early Alpha** - Under active development.

WASM plugins that extend Lunogram. Currently used for **providers** (email, SMS, push integrations) and **actions** (side effects triggered by journeys).

## Why WASM?

Sandboxed, portable, fast. Write in any language that compiles to WASM.

---

## Providers

Providers integrate communication channels. Each provider implements:

- `manifest()` — Returns metadata and config schema
- `send()` — Sends a message via the provider's API

### Built-in

| Provider | Channels | Description |
|----------|----------|-------------|
| `logger` | all | Debug provider (logs messages) |
| `resend` | email | [Resend](https://resend.com) integration |
| `twilio` | email, sms | [Twilio](https://twilio.com) integration |

### Creating a Provider

See `providers/logger/` for a minimal example, or `providers/resend/` for a real integration.

```bash
# Build all modules
make build

# Build specific module
make logger
```

---

## Actions

Actions are WASM modules that perform side effects (HTTP calls, webhooks, etc.) triggered by journey steps. Each action module exports three functions.

### The Three-Export Contract

Every action module must export:

- **`manifest()`** — Returns an `ActionManifest` containing metadata, config/payload JSON schemas, and variable specifications. The host calls this at load time to register the action.
- **`execute()`** — Receives an `ExecuteRequest` with config, payload, and pre-resolved variables. Performs the action (e.g. an HTTP call) and returns an `ExecuteResponse` with status and metadata.
- **`preview()`** — Returns a self-contained HTML document (a Preact app built with Vite) that the frontend renders in a sandboxed iframe for live previewing.

### Variable Specifications

Actions declare the variables they accept via `VariableSpec` entries in the manifest. Each spec has:

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Identifier used in templates (e.g. `user_id`) |
| `title` | `string` | Human-readable label (e.g. "User ID") |
| `description` | `string` | Help text shown in the UI (optional) |
| `type` | `string` | One of: `string`, `number`, `boolean`, `object` |
| `required` | `bool` | Whether a binding is mandatory |
| `default` | `string` | Default Liquid expression (e.g. `{{ user.email }}`) |

Example from the webhook module:

```go
Variables: []actions.VariableSpec{
    {
        Name:     "user_id",
        Title:    "User ID",
        Type:     actions.VariableTypeString,
        Required: true,
        Default:  "{{ user.id }}",
    },
    {
        Name:     "user_email",
        Title:    "User Email",
        Type:     actions.VariableTypeString,
        Required: false,
        Default:  "{{ user.email }}",
    },
},
```

### Host-Side Variable Resolution

The host resolves Liquid templates against `ctx.Data` **before** calling `execute()`. The module receives already-resolved values in the `Variables` map — it never runs Liquid itself.

Available context objects in Liquid expressions:

| Object | Contents |
|--------|----------|
| `user` | User attributes: `id`, `email`, `first_name`, etc. plus custom data |
| `event` | Event name and payload that triggered the journey |
| `journey` | Accumulated journey state from prior steps |

### Preview Architecture

Each action ships a Preact app (in `preview/`) that gets built into a single HTML file and embedded via `//go:embed`. The host serves this HTML in a sandboxed iframe.

**Communication protocol:**

- **Parent → iframe:** The parent React app sends `preview-update` messages via `postMessage` containing `actionType`, `config`, `payload`, and `variables`.
- **Iframe → parent:** The preview sends `resize` messages back with `{ type: 'resize', height }` so the parent can auto-size the iframe.

```
┌─────────────────────────┐     postMessage('preview-update')     ┌──────────────────────┐
│  Parent (React)         │ ──────────────────────────────────▶   │  Iframe (Preact)     │
│                         │                                       │                      │
│                         │  ◀──────────────────────────────────  │                      │
│                         │     postMessage('resize')             │                      │
└─────────────────────────┘                                       └──────────────────────┘
```

### Built-in

| Action | Description |
|--------|-------------|
| `webhook` | Send an HTTP request to an external endpoint |

### Creating a New Action

1. Create `modules/actions/<name>/main.go` implementing `manifest()`, `execute()`, and `preview()`
2. Create `modules/actions/<name>/preview/` with a Preact app (see `webhook/preview/` for reference)
3. Create `modules/actions/<name>/go.mod` with the Extism PDK dependency
4. Run `make actions` to build

See `actions/webhook/` for a complete working example.

### Build Pipeline

Running `make actions` performs these steps for each action module:

```
1. Preview build
   cd modules/actions/<name>/preview && npm ci && npx vite build
   → produces preview/dist/index.html

2. Go source embeds the preview
   //go:embed preview/dist/index.html
   var previewHTML []byte

3. TinyGo compiles the WASM module
   tinygo build -target=wasi -buildmode c-shared -opt=2 -no-debug \
     -o internal/actions/modules/<name>.wasm ./main.go

4. Host embeds all WASM modules at compile time
   //go:embed modules/*    (in internal/actions/registry.go)
```

---

## Package Layout

```
pkg/modules/                      # Shared types (WASM guests import this)
pkg/modules/providers/            # Provider-specific types (payloads, requests)
pkg/modules/actions/              # Action-specific types (manifest, request, variable specs)
internal/providers/               # Provider WASM runtime (host only)
internal/actions/                 # Action WASM runtime (host only)
modules/providers/                # Provider module source code
modules/actions/                  # Action module source code
```

The split keeps `pkg/modules` free of WASM runtime dependencies so TinyGo can compile it.
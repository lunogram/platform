# Modules

> 🚧 **Early Alpha** - Under active development.

WASM plugins that extend Lunogram. Currently used for **providers** (email, SMS, push integrations).

## Why WASM?

Sandboxed, portable, fast. Write in any language that compiles to WASM.

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

## Package Layout

```
pkg/modules/                      # Shared types (WASM guests import this)
pkg/modules/providers/            # Provider-specific types (payloads, requests)
internal/providers/               # WASM runtime (host only)
```

The split keeps `pkg/modules` free of WASM runtime dependencies so TinyGo can compile it.
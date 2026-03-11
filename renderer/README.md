# Renderer

Deno service that compiles and renders [React Email](https://react.email) templates. It connects to NATS and listens on two subjects:

- **`email.compile.>`** — Transpiles JSX/TypeScript source into a self-contained JS bundle (via Sucrase). The compiled output is stored and reused across renders.
- **`email.render.>`** — Executes a previously compiled bundle with the given props and returns the resulting HTML and plain text.

## Running

```sh
# development (watch mode)
deno task dev

# production
deno task start
```

The service expects a `NATS_URL` environment variable (defaults to `nats://localhost:4222`). In Docker Compose it runs as the `renderer` service alongside the main Lunogram app.

## How it works

1. **Compile** — `compiler.ts` uses Sucrase to strip TypeScript and transform JSX, removes import/export statements, and serialises the resulting code plus any detected Tailwind config bindings into a JSON bundle.
2. **Render** — The bundle is executed inside a `new Function()` with `react`, `react/jsx-runtime`, and all `@react-email/components` exports injected as scope. The default-exported component is rendered to HTML and plain text via `@react-email/render`.
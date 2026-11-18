# Working in Lunogram

This file is the entry point for coding agents. Read it before changing the repository, then read the nearest nested `AGENTS.md` for the files you will touch.

## Sources of truth

Use this precedence when guidance conflicts:

1. The user's current request.
2. The nearest `AGENTS.md`, then this file.
3. The OpenAPI specifications: `internal/http/controllers/v1/management/oapi/resources.yml`, `internal/http/controllers/v1/client/oapi/resources.yml`, and `oapi/webhooks.yml`.
4. [`internal/pubsub/README.md`](internal/pubsub/README.md) for the event pipeline and [`console/DESIGN.md`](console/DESIGN.md) for the console design system.
5. Existing code and tests.

If code conflicts with an OpenAPI specification, preserve the specification and call out the implementation gap unless the task is to change the contract.

## Repository map

- `cmd/lunogram/`: the single binary. Every node runs the HTTP server, the NATS consumers, and cluster membership; the elected leader also runs the scheduler.
- `internal/http/`: chi server, authentication, and controllers. `controllers/v1/management/` serves the console and admin API; `controllers/v1/client/` serves the public client API used by SDKs and end users.
- `internal/store/`: PostgreSQL access with sqlx, split across three schemas with their own embedded migrations: `management/`, `subjects/` (the `users` schema), and `journey/`.
- `internal/pubsub/`: NATS JetStream producers and consumers for events, lists, journeys, campaigns, broadcasts, and inboxes.
- `internal/journeys/`: journey step execution. `internal/rules/`: segmentation rule evaluation.
- `internal/cluster/`: consensus, leader election, and the scheduler.
- `internal/rbac/`: actors, roles, and OpenFGA-backed permission checks.
- `internal/wasm/`, `internal/providers/`, `internal/actions/`, `internal/integrations/`: hosts for WASM modules.
- `modules/`: TinyGo WASM providers and actions; `pkg/modules/` is the SDK they compile against. See [`modules/README.md`](modules/README.md).
- `internal/outbound/` and `internal/ssrf/`: outbound HTTP client and SSRF policy.
- `console/`: React and Vite operator console. Its Storybook stories in `console/stories/` are the visual design reference; follow [`console/DESIGN.md`](console/DESIGN.md) and the console UI rules in [`console/AGENTS.md`](console/AGENTS.md) before adding or changing UI.
- `renderer/`: Deno service that compiles and renders React Email templates over NATS.
- `docs/`: Fumadocs user documentation, with content in `docs/content/docs/`.
- `.github/workflows/`: required CI checks.

Start with [`internal/pubsub/README.md`](internal/pubsub/README.md) for how events flow between components.

## Architectural guardrails

- A project is the ownership and authorization boundary. Every project-scoped query constrains `project_id`, and every handler checks permission through `internal/rbac` before reading or writing.
- End-user actors on the client API reach only their own subject. Follow the own-scope guards and their tests (`*_ownscope_test.go`, `*_idor_test.go`) when adding client endpoints.
- Verify ownership of every user-provided foreign key. Do not rely on a later query to make an unsafe lookup harmless.
- User-configured outbound HTTP (webhooks, actions, OIDC discovery, JWKS) goes through `internal/outbound` or the `internal/ssrf` policy so private networks stay unreachable.
- JetStream consumers use explicit acks and messages may be redelivered. Make every handler idempotent, and publish follow-up subjects only after the state they depend on is committed.
- Assume multiple nodes. Work that must happen once per cluster belongs to the leader's scheduler; use Redis locks or database constraints for anything else that must not run concurrently.
- The console currently calls the management API through both the legacy Axios client in `console/src/api.ts` and the typed `oapiClient` in `console/src/oapi/client.ts`. The target is `oapiClient` only; do not add endpoints to `api.ts`.
- Persisted JSONB (journey steps and state, templates, rules, delivery stats) is a compatibility contract. Existing rows must remain readable and executable after schema or behavior changes.
- Migrations are append-only once on `main`. Add a timestamp-prefixed migration that sorts after the latest one in the owning schema's `migrations/`.
- Enterprise features are licensed separately (see [`LICENSE`](LICENSE)). Put Go enterprise code in `*_enterprise.go` files under `//go:build enterprise`, paired with a `//go:build !enterprise` counterpart, and gate console enterprise code on `isEnterprise` from `console/src/config/enterprise.ts`. Both editions must build and pass tests.

## Working rules

- Use the smallest change that satisfies the request. Prefer reusing or extending existing code over adding another path, component, helper, or abstraction. Keep Lunogram's core small and remove duplication instead of hiding it behind wrappers.
- The user's request and pull request description define scope. Review findings do not expand it. Fix defects introduced by the change; leave pre-existing, unaffected debt alone.
- Before editing code for review feedback, summarize the findings, classify what is in scope, and wait for user approval of one fix batch.
- Stop and ask before a review fix adds durable state, a migration, a NATS subject or consumer, a public API contract, a deployment change, or a new top-level package. Offer to narrow the implementation, split a prerequisite, or expand scope; do not choose silently.
- After fixing the approved batch, do not start another fix-and-review cycle without user approval.
- Follow existing patterns before introducing abstractions or dependencies. A new abstraction must replace meaningful duplication or enforce an architectural boundary.
- Write parameterized SQL in `internal/store/`; build dynamic placeholders with `store.QueryArgs` and never interpolate values into SQL.
- A new table with an `updated_at` column attaches the `set_updated_at()` trigger, as existing tables do.
- Do not edit generated files directly. Change their source (OpenAPI specs, `go:generate` inputs) and run `make generate`; CI fails when generated files are stale.
- Do not add secrets, decrypted values, credentials, or production data.
- Add comments only for non-obvious constraints or decision references.
- Use Conventional Commits with a scope: `fix(journey): default exit to the first entrance on placement`.
- Run pnpm commands from `console/` or `docs/`, where their `package.json` lives.
- Do not run deployments or database migrations unless explicitly requested.

## Validation

Choose the smallest relevant validation. CI remains authoritative. Go tests start PostgreSQL, Redis, NATS, and storage emulators with testcontainers, so Docker must be running.

```shell
# Go static checks, both editions
go build ./...
go build -tags enterprise ./...

# Focused Go tests (add -tags enterprise for enterprise code)
go test ./internal/<package>/... -run <Test>

# Linters (golangci-lint and console ESLint)
make lint

# Generated files after spec or go:generate source changes
make generate

# Console
cd console
pnpm build
pnpm vitest run <file>
pnpm storybook        # design notebook on http://localhost:6006
pnpm storybook:build
```

Tests and generators can be expensive or require services. Run them when the task requests them or when required by a changed source file, and state exactly what was and was not run.

## Pull request descriptions

Keep PR descriptions proportional to the change. A simple PR should use a few lines:

- **Why**: the problem or reason for the change.
- **How**: the implementation approach, without a file-by-file inventory.
- **Scope**: affected components and, when useful, explicit non-goals.
- **Alternatives**: optional; include only when a meaningful approach was tried or rejected.
- **Diagram**: optional; use a small Mermaid diagram only when it clarifies a flow or boundary.
- **Screenshots**: required when a PR adds or changes console UI components or pages. Include one screenshot per affected story or state at desktop width (1280px), plus a narrow-width (390px) shot for responsive layouts. Take them from Storybook, cropped to the component. Never commit screenshots to the repository; upload them with `gh pr create --attach` or `gh pr edit --attach` (gh 2.102+). Reference each file in the body as `![state](./state-1280.png)` and pass `--attach './state-1280.png#state'`; `gh` rewrites the reference to the uploaded asset. Update the screenshots when review changes alter the UI.

Do not generate long summaries, exhaustive change lists, or boilerplate sections.

## Documentation requirements

During every change, check whether user, API, operational, setup, or architecture documentation became incomplete or false. Update existing documentation before adding another document. Update `docs/content/docs/` when user-, API-, or operator-visible behavior changes; [`internal/pubsub/README.md`](internal/pubsub/README.md) when subjects or event flows change; [`modules/README.md`](modules/README.md) when the module contract changes; and [`CONTRIBUTING.md`](CONTRIBUTING.md) when setup changes.

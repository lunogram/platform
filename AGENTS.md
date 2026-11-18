# Working in Lunogram

This file is the entry point for coding agents. Read it before changing the repository.

## Sources of truth

Use this precedence when guidance conflicts:

1. The user's current request.
2. This file.
3. The OpenAPI specifications, for every HTTP contract (see [Generated code](#generated-code)).
4. [`console/DESIGN.md`](console/DESIGN.md), for console visuals.
5. Existing code and tests.

## Repository map

- `cmd/lunogram/`: the single binary. One process runs the HTTP server, the NATS consumers, and cluster membership; the elected leader additionally runs the scheduler and leader-only setup.
- `internal/http/`: chi server, authentication (`auth/`), and controllers. `controllers/v1/management/` serves the console and admin API; `controllers/v1/client/` serves the public client API used by SDKs and end users.
- `internal/store/`: PostgreSQL access with sqlx and hand-written SQL, split across three schemas with their own embedded migrations: `management/`, `subjects/` (the `users` schema), and `journey/`.
- `internal/pubsub/`: NATS JetStream event pipeline. [`internal/pubsub/README.md`](internal/pubsub/README.md) maps every subject and handler; keep it current when subjects or flows change.
- `internal/journeys/`: journey step execution. `internal/rules/`: segmentation rule evaluation. `internal/cluster/`: consensus, leader election, and scheduler.
- `internal/rbac/`: actors, roles, and OpenFGA-backed permission checks.
- `internal/wasm/`, `internal/providers/`, `internal/actions/`, `internal/integrations/`: hosts for WASM modules. `modules/` holds the TinyGo module sources and `pkg/modules/` the SDK they compile against; see [`modules/README.md`](modules/README.md).
- `internal/outbound/` and `internal/ssrf/`: the outbound HTTP client and SSRF policy.
- `console/`: React 18 + Vite operator console.
- `docs/`: Fumadocs (Next.js) user documentation, content in `docs/content/docs/`.
- `renderer/`: Deno email template compiler.

## Guardrails

- Scope every query and handler to the authenticated actor's project. Project-scoped SQL constrains `project_id`; handlers check permission through `internal/rbac` before reading or writing. End-user actors on the client API may only reach their own subject; follow the own-scope guards and their tests (`*_ownscope_test.go`, `*_idor_test.go`) when adding client endpoints.
- Verify ownership of every user-supplied foreign key before use. A later query that happens to filter correctly does not replace the check.
- Send user-configured outbound HTTP (webhooks, actions, OIDC discovery, JWKS) through `internal/outbound` or the `internal/ssrf` policy so private networks stay unreachable.
- JetStream consumers use explicit acks and messages are redelivered. Make every handler idempotent, and publish follow-up subjects only after the state they depend on is committed.
- Several nodes run the same binary. Work that must happen once per cluster belongs in the leader's scheduler; use Redis locks or database constraints for anything else that must not run concurrently.
- Migrations are append-only once on `main`. Add a timestamp-prefixed `<timestamp>_name.up.sql` that sorts after the latest one (with `.down.sql` where reversible) in the owning schema's `migrations/`; a migration added in the same branch may still be edited. Existing rows, persisted JSONB shapes, and journey state must keep working after the change.
- Enterprise features are licensed separately (see [`LICENSE`](LICENSE)). Put Go enterprise code in `*_enterprise.go` files under `//go:build enterprise`, paired with a `//go:build !enterprise` counterpart, and gate console enterprise code on `isEnterprise` from `console/src/config/enterprise.ts`. Keep enterprise behavior out of untagged files; both editions must build and pass tests.
- Never commit secrets, credentials, `.env` files, or production data.

## Generated code

Edit the source, then run `make generate`; never hand-edit generated output. CI's `generate` job fails when regenerated files differ from the commit.

- `internal/http/controllers/v1/management/oapi/resources.yml` → `resources_gen.go` (oapi-codegen, with `overlay.yml`) and `console/src/oapi/management.generated.ts` (`pnpm generate:oapi`).
- `internal/http/controllers/v1/client/oapi/resources.yml` → `resources_gen.go`.
- `oapi/webhooks.yml` → `oapi/webhooks_gen.go`.
- `*_string.go` (stringer), `*_mock_test.go` (minimock), and `controllers/v1/client/static/styles.css` (tailwindcss) come from `go:generate` directives.

API changes start in the spec. In the console, call the management API through the typed `oapiClient` in `console/src/oapi/client.ts`; `console/src/api.ts` is the legacy Axios client and shrinks over time.

## Console

- Follow [`console/DESIGN.md`](console/DESIGN.md) for tokens, typography, spacing, and component patterns. Reuse or extend primitives in `console/src/components/ui/` (shadcn-based) before adding new ones.
- Forms use react-hook-form with zod schemas from `console/src/validation/`.
- User-facing strings go through `react-i18next`; add new keys to `console/public/locales/en.json`.
- Use type-only imports (`import type`), enforced by ESLint; formatting is Prettier via ESLint.

## Working rules

- Make the smallest change that satisfies the request. Extend existing code before adding another path, helper, or abstraction; a new abstraction must remove real duplication or enforce a boundary.
- The request and PR description define scope. Fix defects the change introduces; leave unrelated debt alone.
- Comment only non-obvious constraints and the reason behind them.
- Do not run deployments or migrations against shared environments unless asked.
- Commit messages use Conventional Commits with a scope and a lowercase imperative subject: `fix(journey): default exit to the first entrance on placement`.

## Validation

Run the narrowest check that covers the change; CI is authoritative. Go tests start PostgreSQL, Redis, NATS, and storage emulators through testcontainers, so Docker must be running.

```shell
go build ./... && go build -tags enterprise ./...
go test ./internal/<package>/... -run <Test>          # add -tags enterprise for enterprise code
make lint                                             # golangci-lint + console ESLint
make generate                                         # after spec or go:generate source changes

cd console && pnpm lint && pnpm build                 # tsc + vite
cd console && pnpm vitest run <file>                  # vitest
cd docs && pnpm build                                 # after docs changes
```

State exactly which checks ran and which did not.

## Pull request descriptions

Keep PR descriptions proportional to the change:

- **Why**: the problem or reason.
- **How**: the approach, not a file-by-file inventory.
- **Scope**: affected areas and, when useful, non-goals.
- **Screenshots**: required when console UI changes, taken from the running console at desktop width.

Skip boilerplate sections and exhaustive change lists.

## Documentation

When behavior visible to users, API consumers, or operators changes, update the matching page in `docs/content/docs/` in the same PR. Keep `internal/pubsub/README.md`, `modules/README.md`, and `CONTRIBUTING.md` accurate when their subject changes. Update existing documents before adding new ones.

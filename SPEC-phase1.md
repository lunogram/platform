# Phase 1 — Multi-user email/password login (authoritative spec)

Branch `feat/admin-password-auth`, stacked on `feat/admin-auth-foundation` (Phase 0).
Read `SPEC-phase0.md` first if present; this builds directly on its seams.

## Goal

An organization can have many admins who sign in with their own email and
password. Today `AUTH_DRIVER=basic` authenticates ONE credential pair from
environment variables and provisions a single owner — that is not multi-user and
never was.

## What Phase 0 already gave you — use it, do not rebuild it

- `admin_identities` with `provider`, `issuer`, `subject`, `email`,
  `email_verified` and an unused `secret_hash TEXT` column, plus a CHECK that
  `(provider = 'password') = (secret_hash IS NOT NULL)`. No migration is needed
  to start storing password hashes.
- `Verifier` / `VerifiedIdentity` / `Exchanger`. A verifier proves a credential
  and returns an identity; the Exchanger does resolve-or-link, provisioning,
  session creation and the cookie. **Do not** put any of that in your verifier.
- `admin_sessions` with revocation, and `access.ProvisionMembership`.
- `OrgResolver` decides which organization a brand-new admin joins.

## Part A — Platform mailer (prerequisite, build first)

There is no way for the platform to send its own email. Project email providers
are WASM modules scoped to a project and configured in the database, which
cannot work here: the first admin signs up before any project exists. Auth mail
must not depend on tenant data.

Build `internal/mailer`:

- `Mailer` interface: `Send(ctx, Message) error` where `Message` carries To,
  Subject, HTML, Text.
- SMTP transport configured under `MAIL_` env: `HOST`, `PORT`, `USERNAME`,
  `PASSWORD`, `FROM_ADDRESS`, `FROM_NAME`, `TLS` (starttls|implicit|none).
  Use `net/smtp` or a small dependency — justify whichever you pick.
- A `LogMailer` used when no SMTP host is configured: logs the message and the
  action URL at INFO. This keeps local development and the docker-compose
  quickstart working with zero configuration — a self-hoster must be able to
  create an account without standing up an SMTP server. Log the link, never the
  token alone, and make the log line obviously development-only.
- Templates: plain Go `html/template` + `text/template`, both parts always sent.
  Keep them minimal and dependency-free; this is transactional auth mail, not
  marketing. Template the product name and the base URL from `PublicBaseURL()`.
- Every send is asynchronous and MUST NOT block or fail the HTTP request that
  triggered it. A mail failure is logged, never surfaced to the caller — see
  enumeration below.

## Part B — Password credentials

**Hashing**: argon2id via `golang.org/x/crypto/argon2` (already an indirect
dependency; promote it). Store as a self-describing PHC string
(`$argon2id$v=19$m=...,t=...,p=...$salt$hash`) so parameters can be raised later
and existing hashes still verify. Pick parameters targeting ~100ms on a modern
server and state your reasoning in a comment. On successful verification with
outdated parameters, transparently re-hash.

**Storage**: `admin_identities` rows with `provider='password'`,
`issuer='urn:lunogram:password'`, `subject` = the admin's UUID string, and
`secret_hash` set. One password identity per admin.

**`PasswordVerifier`** implements `Verifier`: reads email+password, looks up the
password identity by email, verifies the hash, returns a `VerifiedIdentity` with
`EmailVerified` reflecting the stored flag. It does nothing else.

Constant-time comparison throughout. When an email has no password identity,
still perform a dummy hash comparison so response timing does not reveal whether
an account exists.

## Part C — Flows

All endpoints under `/api/auth/`, no `security` block, cookie/credential-free
except where stated.

1. **Register** `POST /api/auth/register` — email + password (+ optional name).
   Creates the admin, the password identity, and their organization via
   `OrgResolver`. Sends a verification email. Whether registration is open is
   controlled by `AUTH_PASSWORD_REGISTRATION` (`open` | `invite_only` |
   `disabled`, default `invite_only`). A public SaaS and a private deployment
   need different answers and the default must be the safe one.
2. **Verify email** `POST /api/auth/verify` — consumes a token, sets
   `email_verified`. Unverified accounts may sign in but `EmailVerified` stays
   false, so the Exchanger will not email-link them to another identity.
3. **Login** `POST /api/auth/login/password/callback` — via the existing
   callback shape, `PasswordVerifier` + `Exchanger`.
4. **Request reset** `POST /api/auth/password/reset` — always returns 204
   regardless of whether the account exists.
5. **Complete reset** `POST /api/auth/password/reset/confirm` — token + new
   password. On success **revoke every existing session for that admin** — a
   password reset is the user's remedy for a compromised account and must end
   the attacker's sessions.
6. **Change password** `POST /api/admin/profile/password` — authenticated,
   requires the current password, revokes all OTHER sessions but keeps the
   caller's.
7. **Invite acceptance with password**: an invited email with no account can set
   a password and land directly in the inviting organization. Reuse the existing
   invite tables; do not build a second invitation concept.

**Tokens** (verification and reset): new table `admin_action_tokens` — random
32 bytes, stored as SHA-256 **hash only**, single-use (consumed atomically),
short TTL (verification 24h, reset 1h), bound to admin id and purpose. Consuming
must be a single statement so a token cannot be redeemed twice concurrently.
Invalidate outstanding reset tokens when a password changes.

## Part D — Abuse resistance

- Per-account throttling on login and reset: after N failed attempts within a
  window, reject with 429 for a cooldown. Use the existing Redis-backed
  `internal/ratelimit` rather than inventing state.
- Per-IP throttling on register/reset so the mailer cannot be used as a spam
  cannon against third parties.
- **No user enumeration anywhere.** Register, login, and reset must not reveal
  whether an address has an account, via status code, body, or timing. Where the
  UX genuinely requires telling a user their address is taken, do it in the
  emailed message, not the HTTP response.
- Minimum password length 12; reject the top few thousand known-breached
  passwords if you can do it without a large dependency, otherwise length +
  a "not similar to the email address" check. Do not impose composition rules
  (upper/lower/symbol) — they measurably reduce entropy in practice.

## Part E — Console

Login view already selects among drivers from `GET /api/auth/methods`; add the
password driver, plus register / forgot-password / reset / verify views and a
change-password control in profile settings. Match the existing shadcn
component usage and i18n (`console/public/locales/en.json`). `Register.tsx` is
currently hard-wired to Clerk with no driver check — fix that while you are
there.

## Non-negotiables

- The Exchanger stays the only place a session is created.
- Email-based identity linking still requires `EmailVerified`.
- Reset tokens are never logged, never returned in a response body, and never
  stored in plaintext.
- `AUTH_DRIVER=basic` keeps working unchanged — it is the documented quickstart.
- Multiple drivers may be enabled simultaneously; `GET /api/auth/methods` must
  return all configured drivers, not one. Phase 0 may still return a single
  driver; fix it here.

## Definition of done

`go build ./...` and `-tags enterprise` clean; `go vet`, `golangci-lint`,
`gofmt` clean; `make generate` no diff; console `tsc --noEmit` and lint clean.
Tests for: argon2 round-trip and re-hash-on-outdated-params, token single-use
under concurrency, reset revoking sessions, enumeration resistance (same status
and shape for known and unknown addresses), throttling, and every flow
end-to-end through the HTTP layer.

Run tests with `-p 1` — a shared Postgres container is reused across packages
and parallel package runs collide.

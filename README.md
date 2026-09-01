<br />
<div align="center">
  <a href="https://lunogram.com" target="_blank">
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset="https://lunogram.com/logos/logo-white-512.png">
        <img src="https://lunogram.com/logos/logo-dark-512.png" width="360" alt="Logo"/>
    </picture>
  </a>
</div>

<h1 align="center">SaaS Multi-Channel Outreach</h1>

<p align="center">Engage your customers through effortless communication.</p>

<p align="center">
  <a href="https://docs.lunogram.com">Documentation</a> •
  <a href="https://github.com/lunogram/platform/discussions">Discussions</a> •
  <a href="#contributing">Contributing</a>
</p>

<p align="center">⭐️ Enjoying Lunogram? Please <a href="https://github.com/lunogram/platform">leave a star</a>!</p>

<p align="center">
  <em>Lunogram is a fork of <a href="https://github.com/parcelvoy/platform">Parcelvoy</a></em>
</p>

<br />

## Features
- 💬 **Cross Channel Messaging** Send data-driven emails, push notifications and text messages.
- 🛣 **Journeys** Build complex journeys with our drag-and-drop builder to schedule, trigger and segment users.
- 👥 **Segmentation** Create dynamic lists to target users matching any event or user based criteria in real time.
- 📣 **Campaigns** Build campaigns that target specific lists of users and go out at pre-defined times.
- 🔗 **Integrations** Connect Lunogram to your applications using our easy to use SDKs and APIs.
- 🔒 **Secure** OpenID Connect single sign-on, configured like any other login driver, no add-ons. SAML is not supported.
- 📦 **Open Source** Easy to setup and get running in your own cloud.

## 🚀 Deployment

You can run Lunogram locally or in the cloud easily using Docker.

### Docker Compose

To get up and running quickly, clone the repository and start the services:
```
git clone https://github.com/lunogram/platform.git
cd platform
docker compose up -d
```

This single command builds and runs Lunogram along with its backing services —
PostgreSQL, Redis, NATS and Mailpit. Those are defined in
[`docker-compose.deps.yml`](docker-compose.deps.yml), which the root
[`docker-compose.yml`](docker-compose.yml) pulls in automatically.

If you'd rather run Lunogram from source and only need the backing services in Docker,
see the [Contributing Guide](CONTRIBUTING.md) for the local development setup.

Login to the web app at http://localhost:8080 using the default credentials:

```
AUTH_BASIC_EMAIL=admin@localhost
AUTH_BASIC_PASSWORD=change-this-development-password
```

**Note:** These credentials are published in this repository. Change them
before ever using Lunogram in production.

The compose file also ships a development `AUTH_CONSOLE_SIGNING_KEY`, the key
that signs console session tokens. It is published in this repository, so
anyone holding it can mint a session for any admin — set your own before
deploying:

```
AUTH_CONSOLE_SIGNING_KEY="$(openssl ecparam -name prime256v1 -genkey -noout)"
```

### Signing in with your own email and password

`AUTH_DRIVER=basic` gives every admin their own account. `AUTH_BASIC_EMAIL` and
`AUTH_BASIC_PASSWORD` are not a credential the login compares against — they
**seed the first account**. On the first boot the pair is written into a real
admin with the password hashed, so it holds its own permissions, appears in the
admin list and can change its own password, like any account created afterwards.

The plaintext is needed exactly once. Once the account exists you can remove
`AUTH_BASIC_PASSWORD` from the environment; a later boot never overwrites a
stored password, so a restart cannot undo one you changed in the console.

```
AUTH_DRIVER=basic
AUTH_BASIC_EMAIL=you@example.com
AUTH_BASIC_PASSWORD=...
AUTH_BASIC_REGISTRATION=invite_only
```

`AUTH_DRIVER` accepts a comma-separated list, so `basic,oidc` offers both at
once — useful while an organization moves onto single sign-on, which is
configured the same way and described below.

`AUTH_BASIC_REGISTRATION` decides who else may create an account: `open` for a
public sign-up, `disabled` to provision admins some other way, and the default
`invite_only`, which admits addresses holding a pending invite plus the very
first account (nobody could have invited that one).

> **Claim the first account before exposing a new deployment.** Until one admin
> exists, `invite_only` has to admit somebody or the instance could never be set
> up — so on a fresh install that is already reachable from the internet, the
> first person to register becomes its owner, whoever they are. Register before
> you open the port, or start with `AUTH_BASIC_REGISTRATION=disabled` and
> switch it on once you hold the account.

Registering does not ask you to confirm the address: the account works
immediately. Password resets and project invitations are sent by email, so the
deployment has to say where its mail goes. `docker compose up` runs
[Mailpit](https://mailpit.axllent.org) alongside the platform and points it
there, readable at <http://localhost:8025>. Nothing leaves the machine and no
SMTP account is needed.

There is no channel that only writes messages to the log. A deployment offering
password logins with nowhere to send mail cannot reset a password or deliver an
invitation, so it is refused at boot rather than at the first request.

Before production, point it at a real relay:

```
MAIL_CHANNEL=smtp
MAIL_FROM_ADDRESS=no-reply@example.com
MAIL_SMTP_HOST=smtp.example.com
MAIL_SMTP_PORT=587
MAIL_SMTP_USERNAME=lunogram
MAIL_SMTP_PASSWORD=...
MAIL_SMTP_TLS=starttls     # or implicit, or none
```

Or hand delivery to your own system instead, with `MAIL_CHANNEL=webhook`: the
platform posts the rendered message to a URL you configure, which is also how
you reach an HTTP-only provider such as Resend or Postmark without the platform
growing a client for each one. Both channels send the same message, and the
copy and layout are yours to override — see
[Configuration](https://docs.lunogram.com/settings/configuration) and
[Sending mail](https://docs.lunogram.com/settings/mail).

### Working with other people

A project's people live under **Settings → Members**: the roster of who has
access and the role each holds, and the invitations that have not been accepted
yet. Inviting somebody mails them a link to the console, where they accept by
signing in with the address it was sent to — an invitation is claimed by proving
the address, not by holding the link. Invitations expire after 48 hours. See
[Members](https://docs.lunogram.com/settings/members).

### Signing in with your company's identity provider

Point the deployment at your own OpenID Connect providers — Okta, Entra ID,
Google Workspace, Keycloak, Auth0 — and your staff sign in there. It is a driver
like the others: add `oidc` to `AUTH_DRIVER` and configure it. One provider is
configured from the environment; several are declared in the configuration file.

SAML is deliberately not supported, and is not planned. OpenID Connect is what
the platform speaks.

#### 1. Register the application with your identity provider

Create an OpenID Connect **web application** (a confidential client — one that
holds a secret) and note its client ID and client secret. Okta calls this an
"OIDC — Web Application"; Entra ID calls it an "App registration" with a Web
redirect URI; Keycloak calls it a client with "Client authentication" on.

The redirect URI to register is your `PUBLIC_URL`, the callback path, and the
provider's id — `default` for the single-provider form below:

```
https://<your-lunogram-host>/api/auth/oidc/default/callback
```

It is derived from `PUBLIC_URL` and never from anything in a request, and it
must be registered **exactly** — most providers reject a mismatch outright.
Each provider gets its own, so a second one registers
`.../api/auth/oidc/<its-id>/callback`.

#### 2. Configure the driver

```
AUTH_DRIVER=basic,oidc
AUTH_OIDC_ISSUER=https://example.okta.com
AUTH_OIDC_CLIENT_ID=...
AUTH_OIDC_CLIENT_SECRET=...
```

`AUTH_OIDC_ISSUER` is the URL your provider stamps as `iss`, taken verbatim —
issuer identifiers are compared exactly, trailing slash included. The metadata is
read from `<issuer>/.well-known/openid-configuration` unless
`AUTH_OIDC_DISCOVERY_URL` says otherwise, and wherever it points it must be
served by the issuer's own origin: whoever chooses that URL otherwise also
chooses the token endpoint and the JWKS.

The rest have defaults worth knowing about:

| Variable | Default | What it is |
| --- | --- | --- |
| `AUTH_OIDC_SCOPES` | `openid,email,profile` | Requested at the authorization endpoint. `openid` is added if you leave it out |
| `AUTH_OIDC_EMAIL_CLAIM` | `email` | Which claim carries the address |
| `AUTH_OIDC_EMAIL_VERIFIED_CLAIM` | `email_verified`, but only when the address comes from `email` | Which claim attests the address |
| `AUTH_OIDC_GIVEN_NAME_CLAIM` | `given_name` | Which claim carries the first name |
| `AUTH_OIDC_FAMILY_NAME_CLAIM` | `family_name` | Which claim carries the last name |

Naming the driver without the issuer, client ID and client secret refuses to
start, rather than failing the first person who presses the button.

#### More than one provider

Several providers are declared in the configuration file, where `${VAR}`
references keep the secrets in the environment:

```yaml
auth:
  drivers: [basic, oidc]
  oidc:
    providers:
      - id: staff
        name: Staff directory
        issuer: https://example.okta.com
        client_id: 0oa...
        client_secret: ${OKTA_CLIENT_SECRET}
      - id: contractors
        name: Contractors
        issuer: https://login.microsoftonline.com/<tenant>/v2.0
        client_id: ...
        client_secret: ${ENTRA_CLIENT_SECRET}
        allowed_domains: [partner.example]
```

Every field above has the same meaning as its `AUTH_OIDC_*` twin. The `id` is
what appears in that provider's login and callback URLs, so it has to survive a
URL path segment and cannot change without re-registering the redirect URI.
The `name` is what the login page calls it, and falls back to the id.

Set `AUTH_OIDC_*` **or** `auth.oidc.providers`, not both — mixing them is
refused at startup rather than merged, because there is no order in which one of
them obviously wins.

`allowed_domains` bounds the email domains a provider may speak for. It is
optional and matters once there are several: a verified address links a login to
an existing admin whichever provider asserted it, so without it the least
trustworthy provider decides who can reach every account — a consumer tenant
added "so contractors can sign in" would be able to assert a staff address.
Nothing has to be proved here, unlike a domain a customer claims: you own every
provider in this list.

#### 3. Sign in

Staff choose "Continue with single sign-on" on the login page and are sent
straight to your provider, or pick between them by name when you configured
several. Leaving `oidc` as the only `AUTH_DRIVER` makes it the only way in;
listing it alongside `basic` offers both while you migrate.

An address is only linked to an existing account when the provider itself
reports it verified, so a provider that lets people type any address into their
profile cannot inherit somebody else's account by claiming it.

That attestation is tied to the claim the address came from. `email_verified`
attests `email` and nothing else, so pointing `AUTH_OIDC_EMAIL_CLAIM` at an
editable claim such as `preferred_username` or `upn` leaves addresses unverified
— and therefore never linked to an existing account — until
`AUTH_OIDC_EMAIL_VERIFIED_CLAIM` names the claim that attests them.

For full documentation on the platform and more information on deployment, check out our docs.

**[Explore the Docs »](https://docs.lunogram.com)**

### Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) to get started.

Join our community on [GitHub Discussions](https://github.com/lunogram/platform/discussions) to connect with other users and contributors, report bugs, suggest features, or just say hi!

## Acknowledgments

Lunogram is a fork of [Parcelvoy](https://github.com/parcelvoy/platform), an open-source customer engagement platform that was publicly archived by its maintainers. We're grateful to the Parcelvoy team for their foundational work and are committed to continuing the project's development as an open-source solution.

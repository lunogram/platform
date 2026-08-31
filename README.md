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
- 🔒 **Secure** SSO (SAML/OpenID) is provided out of the box, no extra bolts or add-ons.
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

`AUTH_DRIVER` accepts a comma-separated list, so `basic,clerk` offers both at
once — useful while an organization moves onto SSO.

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

For full documentation on the platform and more information on deployment, check out our docs.

**[Explore the Docs »](https://docs.lunogram.com)**

### Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) to get started.

Join our community on [GitHub Discussions](https://github.com/lunogram/platform/discussions) to connect with other users and contributors, report bugs, suggest features, or just say hi!

## Acknowledgments

Lunogram is a fork of [Parcelvoy](https://github.com/parcelvoy/platform), an open-source customer engagement platform that was publicly archived by its maintainers. We're grateful to the Parcelvoy team for their foundational work and are committed to continuing the project's development as an open-source solution.

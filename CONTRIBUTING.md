# Contributing to Lunogram

Thank you for your interest in contributing to Lunogram! We're excited to have you join our community.

## Getting Started

1. **Fork the repository** and clone it locally
2. **Set up your development environment** following the instructions below
3. **Create a branch** for your changes

## Prerequisites

- [Nix](https://nixos.org/download/) and [devenv](https://devenv.sh/getting-started/).
  They provide everything else: Go, Node.js, pnpm, TinyGo, PostgreSQL, Redis,
  NATS and Mailpit.
- Optionally [direnv](https://direnv.net), to load the toolchain whenever you `cd`
  into the repository.

## Development Setup

```bash
devenv up
```

This starts PostgreSQL, Redis, NATS, Mailpit, the Go backend
(http://localhost:8080) and the console (http://localhost:5173). There is no
setup step to repeat after pulling:

- the databases come from [`docker/postgres/init.sql`](docker/postgres/init.sql),
  which runs on every start and creates only what is missing
- the console's dependencies are installed again whenever
  `console/pnpm-lock.yaml` changes
- the server reads [`etc/lunogram.dev.yaml`](etc/lunogram.dev.yaml) on every
  start, so a change to it applies on the next restart

`devenv shell` (or `direnv allow` once) gives you the same toolchain for
`make generate`, `make test` and friends. Service data lives in `.devenv/state`;
delete it for a clean database.

When a default port is already taken, devenv moves that service to the next
free one and points the server at it. Mail lands in Mailpit at
http://localhost:8025, or the next free port.

### Configuration

[`etc/lunogram.dev.yaml`](etc/lunogram.dev.yaml) holds the development
settings: a password login seeded with `admin@localhost` /
`change-this-development-password`, a console signing key and Mailpit for mail.
Where the services listen comes from [`devenv.nix`](devenv.nix). Every value is
published, so it is fit for a laptop and nothing else.

Environment variables win over the file. Put personal overrides in a
`devenv.local.nix`, which git ignores:

```nix
{ env.AUTH_DRIVER = "basic,clerk"; }
```

`AUTH_DRIVER` accepts a comma-separated list, so `basic,clerk` gives you local
accounts and SSO at once. `AUTH_BASIC_EMAIL` / `AUTH_BASIC_PASSWORD` seed the
first account rather than being compared against on every login. Password reset
and project invitation mail goes to Mailpit. The mail settings are not optional:
a deployment offering password logins with nowhere to send mail is refused at
boot.

## How to Contribute

### Reporting Bugs

If you find a bug, please open a [GitHub Issue](https://github.com/lunogram/platform/issues/new) or let us know on [GitHub Discussions](https://github.com/lunogram/platform/discussions) with:

- A clear, descriptive title
- Steps to reproduce the issue
- Expected vs actual behavior
- Your environment (OS, browser, etc.)

### Suggesting Features

We'd love to hear your ideas! Open a [GitHub Issue](https://github.com/lunogram/platform/issues/new) or share on [GitHub Discussions](https://github.com/lunogram/platform/discussions) and describe:

- The problem you're trying to solve
- Your proposed solution
- Any alternatives you've considered

### Submitting Code

1. **Open an issue first** to discuss your proposed changes
2. **Fork and branch** from `main`
3. **Write tests** for your changes
4. **Follow code style** guidelines (run `make lint`)
5. **Submit a pull request** with a clear description

## Pull Request Guidelines

- Keep PRs focused on a single change
- Include tests for new features or bug fixes
- Update documentation if needed
- Reference related issues in your PR description

## Questions?

If you have any questions or need help getting started, don't hesitate to reach out on [GitHub Discussions](https://github.com/lunogram/platform/discussions). We're happy to help!

## Community

- Join our [GitHub Discussions](https://github.com/lunogram/platform/discussions) to chat with other contributors
- Be respectful and inclusive
- Help others when you can

## License

By contributing to Lunogram, you agree that your contributions will be licensed under the same license as the project.

---

Thank you for contributing! 🎉

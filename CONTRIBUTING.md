# Contributing to Lunogram

Thank you for your interest in contributing to Lunogram! We're excited to have you join our community.

## Getting Started

1. **Fork the repository** and clone it locally
2. **Set up your development environment** following the instructions below
3. **Create a branch** for your changes

## Prerequisites

- [Go](https://go.dev/dl/) (1.25+)
- [Node.js](https://nodejs.org/) (24+)
- [pnpm](https://pnpm.io/installation)
- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- [TinyGo](https://tinygo.org/getting-started/install/) (0.40+, for building WASM modules)

## Development Setup

### Option 1: Docker (Quickest)

Run everything in Docker, including the Go backend:

```bash
# Clone your fork and cd into it
cd platform

# Start all services (with rebuild)
docker compose up -d --build
```

The app will be available at http://localhost:8080.

### Option 2: Local Development (Recommended for Development)

Run the Go backend and console locally for faster iteration:

#### 1. Start Dependencies

First, start the required services (PostgreSQL, Redis, NATS):

```bash
docker compose -f docker-compose.deps.yml up -d
```

#### 2. Run the Console (Frontend)

In a separate terminal:

```bash
cd console
pnpm install
pnpm dev
```

The console will be available at http://localhost:5173.

#### 3. Run the Go Backend

In another terminal:

```bash
# Install dependencies and build
make generate

# Run the server
go run ./cmd/lunogram
```

The API will be available at http://localhost:8080.

### Environment Variables

```bash
MANAGEMENT_POSTGRES_URI=postgres://postgres:postgrespw@localhost:5432/management?sslmode=disable
USERS_POSTGRES_URI=postgres://postgres:postgrespw@localhost:5432/users?sslmode=disable
JOURNEY_POSTGRES_URI=postgres://postgres:postgrespw@localhost:5432/journey?sslmode=disable
REDIS_ADDRESS=redis://localhost:6379
NATS_URL=nats://localhost:4222
AUTH_DRIVER=basic
AUTH_BASIC_EMAIL=admin@localhost
AUTH_BASIC_PASSWORD=change-this-development-password

# Mail goes to the Mailpit the dependency stack runs; read it at
# http://localhost:8025. Mailpit's SMTP listener speaks plaintext, so the
# default of starttls has to be turned off for it.
MAIL_CHANNEL=smtp
MAIL_FROM_ADDRESS=no-reply@localhost
MAIL_SMTP_HOST=localhost
MAIL_SMTP_PORT=1025
MAIL_SMTP_TLS=none

# Signs the console session token. Required whenever AUTH_DRIVER is set; the
# server refuses to start without it rather than generating a throwaway key,
# which would log everyone out on each restart. Generate one with:
#   openssl ecparam -name prime256v1 -genkey -noout
AUTH_CONSOLE_SIGNING_KEY="$(openssl ecparam -name prime256v1 -genkey -noout)"
```

`AUTH_DRIVER` accepts a comma-separated list, so `basic,clerk` gives you local
accounts and SSO at once. `AUTH_BASIC_EMAIL` / `AUTH_BASIC_PASSWORD` seed the
first account rather than being compared against on every login. Account confirmation and
password reset mail goes to the Mailpit container the dependency stack starts;
read it at <http://localhost:8025>. The mail settings above are not optional: a
deployment offering password logins with nowhere to send mail is refused at
boot. Reach Mailpit at `localhost` rather than at `mailpit`, which only resolves
inside the compose network.

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

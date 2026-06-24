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
PostgreSQL, Redis and NATS. Those are defined in
[`docker-compose.deps.yml`](docker-compose.deps.yml), which the root
[`docker-compose.yml`](docker-compose.yml) pulls in automatically.

If you'd rather run Lunogram from source and only need the backing services in Docker,
see the [Contributing Guide](CONTRIBUTING.md) for the local development setup.

Login to the web app at http://localhost:8080 using the default credentials:

```
AUTH_BASIC_EMAIL=admin@localhost
AUTH_BASIC_PASSWORD=admin
```

**Note:** We would recommend changing these default credentials as well as your `APP_SECRET` before ever using Lunogram in production.

For full documentation on the platform and more information on deployment, check out our docs.

**[Explore the Docs »](https://docs.lunogram.com)**

### Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) to get started.

Join our community on [GitHub Discussions](https://github.com/lunogram/platform/discussions) to connect with other users and contributors, report bugs, suggest features, or just say hi!

## Acknowledgments

Lunogram is a fork of [Parcelvoy](https://github.com/parcelvoy/platform), an open-source customer engagement platform that was publicly archived by its maintainers. We're grateful to the Parcelvoy team for their foundational work and are committed to continuing the project's development as an open-source solution.

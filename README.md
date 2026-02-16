<br />
<div align="center">
  <a href="https://lunogram.com" target="_blank">
    <picture>
        <source media="(prefers-color-scheme: dark)" srcset=".github/assets/logo-dark.png#gh-dark-mode-only">
        <img src=".github/assets/logo-light.png#gh-light-mode-only" width="360" alt="Logo"/>
    </picture>
  </a>
</div>

<h1 align="center">Open Source Multi-Channel Marketing</h1>

<p align="center">Engage your customers through effortless communication.</p>

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

Login to the web app at http://localhost:8080 using the default credentials:

```
AUTH_BASIC_EMAIL=admin@localhost
AUTH_BASIC_PASSWORD=admin
```

**Note:** We would recommend changing these default credentials as well as your `APP_SECRET` before ever using Lunogram in production.

For full documentation on the platform and more information on deployment, check out our docs.

**[Explore the Docs »](https://docs.lunogram.com)**

### Contributing
You can report bugs, suggest features, or just say hi on [Github discussions](https://github.com/lunogram/platform/discussions/new/choose) 

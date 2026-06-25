# Mailgun Provider

Email provider using [Mailgun](https://mailgun.com) for transactional email delivery.

## Configuration

| Field               | Type   | Required | Description                                          |
|---------------------|--------|----------|------------------------------------------------------|
| `apiKey`            | string | Yes      | Your Mailgun private API key                          |
| `apiRegion`         | string | No       | API region, `US` (default) or `EU`                    |
| `domain`            | string | Yes      | Verified Mailgun sending domain (e.g. `mg.example.com`) |
| `webhookSigningKey` | string | No       | Optional HTTP webhook signing key for verification    |

## Usage

Configure the provider with your private API key from the [Mailgun dashboard](https://app.mailgun.com/settings/api_security). The `apiRegion` must match the region where your sending domain is hosted.

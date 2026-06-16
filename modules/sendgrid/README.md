# SendGrid Provider

Email provider using [SendGrid](https://sendgrid.com) for transactional email delivery and event webhooks.

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apiKey` | string | Yes | Your SendGrid API key |
| `webhookVerificationKey` | string | No | SendGrid Event Webhook verification key. Paste the base64 key from the SendGrid dashboard as-is; PEM-armored keys are also accepted. |

## Usage

Configure the provider with your SendGrid API key from the [SendGrid dashboard](https://app.sendgrid.com/settings/api_keys).

# AhaSend Provider

Email provider using [AhaSend](https://ahasend.com) for transactional email delivery.

## Configuration

| Field           | Type   | Required | Description                                                        |
|-----------------|--------|----------|--------------------------------------------------------------------|
| `apiKey`        | string | Yes      | AhaSend API key v2 with the `messages:send` scope                   |
| `accountId`     | string | Yes      | AhaSend account ID (UUID) from the dashboard's account settings     |
| `webhookSecret` | string | No       | Webhook secret (`aha-whsec-…`) used to verify webhook signatures    |

## Usage

Create an API key under **Credentials** in the [AhaSend dashboard](https://dash.ahasend.com) and scope it to `messages:send:{your-domain}`. Messages are sent through the Create Conversation Message endpoint, which carries cc and bcc and runs no substitutions over the rendered content.

## Development

`main.go` holds the WASM exports and builds with TinyGo only (`make wasm` here, or `make modules` from the repository root). The rest of the module runs under the standard toolchain:

```bash
go test ./...
```

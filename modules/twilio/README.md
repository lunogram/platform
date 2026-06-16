# Twilio SMS Provider

Provider for sending SMS messages via [Twilio](https://twilio.com) with webhook-based delivery tracking.

## Channels

| Channel | Supported |
|---------|-----------|
| SMS     | ✅        |
| Email   | ❌        |
| Push    | ❌        |

## Configuration

| Field         | Type   | Required | Hidden | Description                                                                 |
|---------------|--------|----------|--------|-----------------------------------------------------------------------------|
| `accountSid`  | string | Yes      | No     | Your Twilio Account SID                                                     |
| `authToken`   | string | Yes      | No     | Your Twilio Auth Token                                                      |
| `phoneNumber` | string | Yes      | No     | Twilio phone number in E.164 format (e.g. `+15551234567`) used as the default sender |
| `webhookUrl`  | string | No       | Yes    | Platform webhook callback URL (auto-configured during `init`)               |

## Setup

1. Get your Account SID and Auth Token from the [Twilio Console](https://console.twilio.com)
2. Purchase or configure a Twilio phone number capable of sending SMS
3. Configure the provider with your credentials and phone number

## How It Works

### Sending

The provider uses the [Twilio Messages API](https://www.twilio.com/docs/sms/api/message-resource) via the official `twilio-go` SDK to send SMS messages. Each outgoing message includes a `StatusCallback` URL so that Twilio reports delivery status changes back to the platform.

If the SMS payload includes a `from` number, it takes precedence over the configured `phoneNumber`. Media URLs in the payload are sent as MMS.

### Delivery Tracking

Twilio does not use a global webhook registration — instead, delivery callbacks are set **per-message** via the `StatusCallback` parameter. During `init`, the platform's webhook URL is stored in the provider config and automatically attached to every outgoing message.

Twilio POSTs `x-www-form-urlencoded` status updates with an `X-Twilio-Signature` header. The webhook handler verifies the signature using Twilio's request validation algorithm before processing events.

### Status Mapping

| Twilio Status   | Canonical Event         |
|-----------------|-------------------------|
| `sent`          | `provider.sent`         |
| `delivered`     | `provider.delivered`    |
| `read`          | `provider.delivered`    |
| `failed`        | `provider.bounced`      |
| `undelivered`   | `provider.bounced`      |
| `queued`        | `provider.deferred`     |
| `accepted`      | `provider.deferred`     |
| `sending`       | `provider.deferred`     |

## Building

```sh
make wasm
```

This produces a WASM module at `internal/integrations/modules/twilio.wasm` using TinyGo.

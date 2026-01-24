# Twilio Provider

Provider for sending emails and SMS via [Twilio](https://twilio.com).

## Configuration

| Field        | Type   | Required | Description            |
|--------------|--------|----------|------------------------|
| `accountSid` | string | Yes      | Your Twilio Account SID |
| `authToken`  | string | Yes      | Your Twilio Auth Token  |

## Usage

1. Get your Account SID and Auth Token from the [Twilio Console](https://console.twilio.com)
2. For email, ensure SendGrid is enabled on your Twilio account
3. Configure the provider with your credentials

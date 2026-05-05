# Amazon SES Provider

Email provider using [Amazon SES](https://aws.amazon.com/ses/) for transactional email delivery.

## Configuration

| Field    | Type   | Required | Description                              |
|----------|--------|----------|------------------------------------------|
| `accessKeyId`     | string | Yes      | AWS Access Key ID                        |
| `secretAccessKey` | string | Yes      | AWS Secret Access Key                    |
| `region`          | string | Yes      | AWS region (for example `us-east-1`)     |
| `sessionToken`    | string | No       | Optional AWS session token               |

## Usage

Configure the provider with IAM credentials that can call SES send APIs.

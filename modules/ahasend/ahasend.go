package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/modules/providers"
)

const (
	// Exit codes for the AhaSend provider module.
	ExitSuccess   int32 = 0
	ExitTransient int32 = -1 // retryable error (e.g. rate limit, network issue)
	ExitPermanent int32 = -2 // non-retryable error (e.g. invalid recipient, auth failure)
)

const ahaSendAPIBase = "https://api.ahasend.com"

// Config holds the AhaSend provider configuration persisted by the platform.
type Config struct {
	APIKey        string `json:"apiKey"`
	AccountID     string `json:"accountId"`
	WebhookSecret string `json:"webhookSecret"`
}

func validateConfig(config Config) map[string]string {
	errs := make(map[string]string)
	if config.APIKey == "" {
		errs["apiKey"] = "API key is required"
	}
	if config.AccountID == "" {
		errs["accountId"] = "Account ID is required"
	} else if _, err := uuid.Parse(config.AccountID); err != nil {
		errs["accountId"] = "Account ID must be a UUID"
	}
	return errs
}

type ahaSendAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// ahaSendMessageRequest is the body of AhaSend's Create Conversation Message.
// That endpoint is used rather than Create Message because it carries cc and
// bcc, and it runs no substitutions over content Lunogram already rendered.
type ahaSendMessageRequest struct {
	From        ahaSendAddress    `json:"from"`
	To          []ahaSendAddress  `json:"to"`
	Cc          []ahaSendAddress  `json:"cc,omitempty"`
	Bcc         []ahaSendAddress  `json:"bcc,omitempty"`
	ReplyTo     *ahaSendAddress   `json:"reply_to,omitempty"`
	Subject     string            `json:"subject"`
	TextContent string            `json:"text_content,omitempty"`
	HTMLContent string            `json:"html_content,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type ahaSendMessageResult struct {
	ID        *string `json:"id"`
	Recipient struct {
		Email string `json:"email"`
	} `json:"recipient"`
	Status string  `json:"status"`
	Error  *string `json:"error"`
}

type ahaSendMessageResponse struct {
	Data []ahaSendMessageResult `json:"data"`
}

// ComposeSendEmailRequest converts a generic EmailPayload to an AhaSend request body.
func ComposeSendEmailRequest(email providers.EmailPayload) (ahaSendMessageRequest, error) {
	if email.Text == "" && email.HTML == "" {
		return ahaSendMessageRequest{}, errors.New("email must have at least text or html content")
	}

	req := ahaSendMessageRequest{
		From:        ahaSendAddress{Email: email.From.Address, Name: email.From.Name},
		To:          []ahaSendAddress{{Email: email.To}},
		Subject:     email.Subject,
		TextContent: email.Text,
		HTMLContent: email.HTML,
	}

	if email.Cc != nil && *email.Cc != "" {
		req.Cc = []ahaSendAddress{{Email: *email.Cc}}
	}
	if email.Bcc != nil && *email.Bcc != "" {
		req.Bcc = []ahaSendAddress{{Email: *email.Bcc}}
	}
	if email.ReplyTo != nil && *email.ReplyTo != "" {
		req.ReplyTo = &ahaSendAddress{Email: *email.ReplyTo}
	}

	if len(email.Headers) > 0 || email.List != nil {
		req.Headers = make(map[string]string, len(email.Headers)+1)
		for k, v := range email.Headers {
			req.Headers[k] = v
		}
		if email.List != nil && email.List.Unsubscribe != "" {
			req.Headers["List-Unsubscribe"] = email.List.Unsubscribe
		}
	}

	return req, nil
}

// messageID returns the ID of the copy sent to the To recipient. Every cc and
// bcc recipient gets a copy with an ID of its own.
func messageID(resp ahaSendMessageResponse, to string) (string, error) {
	if len(resp.Data) == 0 {
		return "", errors.New("ahasend response contains no messages")
	}

	result := resp.Data[0]
	for _, candidate := range resp.Data {
		if strings.EqualFold(candidate.Recipient.Email, to) {
			result = candidate
			break
		}
	}

	if result.Status == "error" || result.ID == nil {
		reason := "no message id returned"
		if result.Error != nil {
			reason = *result.Error
		}
		return "", fmt.Errorf("ahasend did not send to %s: %s", result.Recipient.Email, reason)
	}

	return *result.ID, nil
}

func classifyHTTPStatus(status int) int32 {
	switch {
	case status == 409: // an earlier attempt with the same idempotency key is still in progress
		return ExitTransient
	case status == 429:
		return ExitTransient
	case status >= 500:
		return ExitTransient
	case status >= 400:
		return ExitPermanent
	default:
		return ExitTransient
	}
}

func main() {}

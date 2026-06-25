package main

import (
	"fmt"
	"strings"

	"github.com/lunogram/platform/pkg/modules/providers"
)

// Exit code convention for WASM provider modules:
//
//	 0  - success
//	-1  - transient/retryable error  (rate limit, network, server error)
//	-2  - permanent/non-retryable error (invalid recipient, validation, auth)
const (
	ExitTransient int32 = -1
	ExitPermanent int32 = -2
	ExitSuccess   int32 = 0
)

// Config holds the SendGrid provider configuration persisted by the platform.
type Config struct {
	APIKey                 string `json:"apiKey"`
	WebhookVerificationKey string `json:"webhookVerificationKey"`
}

func validateConfig(config Config) map[string]string {
	errs := make(map[string]string)
	if config.APIKey == "" {
		errs["apiKey"] = "API key is required"
	}
	return errs
}

type sendGridAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type sendGridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type sendGridPersonalization struct {
	To      []sendGridAddress `json:"to"`
	Cc      []sendGridAddress `json:"cc,omitempty"`
	Bcc     []sendGridAddress `json:"bcc,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type sendGridMailRequest struct {
	Personalizations []sendGridPersonalization `json:"personalizations"`
	From             sendGridAddress           `json:"from"`
	ReplyTo          *sendGridAddress          `json:"reply_to,omitempty"`
	Subject          string                    `json:"subject"`
	Content          []sendGridContent         `json:"content"`
}

type sendGridErrorBody struct {
	Errors []struct {
		Message string `json:"message"`
		Field   string `json:"field"`
		Help    string `json:"help"`
	} `json:"errors"`
}

// ComposeSendEmailRequest converts platform email payload to a SendGrid request body.
func ComposeSendEmailRequest(email providers.EmailPayload) sendGridMailRequest {
	p := sendGridPersonalization{To: []sendGridAddress{{Email: email.To}}}
	if email.Cc != nil && *email.Cc != "" {
		p.Cc = []sendGridAddress{{Email: *email.Cc}}
	}
	if email.Bcc != nil && *email.Bcc != "" {
		p.Bcc = []sendGridAddress{{Email: *email.Bcc}}
	}
	if len(email.Headers) > 0 {
		p.Headers = email.Headers
	}

	var content []sendGridContent
	if email.Text != "" {
		content = append(content, sendGridContent{Type: "text/plain", Value: email.Text})
	}
	if email.HTML != "" {
		content = append(content, sendGridContent{Type: "text/html", Value: email.HTML})
	}

	req := sendGridMailRequest{
		Personalizations: []sendGridPersonalization{p},
		From: sendGridAddress{
			Email: email.From.Address,
			Name:  email.From.Name,
		},
		Subject: email.Subject,
		Content: content,
	}

	if email.ReplyTo != nil && *email.ReplyTo != "" {
		req.ReplyTo = &sendGridAddress{Email: *email.ReplyTo}
	}

	return req
}

// classifyHTTPStatus maps SendGrid HTTP status codes to WASM exit codes.
func classifyHTTPStatus(status int) int32 {
	if status == 429 {
		return ExitTransient
	}
	if status >= 400 && status < 500 {
		return ExitPermanent
	}
	return ExitTransient
}

func formatSendGridErrors(body sendGridErrorBody) string {
	parts := make([]string, 0, len(body.Errors))
	for _, e := range body.Errors {
		msg := e.Message
		if e.Field != "" {
			msg = fmt.Sprintf("%s (field=%s)", msg, e.Field)
		}
		if e.Help != "" {
			msg = fmt.Sprintf("%s (help=%s)", msg, e.Help)
		}
		parts = append(parts, msg)
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "; ")
}

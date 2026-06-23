package main

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/lunogram/platform/pkg/modules/providers"
)

const (
	// Exit codes for the Mailgun provider module.
	ExitSuccess   int32 = 0
	ExitTransient int32 = -1 // retryable error (e.g. rate limit, network issue)
	ExitPermanent int32 = -2 // non-retryable error (e.g. invalid recipient, auth failure)
)

// Config holds the Mailgun provider configuration persisted by the platform.
type Config struct {
	APIKey            string `json:"apiKey"`
	APIRegion         string `json:"apiRegion"`
	Domain            string `json:"domain"`
	WebhookSigningKey string `json:"webhookSigningKey"`
	WebhookURL        string `json:"webhookUrl"`
}

type mailgunSendRequest struct {
	Domain string
	Form   url.Values
}

func validateConfig(config Config) map[string]string {
	errs := make(map[string]string)
	if config.APIKey == "" {
		errs["apiKey"] = "API key is required"
	}
	if config.Domain == "" {
		errs["domain"] = "Domain is required"
	}
	if config.APIRegion != "" {
		region := strings.ToUpper(strings.TrimSpace(config.APIRegion))
		if region != "US" && region != "EU" {
			errs["apiRegion"] = "API region must be 'US' or 'EU'"
		}
	}
	return errs
}

func formatAddress(address providers.EmailAddress) string {
	if address.Name != "" {
		return fmt.Sprintf("%s <%s>", address.Name, address.Address)
	}
	return address.Address
}

// ComposeSendEmailRequest converts a generic EmailPayload to Mailgun API form fields.
func ComposeSendEmailRequest(email providers.EmailPayload, configuredDomain string) (*mailgunSendRequest, error) {
	if email.Text == "" && email.HTML == "" {
		return nil, errors.New("email must have at least text or html content")
	}

	domain := configuredDomain
	if domain == "" {
		at := strings.LastIndex(email.From.Address, "@")
		if at <= 0 || at == len(email.From.Address)-1 {
			return nil, fmt.Errorf("invalid from address: %q", email.From.Address)
		}
		domain = email.From.Address[at+1:]
	}

	form := url.Values{}
	form.Set("from", formatAddress(email.From))
	form.Set("to", email.To)
	form.Set("subject", email.Subject)
	if email.Text != "" {
		form.Set("text", email.Text)
	}
	if email.HTML != "" {
		form.Set("html", email.HTML)
	}
	if email.Cc != nil && *email.Cc != "" {
		form.Set("cc", *email.Cc)
	}
	if email.Bcc != nil && *email.Bcc != "" {
		form.Set("bcc", *email.Bcc)
	}
	if email.ReplyTo != nil && *email.ReplyTo != "" {
		form.Set("h:Reply-To", *email.ReplyTo)
	}
	for k, v := range email.Headers {
		form.Set("h:"+k, v)
	}
	if email.List != nil && email.List.Unsubscribe != "" {
		form.Set("h:List-Unsubscribe", email.List.Unsubscribe)
	}

	return &mailgunSendRequest{
		Domain: domain,
		Form:   form,
	}, nil
}

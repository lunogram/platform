package mailer

import (
	"embed"
	"fmt"
	"html/template"
	"net/url"
	"strings"
	texttemplate "text/template"
	"time"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// content is the shape every auth message takes: a heading, a short
// explanation, at most one call to action and a closing note.
//
// There is one template pair rather than one per message because these are
// transactional notices, not marketing: giving each its own layout would mean
// four places to fix the next rendering bug and four chances for them to drift.
// The copy lives in the [Renderer] methods below.
type content struct {
	ProductName string
	Subject     string
	Heading     string
	Body        []string
	ActionLabel string
	ActionURL   string
	Footer      string
}

// Renderer turns an auth event into a [Message]. It is safe for concurrent use.
type Renderer struct {
	html        *template.Template
	text        *texttemplate.Template
	productName string
	baseURL     string
}

func NewRenderer(productName, baseURL string) (*Renderer, error) {
	html, err := template.ParseFS(templateFS, "templates/message.html.tmpl")
	if err != nil {
		return nil, err
	}

	text, err := texttemplate.ParseFS(templateFS, "templates/message.txt.tmpl")
	if err != nil {
		return nil, err
	}

	if productName == "" {
		productName = "Lunogram"
	}

	return &Renderer{
		html:        html,
		text:        text,
		productName: productName,
		baseURL:     strings.TrimRight(baseURL, "/"),
	}, nil
}

// BaseURL is the public origin every link in an auth message is built from.
func (r *Renderer) BaseURL() string { return r.baseURL }

// link builds an absolute console URL carrying a single-use token.
func (r *Renderer) link(path, token string) string {
	return r.baseURL + path + "?token=" + url.QueryEscape(token)
}

func (r *Renderer) VerifyEmail(to, token string) Message {
	actionURL := r.link("/verify-email", token)
	return r.render(content{
		Subject: "Confirm your email address",
		Heading: "Confirm your email address",
		Body: []string{
			fmt.Sprintf("Someone created a %s account with this address. Confirm it to finish setting the account up.", r.productName),
		},
		ActionLabel: "Confirm email address",
		ActionURL:   actionURL,
		Footer:      "This link expires in 24 hours. If you did not create an account you can ignore this message.",
	}, to, actionURL)
}

func (r *Renderer) PasswordReset(to, token string, ttl time.Duration) Message {
	actionURL := r.link("/reset-password", token)
	return r.render(content{
		Subject: "Reset your password",
		Heading: "Reset your password",
		Body: []string{
			fmt.Sprintf("Someone asked to reset the password for your %s account. Choose a new one to continue.", r.productName),
		},
		ActionLabel: "Choose a new password",
		ActionURL:   actionURL,
		Footer: fmt.Sprintf(
			"This link expires in %s and can be used once. If you did not ask for it, ignore this message -- your password stays as it is.",
			humaniseTTL(ttl)),
	}, to, actionURL)
}

// AccountExists answers a registration attempt on an address that already has an
// account.
//
// The HTTP response to that attempt is identical to the response for a brand-new
// address, because differing responses are how an account list gets scraped.
// This message is where the person who actually owns the address -- and only
// they -- is told what happened.
func (r *Renderer) AccountExists(to, resetToken string) Message {
	actionURL := r.link("/reset-password", resetToken)
	return r.render(content{
		Subject: "You already have an account",
		Heading: "You already have an account",
		Body: []string{
			fmt.Sprintf("Someone tried to register a %s account with this address, but it already has one. There is nothing to do -- sign in as usual.", r.productName),
			"If that was you and you cannot remember your password, you can set a new one.",
		},
		ActionLabel: "Set a new password",
		ActionURL:   actionURL,
		Footer:      "This link expires in one hour. If you did not try to register, ignore this message.",
	}, to, actionURL)
}

// PasswordChanged tells the account owner their password moved. It carries no
// action link on purpose: it is a notice, and a notice that asks you to click
// something teaches the exact reflex phishing relies on.
func (r *Renderer) PasswordChanged(to string) Message {
	return r.render(content{
		Subject: "Your password was changed",
		Heading: "Your password was changed",
		Body: []string{
			fmt.Sprintf("The password on your %s account has just been changed, and every other session it was signed in to has been ended.", r.productName),
			"If this was not you, reset your password now and contact your administrator.",
		},
		Footer: fmt.Sprintf("You can sign in at %s.", r.baseURL),
	}, to, "")
}

func (r *Renderer) render(c content, to, actionURL string) Message {
	c.ProductName = r.productName

	var html, text strings.Builder
	// Both templates are compiled at construction from an embedded source and
	// every field is a plain string, so execution cannot fail for a reason the
	// caller could act on. A partial body is still better than no mail at all.
	_ = r.html.Execute(&html, c)
	_ = r.text.Execute(&text, c)

	return Message{
		To:        to,
		Subject:   c.Subject,
		HTML:      html.String(),
		Text:      text.String(),
		ActionURL: actionURL,
	}
}

func humaniseTTL(ttl time.Duration) string {
	switch {
	case ttl >= 48*time.Hour:
		return fmt.Sprintf("%d days", int(ttl.Hours()/24))
	case ttl >= 2*time.Hour:
		return fmt.Sprintf("%d hours", int(ttl.Hours()))
	case ttl >= time.Hour:
		return "one hour"
	default:
		return fmt.Sprintf("%d minutes", int(ttl.Minutes()))
	}
}

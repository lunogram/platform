package mailer

// Kinds of message the platform sends. The kind is the key an operator
// overrides a template under, the value carried to a webhook receiver, and the
// name any error names, so it is one constant rather than three strings.
const (
	KindVerifyEmail     = "verify_email"
	KindPasswordReset   = "password_reset"
	KindAccountExists   = "account_exists"
	KindPasswordChanged = "password_changed"
)

// Templates holds the operator's overrides.
//
// Every field is optional and every field is a reference in the sense of
// internal/configfile: a base64:// payload, a file:// path, or a literal. An
// empty field falls back to the embedded default, so a deployment that
// configures nothing still sends the messages this package ships, and one that
// wants a different footer changes a footer rather than adopting a whole
// template set.
type Templates struct {
	Layout          Layout  `envPrefix:"LAYOUT_" yaml:"layout"`
	VerifyEmail     Content `envPrefix:"VERIFY_EMAIL_" yaml:"verify_email"`
	PasswordReset   Content `envPrefix:"PASSWORD_RESET_" yaml:"password_reset"`
	AccountExists   Content `envPrefix:"ACCOUNT_EXISTS_" yaml:"account_exists"`
	PasswordChanged Content `envPrefix:"PASSWORD_CHANGED_" yaml:"password_changed"`
}

// Layout is the chrome every message is rendered into.
//
// There is one layout rather than one per message because these are
// transactional notices, not marketing: giving each its own would mean four
// places to fix the next rendering bug and four chances for them to drift. A
// deployment that genuinely needs a different frame per message can branch on
// .Kind inside its own layout.
type Layout struct {
	HTML string `env:"HTML" yaml:"html"`
	Text string `env:"TEXT" yaml:"text"`
}

// Content is the copy of one message. Each field is a text/template evaluated
// against [TemplateData].
type Content struct {
	Subject     string `env:"SUBJECT" yaml:"subject"`
	Heading     string `env:"HEADING" yaml:"heading"`
	ActionLabel string `env:"ACTION_LABEL" yaml:"action_label"`
	Footer      string `env:"FOOTER" yaml:"footer"`

	// Body is the paragraphs between the heading and the call to action. The
	// environment form takes them separated by a pipe, because the one
	// separator that reads naturally -- a newline -- is the character
	// interpolation refuses to carry.
	Body []string `env:"BODY" envSeparator:"|" yaml:"body"`
}

// TemplateData is what every template in a message is evaluated against.
type TemplateData struct {
	// Kind is one of the Kind constants.
	Kind string
	// ProductName is what the messages call this deployment.
	ProductName string
	// Recipient is the address the message is addressed to.
	Recipient string
	// ActionURL is the link the message asks the recipient to follow. It is
	// empty for a message that deliberately carries no link.
	ActionURL string
	// BaseURL is the console's public origin.
	BaseURL string
	// ExpiresIn is how long the link remains valid, already written out for a
	// human ("one hour", "24 hours"). Empty when there is no link.
	ExpiresIn string
}

// defaultContent is the copy shipped with the platform.
//
// It lives here rather than in the template files because these are four short
// pieces of prose, and a file per field would be sixteen files to read in order
// to answer "what does the reset email say".
var defaultContent = map[string]Content{
	KindVerifyEmail: {
		Subject: "Confirm your email address",
		Heading: "Confirm your email address",
		Body: []string{
			"Someone created a {{ .ProductName }} account with this address. Confirm it to finish setting the account up.",
		},
		ActionLabel: "Confirm email address",
		Footer:      "This link expires in {{ .ExpiresIn }}. If you did not create an account you can ignore this message.",
	},
	KindPasswordReset: {
		Subject: "Reset your password",
		Heading: "Reset your password",
		Body: []string{
			"Someone asked to reset the password for your {{ .ProductName }} account. Choose a new one to continue.",
		},
		ActionLabel: "Choose a new password",
		Footer:      "This link expires in {{ .ExpiresIn }} and can be used once. If you did not ask for it, ignore this message: your password stays as it is.",
	},
	// The HTTP response to a registration on a taken address is identical to the
	// response for a brand-new one, because differing responses are how an
	// account list gets scraped. This message is where the person who actually
	// owns the address -- and only they -- is told what happened.
	KindAccountExists: {
		Subject: "You already have an account",
		Heading: "You already have an account",
		Body: []string{
			"Someone tried to register a {{ .ProductName }} account with this address, but it already has one. There is nothing to do: sign in as usual.",
			"If that was you and you cannot remember your password, you can set a new one.",
		},
		ActionLabel: "Set a new password",
		Footer:      "This link expires in {{ .ExpiresIn }}. If you did not try to register, ignore this message.",
	},
	// This one carries no action link on purpose: it is a notice, and a notice
	// that asks you to click something teaches the exact reflex phishing relies
	// on.
	KindPasswordChanged: {
		Subject: "Your password was changed",
		Heading: "Your password was changed",
		Body: []string{
			"The password on your {{ .ProductName }} account has just been changed, and every other session it was signed in to has been ended.",
			"If this was not you, reset your password now and contact your administrator.",
		},
		Footer: "You can sign in at {{ .BaseURL }}.",
	},
}

// override returns the operator's content for a kind, or the zero value when
// they configured none.
func (t Templates) override(kind string) Content {
	switch kind {
	case KindVerifyEmail:
		return t.VerifyEmail
	case KindPasswordReset:
		return t.PasswordReset
	case KindAccountExists:
		return t.AccountExists
	case KindPasswordChanged:
		return t.PasswordChanged
	default:
		return Content{}
	}
}

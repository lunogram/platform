package mailer

// Kinds of message the platform sends. The kind is the value carried to a
// webhook receiver and the name any log line or error names, so it is one
// constant rather than two strings. It follows the noun.verb shape the
// platform's webhook events use, so a receiver routing on both reads one
// vocabulary rather than two.
const (
	KindVerifyEmail     = "email.verify"
	KindPasswordReset   = "password.reset"
	KindAccountExists   = "account.exists"
	KindPasswordChanged = "password.changed"
	KindProjectInvite   = "project.invite"
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
	ProjectInvite   Content `envPrefix:"PROJECT_INVITE_" yaml:"project_invite"`
}

// Layout is the chrome every message is rendered into.
//
// There is one layout rather than one per message because these are
// transactional notices, not marketing: giving each its own would mean a place
// per message to fix the next rendering bug, and as many chances for them to
// drift. A deployment that genuinely needs a different frame per message can
// branch on .Kind inside its own layout.
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
	// ProjectName is the project a message is about. Only KindProjectInvite
	// sets it.
	ProjectName string
	// InviterName is who sent the invitation, as a name when the account has
	// one and an address otherwise. Only KindProjectInvite sets it.
	InviterName string
}

// defaultContent is the copy shipped with the platform.
//
// It lives here rather than in the template files because these are a handful
// of short pieces of prose, and a file per field would be dozens of files to
// read in order to answer "what does the reset email say".
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
	// The link is the console's invite page rather than a token: an invite is
	// bound to the invitee's address and is claimed by proving that address, so
	// a link that granted access on its own would hand the project to whoever
	// forwarded the mail. Following it asks them to sign in -- or, on a
	// deployment that admits them, to register first.
	KindProjectInvite: {
		Subject: "{{ .InviterName }} invited you to {{ .ProjectName }}",
		Heading: "You have been invited to {{ .ProjectName }}",
		Body: []string{
			"{{ .InviterName }} invited you to work on {{ .ProjectName }} in {{ .ProductName }}.",
			"Sign in with this address to accept. If you do not have an account yet, you will be asked to create one first.",
		},
		ActionLabel: "View invitation",
		Footer:      "This invitation expires in {{ .ExpiresIn }}. If you were not expecting it you can ignore this message.",
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
	case KindProjectInvite:
		return t.ProjectInvite
	default:
		return Content{}
	}
}

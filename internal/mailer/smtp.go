package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// SMTP delivers mail over SMTP using the standard library's net/smtp.
//
// net/smtp is frozen rather than actively developed, and it is still the right
// choice here: this package sends a handful of short, single-recipient
// transactional messages with no attachments, which is exactly the surface
// net/smtp covers. Every third-party alternative would add a dependency to the
// server binary in exchange for features (attachments, pooling, DKIM) that auth
// mail does not use. It also enforces a property worth having for free: PLAIN
// authentication is refused on an unencrypted connection to a non-local server,
// so a misconfigured TLS mode cannot silently leak the SMTP password.
type SMTP struct {
	config Config
	from   mail.Address
}

func NewSMTP(config Config) (*SMTP, error) {
	if !config.Configured() {
		return nil, fmt.Errorf("mailer: MAIL_HOST is not set")
	}
	if config.FromAddress == "" {
		return nil, fmt.Errorf("mailer: MAIL_FROM_ADDRESS is required when MAIL_HOST is set")
	}
	switch config.TLS {
	case TLSStartTLS, TLSImplicit, TLSNone:
	default:
		return nil, fmt.Errorf("mailer: unknown MAIL_TLS %q, expected starttls, implicit or none", config.TLS)
	}

	return &SMTP{
		config: config,
		from:   mail.Address{Name: config.FromName, Address: config.FromAddress},
	}, nil
}

func (s *SMTP) Send(ctx context.Context, message Message) error {
	recipient, err := mail.ParseAddress(message.To)
	if err != nil {
		return fmt.Errorf("mailer: invalid recipient: %w", err)
	}

	client, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer client.Close() //nolint:errcheck

	if err := s.authenticate(client); err != nil {
		return err
	}

	if err := client.Mail(s.from.Address); err != nil {
		return fmt.Errorf("mailer: MAIL FROM rejected: %w", err)
	}
	if err := client.Rcpt(recipient.Address); err != nil {
		return fmt.Errorf("mailer: RCPT TO rejected: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: DATA rejected: %w", err)
	}
	if _, err := writer.Write(s.render(*recipient, message)); err != nil {
		return fmt.Errorf("mailer: failed to write message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("mailer: server rejected the message: %w", err)
	}

	return client.Quit()
}

// connect dials the server and returns a client whose connection carries the
// deadline derived from ctx. net/smtp predates context, so the deadline on the
// underlying connection is what actually bounds every subsequent command.
func (s *SMTP) connect(ctx context.Context) (*smtp.Client, error) {
	address := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("mailer: failed to connect to %s: %w", address, err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			conn.Close() //nolint:errcheck
			return nil, err
		}
	}

	tlsConfig := &tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12}

	if s.config.TLS == TLSImplicit {
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close() //nolint:errcheck
			return nil, fmt.Errorf("mailer: TLS handshake with %s failed: %w", address, err)
		}
		conn = tlsConn
	}

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		conn.Close() //nolint:errcheck
		return nil, fmt.Errorf("mailer: SMTP greeting from %s failed: %w", address, err)
	}

	if s.config.TLS == TLSStartTLS {
		// Fail rather than fall back to plaintext. A server that stopped
		// offering STARTTLS is either misconfigured or being stripped, and
		// continuing would hand it the credentials in the clear.
		if ok, _ := client.Extension("STARTTLS"); !ok {
			client.Close() //nolint:errcheck
			return nil, fmt.Errorf("mailer: %s does not offer STARTTLS", address)
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			client.Close() //nolint:errcheck
			return nil, fmt.Errorf("mailer: STARTTLS with %s failed: %w", address, err)
		}
	}

	return client, nil
}

func (s *SMTP) authenticate(client *smtp.Client) error {
	if s.config.Username == "" {
		return nil
	}

	ok, mechanisms := client.Extension("AUTH")
	if !ok {
		return fmt.Errorf("mailer: %s does not offer AUTH but MAIL_USERNAME is set", s.config.Host)
	}

	var auth smtp.Auth
	switch {
	case strings.Contains(mechanisms, "PLAIN"):
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	case strings.Contains(mechanisms, "CRAM-MD5"):
		auth = smtp.CRAMMD5Auth(s.config.Username, s.config.Password)
	default:
		return fmt.Errorf("mailer: %s offers no supported AUTH mechanism (%s)", s.config.Host, mechanisms)
	}

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("mailer: authentication with %s failed: %w", s.config.Host, err)
	}
	return nil
}

// render builds the RFC 5322 message as a multipart/alternative with the plain
// text part first, which is the order clients use to pick the richest part they
// can display.
func (s *SMTP) render(recipient mail.Address, message Message) []byte {
	boundary := "lunogram-" + strconv.FormatInt(time.Now().UnixNano(), 36)

	var b strings.Builder
	b.WriteString("From: " + s.from.String() + "\r\n")
	b.WriteString("To: " + recipient.String() + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", headerSafe(message.Subject)) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	// Transactional mail must never be auto-replied to, and must not be filed as
	// bulk by the recipient's provider.
	b.WriteString("Auto-Submitted: auto-generated\r\n")
	b.WriteString(`Content-Type: multipart/alternative; boundary="` + boundary + "\"\r\n\r\n")

	writePart := func(contentType, body string) {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: " + contentType + "; charset=utf-8\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		b.WriteString(quotedPrintable(body))
		b.WriteString("\r\n")
	}

	writePart("text/plain", message.Text)
	writePart("text/html", message.HTML)
	b.WriteString("--" + boundary + "--\r\n")

	return []byte(b.String())
}

// quotedPrintable encodes a body for transport. Encoding is not optional here:
// an action URL can easily push a line past the 998-octet limit SMTP imposes,
// and a folded line is what keeps the message intact when it does.
func quotedPrintable(body string) string {
	var out strings.Builder
	writer := quotedprintable.NewWriter(&out)
	writer.Write([]byte(body)) //nolint:errcheck
	writer.Close()             //nolint:errcheck
	return out.String()
}

// headerSafe strips anything that could terminate a header line early. Q-encoding
// passes plain ASCII through verbatim, so a bare CR or LF reaching a header is
// header injection.
func headerSafe(value string) string {
	return textproto.TrimString(strings.NewReplacer("\r", "", "\n", "").Replace(value))
}

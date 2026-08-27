package mailer

import (
	"context"

	"go.uber.org/zap"
)

// LogMailer writes messages to the log instead of delivering them. It is what
// runs when MAIL_HOST is unset, which keeps the docker-compose quickstart and
// local development working with zero configuration: a self-hoster must be able
// to create an account, verify it and reset its password without first standing
// up an SMTP server.
//
// It logs the full action URL rather than the token, because the URL is what the
// operator has to paste into a browser and because a token on its own in a log
// line invites someone to build tooling around it.
type LogMailer struct {
	logger *zap.Logger
}

func NewLogMailer(logger *zap.Logger) *LogMailer {
	return &LogMailer{logger: logger}
}

func (m *LogMailer) Send(_ context.Context, message Message) error {
	m.logger.Info("DEVELOPMENT MAILER: no SMTP server is configured (set MAIL_HOST to deliver mail), so this message was written to the log instead of being sent",
		zap.String("to", message.To),
		zap.String("subject", message.Subject),
		zap.String("action_url", message.ActionURL),
		zap.String("body", message.Text),
	)
	return nil
}

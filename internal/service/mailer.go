package service

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// Mailer sends transactional emails.
type Mailer interface {
	SendPasswordReset(ctx context.Context, toEmail, resetToken string) error
}

// LogMailer logs emails to stdout instead of sending them. Useful for development.
type LogMailer struct {
	frontendURL string
}

func NewLogMailer(frontendURL string) Mailer {
	return &LogMailer{frontendURL: frontendURL}
}

func (m *LogMailer) SendPasswordReset(_ context.Context, toEmail, resetToken string) error {
	link := fmt.Sprintf("%s/reset-password?token=%s", m.frontendURL, resetToken)
	log.Info().
		Str("to", toEmail).
		Str("reset_link", link).
		Msg("password reset email (log-only mailer)")
	return nil
}

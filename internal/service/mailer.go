package service

import (
	"context"
	"fmt"
	"net/smtp"

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

// SmtpMailer sends real emails via SMTP using STARTTLS (port 587).
type SmtpMailer struct {
	host        string
	port        string
	username    string
	password    string
	fromAddress string
	frontendURL string
}

func NewSmtpMailer(host, port, username, password, fromAddress, frontendURL string) Mailer {
	return &SmtpMailer{
		host:        host,
		port:        port,
		username:    username,
		password:    password,
		fromAddress: fromAddress,
		frontendURL: frontendURL,
	}
}

func (m *SmtpMailer) SendPasswordReset(_ context.Context, toEmail, resetToken string) error {
	link := fmt.Sprintf("%s/reset-password?token=%s", m.frontendURL, resetToken)

	subject := "Redefinição de senha — Wedding MC"
	body := fmt.Sprintf("Olá!\r\n\r\nClique no link abaixo para redefinir sua senha:\r\n%s\r\n\r\nO link expira em 1 hora.\r\n\r\nSe você não solicitou a redefinição, ignore este e-mail.", link)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.fromAddress, toEmail, subject, body)

	auth := smtp.PlainAuth("", m.username, m.password, m.host)
	addr := m.host + ":" + m.port

	return smtp.SendMail(addr, auth, m.username, []string{toEmail}, []byte(msg))
}

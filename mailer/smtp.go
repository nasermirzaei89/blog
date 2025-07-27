package mailer

import (
	"context"
	"fmt"
	"net/smtp"
)

type SMTPMailer struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func (mailer *SMTPMailer) SendEmail(ctx context.Context, to, subject, body string) error {
	msg := []byte("To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")

	var auth smtp.Auth
	if mailer.Username != "" {
		auth = smtp.PlainAuth("", mailer.Username, mailer.Password, mailer.Host)
	} else {
		auth = nil
	}

	addr := fmt.Sprintf("%s:%s", mailer.Host, mailer.Port)
	err := smtp.SendMail(addr, auth, mailer.From, []string{to}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

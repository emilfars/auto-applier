package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
)

// Mailer sends account emails.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// SMTPMailer sends plain-text mail through an SMTP server.
type SMTPMailer struct {
	addr string
	host string
	from *mail.Address
	auth smtp.Auth
}

// NewSMTPMailer validates SMTP settings and returns a mailer.
func NewSMTPMailer(addr, username, password, from string) (*SMTPMailer, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("auth: parse SMTP address: %w", err)
	}
	sender, err := mail.ParseAddress(from)
	if err != nil {
		return nil, fmt.Errorf("auth: parse SMTP sender: %w", err)
	}
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}
	return &SMTPMailer{addr: addr, host: host, from: sender, auth: auth}, nil
}

// Send delivers one plain-text email.
func (m *SMTPMailer) Send(ctx context.Context, to, subject, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	recipient, err := mail.ParseAddress(to)
	if err != nil || recipient.Address != to {
		return fmt.Errorf("auth: invalid recipient")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("auth: invalid subject")
	}
	message := []byte("From: " + m.from.String() + "\r\n" +
		"To: " + recipient.String() + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body)
	// ponytail: net/smtp lacks context cancellation; replace it if blocked calls become measurable.
	client, err := smtp.Dial(m.addr)
	if err != nil {
		return fmt.Errorf("auth: connect SMTP: %w", err)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("auth: SMTP server does not support STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{ServerName: m.host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("auth: start SMTP TLS: %w", err)
	}
	if m.auth != nil {
		if err := client.Auth(m.auth); err != nil {
			return fmt.Errorf("auth: authenticate SMTP: %w", err)
		}
	}
	if err := client.Mail(m.from.Address); err != nil {
		return fmt.Errorf("auth: set SMTP sender: %w", err)
	}
	if err := client.Rcpt(recipient.Address); err != nil {
		return fmt.Errorf("auth: set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("auth: start SMTP body: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("auth: write SMTP body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("auth: close SMTP body: %w", err)
	}
	// DATA completion is the server's acceptance point; a later QUIT failure
	// must not make the service delete an account whose email was accepted.
	return nil
}

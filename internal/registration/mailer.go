package registration

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// SMTPConfig memuat konfigurasi koneksi email OTP.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// SMTPMailer mengirim email OTP melalui SMTP dengan STARTTLS bila tersedia.
type SMTPMailer struct {
	configuration SMTPConfig
	timeout       time.Duration
}

// NewSMTPMailer membuat pengirim email SMTP dengan timeout sepuluh detik.
func NewSMTPMailer(configuration SMTPConfig) *SMTPMailer {
	return &SMTPMailer{configuration: configuration, timeout: 10 * time.Second}
}

// SendOTP mengirim email teks berisi kode dan masa berlaku tanpa mencatat kode ke log.
func (mailer *SMTPMailer) SendOTP(ctx context.Context, to, code string, validFor time.Duration) error {
	address := net.JoinHostPort(mailer.configuration.Host, fmt.Sprint(mailer.configuration.Port))
	connection, err := (&net.Dialer{Timeout: mailer.timeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect SMTP: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(mailer.timeout))
	client, err := smtp.NewClient(connection, mailer.configuration.Host)
	if err != nil {
		return fmt.Errorf("initialize SMTP: %w", err)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: mailer.configuration.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	}
	if mailer.configuration.Username != "" {
		auth := smtp.PlainAuth("", mailer.configuration.Username, mailer.configuration.Password, mailer.configuration.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authenticate SMTP: %w", err)
		}
	}
	fromAddress := extractEmailAddress(mailer.configuration.From)
	if err := client.Mail(fromAddress); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message: %w", err)
	}
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Kode OTP Registrasi Dikasa\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nKode OTP Dikasa Anda: %s\r\nKode berlaku selama %d menit. Jangan bagikan kode ini kepada siapa pun.\r\n", mailer.configuration.From, to, code, int(validFor.Minutes()))
	buffered := bufio.NewWriter(writer)
	if _, err := buffered.WriteString(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := buffered.Flush(); err != nil {
		_ = writer.Close()
		return fmt.Errorf("flush SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("finish SMTP delivery: %w", err)
	}
	return nil
}

func extractEmailAddress(from string) string {
	start, end := strings.LastIndex(from, "<"), strings.LastIndex(from, ">")
	if start >= 0 && end > start {
		return strings.TrimSpace(from[start+1 : end])
	}
	return strings.TrimSpace(from)
}

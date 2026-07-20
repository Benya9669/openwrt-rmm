package httpapi

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

type PasswordResetSender interface {
	SendPasswordReset(ctx context.Context, recipient, resetURL string) error
}

type SMTPConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	From       string
	TLSMode    string
	ServerName string
}

type smtpPasswordResetSender struct {
	config SMTPConfig
	from   *mail.Address
}

func NewSMTPPasswordResetSender(config SMTPConfig) (PasswordResetSender, error) {
	config.Host = strings.TrimSpace(config.Host)
	config.Username = strings.TrimSpace(config.Username)
	config.From = strings.TrimSpace(config.From)
	config.TLSMode = strings.ToLower(strings.TrimSpace(config.TLSMode))
	config.ServerName = strings.TrimSpace(config.ServerName)
	if config.Host == "" || config.From == "" {
		return nil, errors.New("SMTP host and sender are required")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return nil, errors.New("SMTP port must be between 1 and 65535")
	}
	if config.TLSMode == "" {
		config.TLSMode = "starttls"
	}
	if config.TLSMode != "starttls" && config.TLSMode != "tls" && config.TLSMode != "none" {
		return nil, errors.New("SMTP TLS mode must be starttls, tls or none")
	}
	if config.TLSMode == "none" && (config.Username != "" || config.Password != "") {
		return nil, errors.New("SMTP authentication requires TLS")
	}
	from, err := mail.ParseAddress(config.From)
	if err != nil || strings.ContainsAny(config.From, "\r\n") {
		return nil, errors.New("SMTP sender is invalid")
	}
	if config.ServerName == "" {
		config.ServerName = config.Host
	}
	return &smtpPasswordResetSender{config: config, from: from}, nil
}

func (s *smtpPasswordResetSender) SendPasswordReset(ctx context.Context, recipient, resetURL string) error {
	to, err := mail.ParseAddress(strings.TrimSpace(recipient))
	if err != nil || strings.ContainsAny(recipient, "\r\n") {
		return errors.New("password reset recipient is invalid")
	}
	if !strings.HasPrefix(resetURL, "https://") && !strings.HasPrefix(resetURL, "http://") {
		return errors.New("password reset URL is invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	address := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	if s.config.TLSMode == "tls" {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.config.ServerName})
		if err != nil {
			return fmt.Errorf("connect to SMTP over TLS: %w", err)
		}
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return fmt.Errorf("connect to SMTP: %w", err)
		}
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	text := textproto.NewConn(conn)
	if _, _, err := text.ReadResponse(220); err != nil {
		return fmt.Errorf("read SMTP greeting: %w", err)
	}
	ehlo, err := smtpCommand(text, 250, "EHLO rmm.local")
	if err != nil {
		return fmt.Errorf("SMTP EHLO: %w", err)
	}
	if s.config.TLSMode == "starttls" {
		if !strings.Contains(strings.ToUpper(ehlo), "STARTTLS") {
			return errors.New("SMTP server does not support STARTTLS")
		}
		if _, err := smtpCommand(text, 220, "STARTTLS"); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
		tlsConn := tls.Client(conn, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.config.ServerName})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("negotiate SMTP TLS: %w", err)
		}
		conn = tlsConn
		text = textproto.NewConn(conn)
		if _, err := smtpCommand(text, 250, "EHLO rmm.local"); err != nil {
			return fmt.Errorf("SMTP EHLO after TLS: %w", err)
		}
	}
	if s.config.Username != "" {
		credentials := base64.StdEncoding.EncodeToString([]byte("\x00" + s.config.Username + "\x00" + s.config.Password))
		if _, err := smtpCommand(text, 235, "AUTH PLAIN %s", credentials); err != nil {
			return fmt.Errorf("authenticate to SMTP: %w", err)
		}
	}
	if _, err := smtpCommand(text, 250, "MAIL FROM:<%s>", s.from.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if _, err := smtpCommand(text, 250, "RCPT TO:<%s>", to.Address); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	if _, err := smtpCommand(text, 354, "DATA"); err != nil {
		return fmt.Errorf("open SMTP message: %w", err)
	}
	w := text.DotWriter()
	message := "From: " + s.from.String() + "\r\n" +
		"To: " + to.String() + "\r\n" +
		"Subject: OpenWrt RMM password reset\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n\r\n" +
		"A password reset was requested for your OpenWrt RMM account.\r\n\r\n" +
		resetURL + "\r\n\r\n" +
		"The link expires in 30 minutes. If you did not request this, ignore this message.\r\n"
	if _, err := w.Write([]byte(message)); err != nil {
		w.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("send SMTP message: %w", err)
	}
	if _, _, err := text.ReadResponse(250); err != nil {
		return fmt.Errorf("confirm SMTP message: %w", err)
	}
	_, _ = smtpCommand(text, 221, "QUIT")
	return nil
}

func smtpCommand(conn *textproto.Conn, expectedCode int, format string, args ...any) (string, error) {
	id, err := conn.Cmd(format, args...)
	if err != nil {
		return "", err
	}
	conn.StartResponse(id)
	defer conn.EndResponse(id)
	_, message, err := conn.ReadResponse(expectedCode)
	return message, err
}

package httpapi

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSMTPPasswordResetSender(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	messages := make(chan string, 1)
	serverErrors := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErrors <- err
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		if _, err := fmt.Fprint(conn, "220 smtp.test ESMTP\r\n"); err != nil {
			serverErrors <- err
			return
		}
		var message strings.Builder
		inData := false
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				serverErrors <- err
				return
			}
			trimmed := strings.TrimRight(line, "\r\n")
			if inData {
				if trimmed == "." {
					messages <- message.String()
					inData = false
					_, err = fmt.Fprint(conn, "250 queued\r\n")
				} else {
					message.WriteString(trimmed + "\n")
				}
			} else {
				switch {
				case strings.HasPrefix(trimmed, "EHLO "):
					_, err = fmt.Fprint(conn, "250-smtp.test\r\n250 OK\r\n")
				case strings.HasPrefix(trimmed, "MAIL FROM:"), strings.HasPrefix(trimmed, "RCPT TO:"):
					_, err = fmt.Fprint(conn, "250 OK\r\n")
				case trimmed == "DATA":
					inData = true
					_, err = fmt.Fprint(conn, "354 End data\r\n")
				case trimmed == "QUIT":
					_, err = fmt.Fprint(conn, "221 Bye\r\n")
					if err == nil {
						serverErrors <- nil
					}
					return
				default:
					err = fmt.Errorf("unexpected SMTP command %q", trimmed)
				}
			}
			if err != nil {
				serverErrors <- err
				return
			}
		}
	}()

	address := listener.Addr().(*net.TCPAddr)
	sender, err := NewSMTPPasswordResetSender(SMTPConfig{
		Host: "127.0.0.1", Port: address.Port, From: "RMM <rmm@example.test>", TLSMode: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	resetURL := "https://rmm.example.test/#password-reset=test-token"
	if err := sender.SendPasswordReset(context.Background(), "owner@example.test", resetURL); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-messages:
		if !strings.Contains(message, resetURL) || !strings.Contains(message, "To: <owner@example.test>") {
			t.Fatalf("unexpected SMTP message: %s", message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SMTP message was not received")
	}
	if err := <-serverErrors; err != nil {
		t.Fatal(err)
	}
}

func TestSMTPPasswordResetSenderRejectsAuthenticationWithoutTLS(t *testing.T) {
	_, err := NewSMTPPasswordResetSender(SMTPConfig{
		Host: "smtp.example.test", Port: 25, From: "rmm@example.test", TLSMode: "none", Username: "rmm",
	})
	if err == nil {
		t.Fatal("expected SMTP authentication without TLS to be rejected")
	}
}

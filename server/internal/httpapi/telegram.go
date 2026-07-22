package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type telegramNotificationSender struct {
	endpoint string
	client   *http.Client
}

func NewTelegramNotificationSender(token string) (NotificationSender, error) {
	token = strings.TrimSpace(token)
	if len(token) < 20 || len(token) > 256 || strings.ContainsAny(token, "\r\n\t /?") {
		return nil, errors.New("Telegram bot token is invalid")
	}
	return &telegramNotificationSender{
		endpoint: "https://api.telegram.org/bot" + token + "/sendMessage",
		client:   &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (s *telegramNotificationSender) SendNotification(ctx context.Context, destination, title, body string) error {
	if !validTelegramChatID(destination) {
		return errors.New("Telegram chat ID is invalid")
	}
	text := strings.TrimSpace(title) + "\n\n" + strings.TrimSpace(body)
	if len(text) > 4096 {
		text = text[:4096]
	}
	payload, err := json.Marshal(map[string]any{
		"chat_id":                  destination,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send Telegram notification: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Telegram API returned HTTP %d", response.StatusCode)
	}
	return nil
}

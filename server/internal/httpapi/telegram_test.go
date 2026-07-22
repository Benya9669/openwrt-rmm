package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTelegramNotificationSender(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected Telegram request: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	sender := &telegramNotificationSender{endpoint: server.URL, client: server.Client()}
	if err := sender.SendNotification(context.Background(), "-1001234567890", "Router offline", "Check the uplink"); err != nil {
		t.Fatal(err)
	}
	if payload["chat_id"] != "-1001234567890" || payload["text"] != "Router offline\n\nCheck the uplink" {
		t.Fatalf("unexpected Telegram payload: %#v", payload)
	}
}

func TestTelegramNotificationSenderRejectsInvalidConfiguration(t *testing.T) {
	if _, err := NewTelegramNotificationSender("short"); err == nil {
		t.Fatal("expected a short Telegram token to be rejected")
	}
	sender := &telegramNotificationSender{endpoint: "https://example.invalid", client: http.DefaultClient}
	if err := sender.SendNotification(context.Background(), "not-a-chat", "test", "test"); err == nil {
		t.Fatal("expected invalid Telegram chat ID to be rejected")
	}
}

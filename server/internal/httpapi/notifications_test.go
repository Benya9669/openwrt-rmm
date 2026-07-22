package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"rmm-openwrt/server/internal/httpapi"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

type capturedNotification struct {
	destination string
	title       string
	body        string
}

type captureNotificationSender struct {
	messages chan capturedNotification
}

func (s *captureNotificationSender) SendNotification(_ context.Context, destination, title, body string) error {
	s.messages <- capturedNotification{destination: destination, title: title, body: body}
	return nil
}

func TestNotificationSettingsTestAndAlertLifecycle(t *testing.T) {
	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "notifications.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sender := &captureNotificationSender{messages: make(chan capturedNotification, 8)}
	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{
		OperatorUsername: "admin",
		OperatorPassword: "notification-password-123",
		PublicURL:        "https://rmm.example.test",
		AlertEmailSender: sender,
	}))
	defer srv.Close()
	client := authenticatedClient(t, srv.URL, "admin", "notification-password-123")
	authRequestJSON(t, client, http.MethodPatch, srv.URL+"/api/auth/profile", map[string]any{
		"display_name": "Owner", "email": "owner@example.test",
	}, http.StatusOK, nil)

	var defaults struct {
		Settings model.NotificationSettings `json:"settings"`
	}
	authRequestJSON(t, client, http.MethodGet, srv.URL+"/api/notifications/settings", nil, http.StatusOK, &defaults)
	if defaults.Settings.Configured || defaults.Settings.MemoryThresholdPercent != 85 {
		t.Fatalf("unexpected defaults: %#v", defaults.Settings)
	}

	var saved struct {
		Settings model.NotificationSettings `json:"settings"`
	}
	authRequestJSON(t, client, http.MethodPut, srv.URL+"/api/notifications/settings", map[string]any{
		"email_enabled": true, "telegram_enabled": false, "notify_warning": true,
		"notify_critical": true, "notify_resolved": true, "memory_threshold_percent": 80,
		"disk_threshold_percent": 90, "packet_loss_percent": 25, "latency_threshold_ms": 250,
		"repeat_minutes": 0,
	}, http.StatusOK, &saved)
	if !saved.Settings.Configured || !saved.Settings.EmailEnabled || saved.Settings.MemoryThresholdPercent != 80 {
		t.Fatalf("settings were not saved: %#v", saved.Settings)
	}
	authRequestJSON(t, client, http.MethodPost, srv.URL+"/api/notifications/test", nil, http.StatusOK, nil)
	testMessage := waitNotification(t, sender.messages)
	if testMessage.destination != "owner@example.test" || testMessage.title == "" || testMessage.body == "" {
		t.Fatalf("unexpected test notification: %#v", testMessage)
	}

	enrolled := enrollForClient(t, srv.URL, client, "notify-router", "notify-router")
	requestJSON(t, http.MethodPost, srv.URL+"/api/agent/heartbeat", enrolled.DeviceToken, map[string]any{
		"device_id": enrolled.DeviceID,
		"inventory": map[string]any{"hostname": "notify-router"},
		"metrics":   map[string]any{"memory": map[string]any{"used_kb": 900, "total_kb": 1000}},
	}, http.StatusOK, nil)
	activeMessage := waitNotification(t, sender.messages)
	if activeMessage.title == "" {
		t.Fatal("active alert notification was empty")
	}

	requestJSON(t, http.MethodPost, srv.URL+"/api/agent/heartbeat", enrolled.DeviceToken, map[string]any{
		"device_id": enrolled.DeviceID,
		"inventory": map[string]any{"hostname": "notify-router"},
		"metrics":   map[string]any{"memory": map[string]any{"used_kb": 100, "total_kb": 1000}},
	}, http.StatusOK, nil)
	resolvedMessage := waitNotification(t, sender.messages)
	if resolvedMessage.title == activeMessage.title {
		t.Fatalf("resolved notification did not change state: active=%q resolved=%q", activeMessage.title, resolvedMessage.title)
	}

	var history struct {
		Notifications []model.NotificationDelivery `json:"notifications"`
	}
	authRequestJSON(t, client, http.MethodGet, srv.URL+"/api/notifications", nil, http.StatusOK, &history)
	if len(history.Notifications) < 3 {
		t.Fatalf("expected test, active and resolved deliveries, got %#v", history.Notifications)
	}
	for _, delivery := range history.Notifications {
		if delivery.Destination != "" {
			t.Fatal("raw notification destination leaked through API")
		}
	}
}

func TestNotificationSettingsValidation(t *testing.T) {
	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "notification-validation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{OperatorUsername: "admin", OperatorPassword: "notification-password-123"}))
	defer srv.Close()
	client := authenticatedClient(t, srv.URL, "admin", "notification-password-123")
	authRequestJSON(t, client, http.MethodPut, srv.URL+"/api/notifications/settings", map[string]any{
		"email_enabled": true, "notify_warning": true, "notify_critical": true, "notify_resolved": true,
		"memory_threshold_percent": 85, "disk_threshold_percent": 85, "packet_loss_percent": 20,
		"latency_threshold_ms": 200, "repeat_minutes": 0,
	}, http.StatusBadRequest, nil)
	authRequestJSON(t, client, http.MethodPut, srv.URL+"/api/notifications/settings", map[string]any{
		"telegram_enabled": true, "telegram_chat_id": "not-a-chat", "notify_warning": true,
		"notify_critical": true, "notify_resolved": true, "memory_threshold_percent": 85,
		"disk_threshold_percent": 85, "packet_loss_percent": 20, "latency_threshold_ms": 200,
		"repeat_minutes": 0,
	}, http.StatusBadRequest, nil)
}

func waitNotification(t *testing.T, messages <-chan capturedNotification) capturedNotification {
	t.Helper()
	select {
	case message := <-messages:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("notification was not delivered")
		return capturedNotification{}
	}
}

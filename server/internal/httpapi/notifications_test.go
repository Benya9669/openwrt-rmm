package httpapi_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rmm-openwrt/server/internal/httpapi"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"

	_ "modernc.org/sqlite"
)

type capturedNotification struct {
	destination string
	title       string
	body        string
}

func verifyEmailForTest(t *testing.T, st *store.Store, username, email string) {
	t.Helper()
	user, _, found, err := st.GetUserByUsername(context.Background(), username)
	if err != nil || !found {
		t.Fatalf("load verification user: found=%v err=%v", found, err)
	}
	hash, err := store.VerificationCodeHash("123456")
	if err != nil {
		t.Fatalf("hash verification code: %v", err)
	}
	if err := st.BeginContactVerification(context.Background(), user.ID, "email", email, hash, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, confirmed, err := st.ConfirmContactVerification(context.Background(), user.ID, "email", "123456"); err != nil || !confirmed {
		t.Fatalf("confirm test email: confirmed=%v err=%v", confirmed, err)
	}
}

type captureNotificationSender struct {
	messages chan capturedNotification
}

type failingNotificationSender struct{}

func (failingNotificationSender) SendNotification(context.Context, string, string, string) error {
	return errors.New("smtp.internal.example: authentication secret rejected")
}

func (s *captureNotificationSender) SendNotification(_ context.Context, destination, title, body string) error {
	s.messages <- capturedNotification{destination: destination, title: title, body: body}
	return nil
}

func TestNotificationSettingsTestAndAlertLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "notifications.db")
	st, err := store.OpenSQLite(context.Background(), dbPath)
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
	verifyEmailForTest(t, st, "admin", "owner@example.test")

	var defaults struct {
		Settings model.NotificationSettings `json:"settings"`
		Channels map[string]struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"channels"`
	}
	authRequestJSON(t, client, http.MethodGet, srv.URL+"/api/notifications/settings", nil, http.StatusOK, &defaults)
	if defaults.Settings.Configured || defaults.Settings.MemoryThresholdPercent != 85 {
		t.Fatalf("unexpected defaults: %#v", defaults.Settings)
	}
	if defaults.Channels["email"].Status != "disabled" || defaults.Channels["telegram"].Status != "unavailable" {
		t.Fatalf("unexpected channel diagnostics: %#v", defaults.Channels)
	}

	var saved struct {
		Settings model.NotificationSettings `json:"settings"`
	}
	authRequestJSON(t, client, http.MethodPut, srv.URL+"/api/notifications/settings", map[string]any{
		"email_enabled": true, "telegram_enabled": false, "notify_warning": true,
		"notify_critical": true, "notify_resolved": true, "memory_threshold_percent": 80,
		"disk_threshold_percent": 90, "packet_loss_percent": 25, "latency_threshold_ms": 250,
		"repeat_minutes": 15,
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
	waitNotificationDelivery(t, client, srv.URL, "active", "sent")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-16 * time.Minute).Format(time.RFC3339Nano)
	if _, err := db.Exec(`UPDATE notification_deliveries SET created_at = ?, updated_at = ? WHERE event = 'active'`, old, old); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/agent/heartbeat", enrolled.DeviceToken, map[string]any{
		"device_id": enrolled.DeviceID,
		"inventory": map[string]any{"hostname": "notify-router"},
		"metrics":   map[string]any{"memory": map[string]any{"used_kb": 900, "total_kb": 1000}},
	}, http.StatusOK, nil)
	repeatMessage := waitNotification(t, sender.messages)
	if repeatMessage.title == "" {
		t.Fatal("repeat alert notification was empty")
	}
	waitNotificationDelivery(t, client, srv.URL, "repeat", "sent")

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
		Notifications []model.NotificationDelivery      `json:"notifications"`
		Metrics       model.NotificationDeliveryMetrics `json:"metrics"`
	}
	authRequestJSON(t, client, http.MethodGet, srv.URL+"/api/notifications", nil, http.StatusOK, &history)
	if len(history.Notifications) < 4 {
		t.Fatalf("expected test, active, repeat and resolved deliveries, got %#v", history.Notifications)
	}
	if history.Metrics.Sent < 4 {
		t.Fatalf("notification metrics did not include sent deliveries: %#v", history.Metrics)
	}
	for _, delivery := range history.Notifications {
		if delivery.Destination != "" {
			t.Fatal("raw notification destination leaked through API")
		}
	}
	var filtered struct {
		Notifications []model.NotificationDelivery `json:"notifications"`
	}
	authRequestJSON(t, client, http.MethodGet, srv.URL+"/api/notifications?device_id="+enrolled.DeviceID+"&severity=critical&channel=email&status=sent", nil, http.StatusOK, &filtered)
	if len(filtered.Notifications) < 3 {
		t.Fatalf("expected filtered active lifecycle, got %#v", filtered.Notifications)
	}
	for _, delivery := range filtered.Notifications {
		if delivery.DeviceID != enrolled.DeviceID || delivery.Severity != "critical" || delivery.Channel != "email" || delivery.Status != "sent" {
			t.Fatalf("delivery escaped filters: %#v", delivery)
		}
	}
	authRequestJSON(t, client, http.MethodGet, srv.URL+"/api/notifications?channel=carrier-pigeon", nil, http.StatusBadRequest, nil)
}

func waitNotificationDelivery(t *testing.T, client *http.Client, serverURL, event, status string) model.NotificationDelivery {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var history struct {
			Notifications []model.NotificationDelivery `json:"notifications"`
		}
		authRequestJSON(t, client, http.MethodGet, serverURL+"/api/notifications", nil, http.StatusOK, &history)
		for _, delivery := range history.Notifications {
			if delivery.Event == event && delivery.Status == status {
				return delivery
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("notification event %q did not reach status %q", event, status)
	return model.NotificationDelivery{}
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

func TestNotificationFailureIsRetriedWithoutLeakingProviderError(t *testing.T) {
	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "notification-failure.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{
		OperatorUsername: "admin", OperatorPassword: "notification-password-123",
		AlertEmailSender: failingNotificationSender{}, NotificationMaxAttempts: 3,
	}))
	defer srv.Close()
	client := authenticatedClient(t, srv.URL, "admin", "notification-password-123")
	authRequestJSON(t, client, http.MethodPatch, srv.URL+"/api/auth/profile", map[string]any{
		"display_name": "Owner", "email": "owner@example.test",
	}, http.StatusOK, nil)
	verifyEmailForTest(t, st, "admin", "owner@example.test")
	authRequestJSON(t, client, http.MethodPut, srv.URL+"/api/notifications/settings", map[string]any{
		"email_enabled": true, "notify_warning": true, "notify_critical": true, "notify_resolved": true,
		"memory_threshold_percent": 85, "disk_threshold_percent": 85, "packet_loss_percent": 20,
		"latency_threshold_ms": 200, "repeat_minutes": 0,
	}, http.StatusOK, nil)
	var result struct {
		Notifications []model.NotificationDelivery `json:"notifications"`
	}
	authRequestJSON(t, client, http.MethodPost, srv.URL+"/api/notifications/test", nil, http.StatusOK, &result)
	if len(result.Notifications) != 1 {
		t.Fatalf("unexpected test result: %#v", result.Notifications)
	}
	delivery := result.Notifications[0]
	if delivery.Status != "retry" || delivery.AttemptCount != 1 || delivery.MaxAttempts != 3 || delivery.NextAttemptAt == nil {
		t.Fatalf("unexpected retry state: %#v", delivery)
	}
	if strings.Contains(delivery.Error, "smtp.internal") || strings.Contains(delivery.Error, "secret") {
		t.Fatalf("provider error leaked to API: %q", delivery.Error)
	}
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

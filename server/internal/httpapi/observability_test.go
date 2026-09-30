package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rmm-openwrt/server/internal/httpapi"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

func TestReadinessMetricsAndShutdown(t *testing.T) {
	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	handler := httpapi.NewHandler(st, httpapi.Config{OperatorToken: "test-operator-token", OperatorPassword: "correct-horse-battery-staple", BackgroundTasks: true, NotificationWorkerInterval: time.Hour})
	srv := httptest.NewServer(handler)
	defer srv.Close()
	check := func(path, token string, want int) string {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("%s: status %d: %s", path, resp.StatusCode, body)
		}
		return string(body)
	}
	check("/readyz", "", http.StatusOK)
	check("/metrics", "", http.StatusUnauthorized)
	user, err := st.CreateUser(context.Background(), "viewer", "Viewer", "", "test-hash", "user")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateOperatorSession(context.Background(), store.TokenHash("viewer-session"), user.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	restricted, _ := http.NewRequest(http.MethodGet, srv.URL+"/metrics", nil)
	restricted.AddCookie(&http.Cookie{Name: "rmm_operator_session", Value: "viewer-session"})
	denied, err := srv.Client().Do(restricted)
	if err != nil {
		t.Fatal(err)
	}
	denied.Body.Close()
	if denied.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin metrics: %d", denied.StatusCode)
	}
	body := check("/metrics", "test-operator-token", http.StatusOK)
	for _, expected := range []string{`rmm_database_info{backend="sqlite"} 1`, `rmm_commands{status="queued"} 0`, "rmm_tunnel_sessions 0", "rmm_notification_queue_oldest_age_seconds 0"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing metric %s in %s", expected, body)
		}
	}
	// A live SSE request must finish as part of shutdown, without waiting for its client.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/events", nil)
	req.Header.Set("Authorization", "Bearer test-operator-token")
	stream, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	if stream.StatusCode != http.StatusOK {
		t.Fatalf("SSE status %d", stream.StatusCode)
	}
	done := make(chan error, 1)
	go func() { _, err := io.ReadAll(stream.Body); done <- err }()
	lifecycle := handler.(interface{ Shutdown(context.Context) error })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := lifecycle.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("SSE did not stop")
	}
	check("/readyz", "", http.StatusServiceUnavailable)
	check("/healthz", "", http.StatusOK)
	st.Close()
	check("/healthz", "", http.StatusServiceUnavailable)
	check("/metrics", "test-operator-token", http.StatusUnauthorized)
}

type shutdownNotificationSender struct{ entered chan struct{} }

func (s *shutdownNotificationSender) SendNotification(ctx context.Context, _, _, _ string) error {
	close(s.entered)
	<-ctx.Done()
	return ctx.Err()
}

func TestShutdownLeavesInterruptedDeliveryRecoverable(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenSQLite(ctx, filepath.Join(t.TempDir(), "worker.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	user, err := st.EnsureBootstrapUser(ctx, "admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	delivery, found, err := st.CreateNotificationDelivery(ctx, model.NotificationDelivery{UserID: user.ID, Event: "test", Channel: "email", Title: "test", Body: "body", Destination: "test@example.test", MaxAttempts: 3}, "shutdown-delivery")
	if err != nil || !found {
		t.Fatalf("create delivery: %v %v", found, err)
	}
	sender := &shutdownNotificationSender{entered: make(chan struct{})}
	handler := httpapi.NewHandler(st, httpapi.Config{OperatorUsername: "admin", OperatorPassword: "correct-horse-battery-staple", BackgroundTasks: true, AlertEmailSender: sender})
	shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	lifecycle := handler.(interface{ Shutdown(context.Context) error })
	defer lifecycle.Shutdown(shutdownCtx)
	select {
	case <-sender.entered:
	case <-shutdownCtx.Done():
		t.Fatal("delivery did not start")
	}
	if err := lifecycle.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListNotificationDeliveries(ctx, store.NotificationListOptions{UserID: user.ID})
	if err != nil || len(rows) != 1 || rows[0].Status != "sending" {
		t.Fatalf("interrupted delivery was completed: %+v %v", rows, err)
	}
	recovered, found, err := st.ClaimNotificationDelivery(ctx, delivery.ID, time.Now().Add(5*time.Minute), time.Minute)
	if err != nil || !found || recovered.AttemptCount != 2 {
		t.Fatalf("lease not recoverable: %+v %v %v", recovered, found, err)
	}
}

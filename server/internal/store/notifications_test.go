package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"rmm-openwrt/server/internal/model"
)

func TestMigrateLegacyNotificationDeliveries(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy-notifications.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE notification_deliveries (
  id TEXT PRIMARY KEY, user_id TEXT NOT NULL, device_id TEXT, alert_id TEXT NOT NULL DEFAULT '',
  dedupe_key TEXT NOT NULL UNIQUE, event TEXT NOT NULL, channel TEXT NOT NULL, status TEXT NOT NULL,
  title TEXT NOT NULL, body TEXT NOT NULL, destination TEXT NOT NULL, destination_masked TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, sent_at TEXT, updated_at TEXT NOT NULL
);
INSERT INTO notification_deliveries VALUES
  ('ntf_sent', 'usr_legacy', NULL, '', 'sent', 'test', 'email', 'sent', 'sent', 'body', 'a@example.test', 'a***@example.test', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:01Z', '2026-01-01T00:00:01Z'),
  ('ntf_failed', 'usr_legacy', NULL, '', 'failed', 'test', 'email', 'failed', 'failed', 'body', 'a@example.test', 'a***@example.test', 'old error', '2026-01-01T00:00:00Z', NULL, '2026-01-01T00:00:01Z'),
  ('ntf_queued', 'usr_legacy', NULL, '', 'queued', 'test', 'email', 'queued', 'queued', 'body', 'a@example.test', 'a***@example.test', '', '2026-01-01T00:00:00Z', NULL, '2026-01-01T00:00:01Z');
`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	st, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	history, err := st.ListNotificationDeliveries(ctx, NotificationListOptions{UserID: "usr_legacy"})
	if err != nil || len(history) != 3 {
		t.Fatalf("migrated history: %#v err=%v", history, err)
	}
	byID := make(map[string]model.NotificationDelivery, len(history))
	for _, delivery := range history {
		byID[delivery.ID] = delivery
	}
	if byID["ntf_sent"].NextAttemptAt != nil {
		t.Fatalf("sent delivery became retryable: %#v", byID["ntf_sent"])
	}
	if byID["ntf_failed"].Status != "dead_letter" || byID["ntf_failed"].AttemptCount != 1 {
		t.Fatalf("failed delivery was not migrated safely: %#v", byID["ntf_failed"])
	}
	if byID["ntf_queued"].NextAttemptAt == nil {
		t.Fatalf("queued delivery did not become claimable: %#v", byID["ntf_queued"])
	}
}

func TestNotificationQueueSurvivesRestartAndRetries(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "notification-queue.db")
	st, err := OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	user, err := st.EnsureBootstrapUser(ctx, "admin", "test-password-hash")
	if err != nil {
		t.Fatal(err)
	}
	created, inserted, err := st.CreateNotificationDelivery(ctx, model.NotificationDelivery{
		UserID: user.ID, Event: "test", Channel: "email", Title: "test", Body: "body",
		Destination: "owner@example.test", DestinationMasked: "o***@example.test", MaxAttempts: 3,
	}, "restart-retry")
	if err != nil || !inserted {
		t.Fatalf("create delivery: inserted=%v err=%v", inserted, err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	st, err = OpenSQLite(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now().UTC().Add(time.Second)
	claimed, found, err := st.ClaimNotificationDelivery(ctx, created.ID, now, time.Minute)
	if err != nil || !found {
		t.Fatalf("claim after restart: found=%v err=%v", found, err)
	}
	if claimed.Status != "sending" || claimed.AttemptCount != 1 || claimed.MaxAttempts != 3 {
		t.Fatalf("unexpected first claim: %#v", claimed)
	}
	next := now.Add(30 * time.Second)
	if err := st.CompleteNotificationDelivery(ctx, claimed.ID, "retry", "temporary failure", &next); err != nil {
		t.Fatal(err)
	}
	if deliveries, err := st.ClaimNotificationDeliveries(ctx, next.Add(-time.Second), time.Minute, 10); err != nil || len(deliveries) != 0 {
		t.Fatalf("delivery became ready too early: deliveries=%#v err=%v", deliveries, err)
	}
	claimed, found, err = st.ClaimNotificationDelivery(ctx, created.ID, next, time.Minute)
	if err != nil || !found || claimed.AttemptCount != 2 {
		t.Fatalf("claim retry: delivery=%#v found=%v err=%v", claimed, found, err)
	}
	if err := st.CompleteNotificationDelivery(ctx, claimed.ID, "sent", "", nil); err != nil {
		t.Fatal(err)
	}
	history, err := st.ListNotificationDeliveries(ctx, NotificationListOptions{UserID: user.ID})
	if err != nil || len(history) != 1 {
		t.Fatalf("list history: %#v err=%v", history, err)
	}
	if history[0].Status != "sent" || history[0].AttemptCount != 2 || history[0].SentAt == nil {
		t.Fatalf("unexpected completed delivery: %#v", history[0])
	}
}

func TestNotificationQueueDeadLetterLeaseRecoveryAndRetention(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "notification-dead-letter.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	user, err := st.EnsureBootstrapUser(ctx, "admin", "test-password-hash")
	if err != nil {
		t.Fatal(err)
	}
	created, inserted, err := st.CreateNotificationDelivery(ctx, model.NotificationDelivery{
		UserID: user.ID, Event: "active", Channel: "telegram", Title: "test", Body: "body",
		Destination: "12345", DestinationMasked: "***2345", MaxAttempts: 1,
	}, "lease-dead-letter")
	if err != nil || !inserted {
		t.Fatalf("create delivery: inserted=%v err=%v", inserted, err)
	}
	now := time.Now().UTC().Add(time.Second)
	claimed, found, err := st.ClaimNotificationDelivery(ctx, created.ID, now, time.Minute)
	if err != nil || !found || claimed.AttemptCount != 1 {
		t.Fatalf("initial claim: delivery=%#v found=%v err=%v", claimed, found, err)
	}
	if deliveries, err := st.ClaimNotificationDeliveries(ctx, now.Add(2*time.Minute), time.Minute, 10); err != nil || len(deliveries) != 0 {
		t.Fatalf("exhausted lease was reclaimed: deliveries=%#v err=%v", deliveries, err)
	}
	history, err := st.ListNotificationDeliveries(ctx, NotificationListOptions{UserID: user.ID})
	if err != nil || len(history) != 1 || history[0].Status != "dead_letter" {
		t.Fatalf("delivery was not moved to dead letter: %#v err=%v", history, err)
	}
	queued, inserted, err := st.CreateNotificationDelivery(ctx, model.NotificationDelivery{
		UserID: user.ID, Event: "test", Channel: "email", Title: "queued", Body: "body",
		Destination: "owner@example.test", DestinationMasked: "o***@example.test",
	}, "retention-keeps-queued")
	if err != nil || !inserted || queued.Status != "queued" {
		t.Fatalf("create queued delivery: %#v inserted=%v err=%v", queued, inserted, err)
	}
	deleted, err := st.PurgeNotificationDeliveriesBefore(ctx, time.Now().UTC().Add(24*time.Hour))
	if err != nil || deleted != 1 {
		t.Fatalf("purge terminal history: deleted=%d err=%v", deleted, err)
	}
	history, err = st.ListNotificationDeliveries(ctx, NotificationListOptions{UserID: user.ID})
	if err != nil || len(history) != 1 || history[0].ID != queued.ID {
		t.Fatalf("purge removed non-terminal delivery: %#v err=%v", history, err)
	}
}

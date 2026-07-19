package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateLegacyRemoteSessionsTable(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
CREATE TABLE devices (
	id TEXT PRIMARY KEY,
	token TEXT NOT NULL UNIQUE,
	hostname TEXT NOT NULL,
	openwrt_version TEXT NOT NULL,
	inventory_json TEXT NOT NULL DEFAULT '{}',
	metrics_json TEXT NOT NULL DEFAULT '{}',
	last_seen_at TEXT,
	created_at TEXT NOT NULL
);
CREATE TABLE remote_sessions (
	id TEXT PRIMARY KEY,
	device_id TEXT NOT NULL
);
INSERT INTO devices (id, token, hostname, openwrt_version, created_at)
VALUES ('dev_legacy', 'tok_legacy', 'legacy-router', 'OpenWrt', '2026-01-01T00:00:00Z');
INSERT INTO remote_sessions (id, device_id)
VALUES ('rs_legacy', 'dev_legacy');
`)
	if err != nil {
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

	sessions, found, err := st.ListRemoteSessions(ctx, "dev_legacy", RemoteSessionListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("legacy device was not found")
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].Target != "ssh" {
		t.Fatalf("got target %q, want ssh", sessions[0].Target)
	}
	if allowed, err := st.AuthorizeDevice(ctx, "dev_legacy", "tok_legacy"); err != nil || !allowed {
		t.Fatalf("legacy device token did not survive hash migration: allowed=%v err=%v", allowed, err)
	}
	var storedToken, storedHash string
	if err := st.db.QueryRowContext(ctx, `SELECT token, token_hash FROM devices WHERE id = 'dev_legacy'`).Scan(&storedToken, &storedHash); err != nil {
		t.Fatal(err)
	}
	if storedToken == "tok_legacy" || storedHash != TokenHash("tok_legacy") {
		t.Fatalf("legacy token was not replaced safely: token=%q hash=%q", storedToken, storedHash)
	}
}

func TestRedactSensitiveOpenWrtOutput(t *testing.T) {
	input := "wireless.radio0.key='wifi-secret'\n\toption private_key 'private-value'\nmonkey=value\n"
	got := RedactSensitiveOutput(input)
	if got != "wireless.radio0.key='[redacted]'\n\toption private_key '[redacted]'\nmonkey=value\n" {
		t.Fatalf("unexpected redaction:\n%s", got)
	}
}

func TestSQLiteConcurrentAccessDoesNotReturnBusy(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "concurrent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	enrolled, err := st.EnrollDevice(ctx, "router", "OpenWrt")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 80)
	for range 40 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _, err := st.ListRemoteSessions(ctx, enrolled.DeviceID, RemoteSessionListOptions{})
			errs <- err
		}()
		go func() {
			defer wg.Done()
			_, err := st.SaveHeartbeat(ctx, enrolled.DeviceID, []byte(`{"hostname":"router"}`), []byte(`{}`))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

package store

import (
	"context"
	"database/sql"
	"path/filepath"
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
}

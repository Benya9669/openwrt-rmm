package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"rmm-openwrt/server/internal/model"
)

func TestAgentRolloutQueuesSequentialBatchesAndPausesOnFailure(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "rollout.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var devices []model.RolloutDevice
	for i := 0; i < 3; i++ {
		d, err := s.EnrollDevice(ctx, "router", "24.10")
		if err != nil {
			t.Fatal(err)
		}
		devices = append(devices, model.RolloutDevice{DeviceID: d.DeviceID, FeedURL: "https://packages.example.test/feed", PackageManager: "opkg", PackageVersion: "1.2.3-1"})
	}
	r, err := s.CreateAgentRollout(ctx, "stable", "1.2.3", 1, 1, devices)
	if err != nil {
		t.Fatal(err)
	}
	queued := 0
	var first model.RolloutDevice
	for _, device := range r.Devices {
		if device.Status == "queued" {
			queued++
			first = device
		}
	}
	if queued != 1 {
		t.Fatalf("initial batch = %#v", r.Devices)
	}
	if _, err := s.SaveCommandResult(ctx, first.CommandID, first.DeviceID, "completed", 0, "", json.RawMessage(`{"health_status":"waiting_reconnect"}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.HandleAgentRolloutResult(ctx, first.CommandID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	waiting, err := s.GetAgentRollout(ctx, r.ID)
	waitingStatus := ""
	for _, device := range waiting.Devices {
		if device.CommandID == first.CommandID {
			waitingStatus = device.Status
		}
	}
	if err != nil || waitingStatus != "waiting_reconnect" {
		t.Fatalf("successful install must wait for reconnect confirmation: %#v, %v", waiting, err)
	}
	if _, err := s.SaveHeartbeat(ctx, first.DeviceID, json.RawMessage(`{"agent_version":"1.2.3"}`), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	r, err = s.GetAgentRollout(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	queued = 0
	var second model.RolloutDevice
	for _, device := range r.Devices {
		if device.Status == "queued" {
			queued++
			second = device
		}
	}
	if queued != 1 {
		t.Fatalf("second batch was not queued: %#v", r.Devices)
	}
	if _, err := s.SaveCommandResult(ctx, second.CommandID, second.DeviceID, "failed", 1, "update failed", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.HandleAgentRolloutResult(ctx, second.CommandID, "failed", "update failed"); err != nil {
		t.Fatal(err)
	}
	r, err = s.GetAgentRollout(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	pending := 0
	for _, device := range r.Devices {
		if device.Status == "pending" {
			pending++
		}
	}
	if r.Status != "paused" || pending != 1 {
		t.Fatalf("failure must pause without queueing: %#v", r)
	}
	if _, err := s.SaveHeartbeat(ctx, second.DeviceID, json.RawMessage(`{"agent_version":"1.2.3"}`), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	r, err = s.GetAgentRollout(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	queued = 0
	secondStatus := ""
	for _, device := range r.Devices {
		if device.Status == "queued" {
			queued++
		}
		if device.CommandID == second.CommandID {
			secondStatus = device.Status
		}
	}
	if r.Status != "running" || r.FailureCount != 0 || secondStatus != "completed" || queued != 1 {
		t.Fatalf("heartbeat recovery must resume rollout and queue the next batch: %#v", r)
	}
}

func TestAgentRolloutReconnectTimeoutPausesRollout(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "rollout-timeout.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d, err := s.EnrollDevice(ctx, "router", "25.12")
	if err != nil {
		t.Fatal(err)
	}
	rollout, err := s.CreateAgentRollout(ctx, "stable", "0.6.10", 1, 1, []model.RolloutDevice{{DeviceID: d.DeviceID, FeedURL: "https://packages.example.test/feed", PackageManager: "apk", PackageVersion: "0.6.10-r1"}})
	if err != nil {
		t.Fatal(err)
	}
	commandID := rollout.Devices[0].CommandID
	if _, err := s.SaveCommandResult(ctx, commandID, d.DeviceID, "completed", 0, "", json.RawMessage(`{"health_status":"waiting_reconnect"}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.HandleAgentRolloutResult(ctx, commandID, "completed", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE commands SET completed_at = ? WHERE id = ?`, time.Now().UTC().Add(-10*time.Minute).Format(time.RFC3339Nano), commandID); err != nil {
		t.Fatal(err)
	}
	count, err := s.ReconcileAgentRollouts(ctx, 5*time.Minute)
	if err != nil || count != 1 {
		t.Fatalf("reconcile = %d, %v", count, err)
	}
	rollout, err = s.GetAgentRollout(ctx, rollout.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rollout.Status != "paused" || rollout.FailureCount != 1 || rollout.Devices[0].Status != "failed" {
		t.Fatalf("timed-out rollout was not paused: %#v", rollout)
	}
}

func TestHeartbeatRecoversFailedAgentUpdateWhenTargetVersionIsRunning(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "agent-update-recovery.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	device, err := s.EnrollDevice(ctx, "router", "25.12")
	if err != nil {
		t.Fatal(err)
	}
	command, _, err := s.CreateCommand(ctx, device.DeviceID, "agent_update", json.RawMessage(`{"target_version":"0.6.14"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveCommandResult(ctx, command.ID, device.DeviceID, "failed", 1, "package manager was interrupted", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveHeartbeat(ctx, device.DeviceID, json.RawMessage(`{"agent_version":"0.6.14"}`), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	recovered, ok, err := s.GetCommand(ctx, device.DeviceID, command.ID)
	if err != nil || !ok {
		t.Fatalf("GetCommand() = %#v, %v, %v", recovered, ok, err)
	}
	if recovered.Status != "completed" || recovered.ExitCode == nil || *recovered.ExitCode != 0 {
		t.Fatalf("failed update was not reconciled from heartbeat: %#v", recovered)
	}
	var result map[string]any
	if json.Unmarshal(recovered.Result, &result) != nil || result["health_status"] != "healthy" || result["recovered_from_failed_result"] != true {
		t.Fatalf("unexpected reconciled result: %s", recovered.Result)
	}
}

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

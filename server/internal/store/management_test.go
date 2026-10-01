package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"rmm-openwrt/server/internal/model"
)

func managementFixture(t *testing.T) (*Store, model.User, model.User, []string) {
	t.Helper()
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "management.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	admin, err := s.EnsureBootstrapUser(ctx, "admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.CreateUser(ctx, "operator", "Operator", "", "test-hash", "user")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, group := range []string{"branch", "other", "branch", "branch"} {
		d, err := s.EnrollDevice(ctx, group, "24.10")
		if err != nil {
			t.Fatal(err)
		}
		if _, found, err := s.TransferDevice(ctx, d.DeviceID, owner.ID, admin.ID, true); err != nil || !found {
			t.Fatal(err)
		}
		if _, _, err := s.UpdateDeviceFleet(ctx, d.DeviceID, group, nil); err != nil {
			t.Fatal(err)
		}
		if err := s.SaveFleetAsset(ctx, admin.ID, FleetAsset{DeviceID: d.DeviceID, Site: "Lab"}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, d.DeviceID)
	}
	return s, admin, owner, ids
}

func TestPermissionsScopesAndRevocationBeforeDelivery(t *testing.T) {
	s, admin, owner, ids := managementFixture(t)
	ctx := context.Background()
	p := PermissionPolicy{Permissions: []string{"view", "diagnostics"}, Groups: []string{"branch"}, Sites: []string{"Lab"}}
	if err := s.SavePermissionPolicy(ctx, owner.ID, owner.ID, p); !errors.Is(err, ErrFleetAccess) {
		t.Fatalf("operator changed permissions: %v", err)
	}
	if err := s.SavePermissionPolicy(ctx, admin.ID, admin.ID, p); err == nil {
		t.Fatal("administrator lockout accepted")
	}
	if err := s.SavePermissionPolicy(ctx, admin.ID, owner.ID, p); err != nil {
		t.Fatal(err)
	}
	visible, err := s.ListDevicesForUser(ctx, owner.ID, false)
	if err != nil || len(visible) != 3 {
		t.Fatalf("scope: %d %v", len(visible), err)
	}
	if err := s.CheckPermission(ctx, owner.ID, ids[1], "view"); !errors.Is(err, ErrFleetAccess) {
		t.Fatal("out-of-scope device allowed")
	}
	if _, _, err := s.CreateAuthorizedCommand(ctx, owner.ID, ids[0], "uci_commit", json.RawMessage(`{}`)); !errors.Is(err, ErrFleetAccess) {
		t.Fatalf("viewer configuration accepted: %v", err)
	}
	if _, err := s.CreateAuthorizedCommandBatch(ctx, owner.ID, []string{ids[0], ids[1]}, "ping", json.RawMessage(`{}`)); !errors.Is(err, ErrFleetAccess) {
		t.Fatalf("batch accepted excluded target: %v", err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM commands").Scan(&count); err != nil || count != 0 {
		t.Fatal("denied batch left partial commands")
	}
	command, found, err := s.CreateAuthorizedCommand(ctx, owner.ID, ids[0], "ping", json.RawMessage(`{"target":"127.0.0.1"}`))
	if err != nil || !found {
		t.Fatal(err)
	}
	p.Permissions = []string{"view"}
	if err = s.SavePermissionPolicy(ctx, admin.ID, owner.ID, p); err != nil {
		t.Fatal(err)
	}
	if _, found, err = s.ClaimNextCommand(ctx, ids[0]); err != nil || found {
		t.Fatalf("revoked command delivered: %v", err)
	}
	var status string
	if err = s.db.QueryRowContext(ctx, "SELECT status FROM commands WHERE id=?", command.ID).Scan(&status); err != nil || status != "cancelled" {
		t.Fatalf("revocation status %s %v", status, err)
	}
	if err = s.SavePermissionPolicy(ctx, admin.ID, owner.ID, PermissionPolicy{}); err != nil {
		t.Fatal(err)
	}
	empty, err := s.GetPermissionPolicy(ctx, owner.ID)
	if err != nil || empty.Permissions == nil || len(empty.Permissions) != 0 {
		t.Fatal("empty policy must expose an empty permission array")
	}
	if err = s.CheckPermission(ctx, owner.ID, ids[0], "view"); !errors.Is(err, ErrFleetAccess) {
		t.Fatal("empty policy granted view")
	}
	if err = s.CheckPermission(ctx, admin.ID, ids[1], "uci"); err != nil {
		t.Fatal("administrator lost access")
	}
}

func TestIncidentCorrelationAcknowledgementRecoveryAndRecurrence(t *testing.T) {
	s, admin, owner, ids := managementFixture(t)
	ctx := context.Background()
	syncSource := func(key string, active bool) {
		t.Helper()
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		if err = syncIncidentSource(ctx, tx, ids[0], key, "network", "Network incident", active); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	syncSource("rule:dns", true)
	syncSource("rule:wan", true)
	syncSource("rule:wan", true)
	entries, err := s.ListIncidents(ctx, owner.ID)
	if err != nil || len(entries) != 1 || len(entries[0].Events) != 2 {
		t.Fatalf("correlation: %+v %v", entries, err)
	}
	id := entries[0].ID
	for _, step := range []struct{ action, body string }{{"assign", owner.ID}, {"comment", "Checking uplink"}, {"acknowledge", ""}} {
		if err = s.UpdateIncident(ctx, owner.ID, id, step.action, step.body); err != nil {
			t.Fatal(err)
		}
	}
	syncSource("rule:dns", false)
	entries, _ = s.ListIncidents(ctx, owner.ID)
	if entries[0].Status != "acknowledged" {
		t.Fatal("one healthy source resolved other active condition")
	}
	syncSource("rule:wan", false)
	entries, _ = s.ListIncidents(ctx, owner.ID)
	if entries[0].Status != "resolved" || entries[0].ResolvedAt == "" {
		t.Fatal("recovery did not resolve incident")
	}
	syncSource("rule:wan", true)
	entries, _ = s.ListIncidents(ctx, owner.ID)
	if entries[0].Status != "open" || entries[0].Occurrences != 2 || entries[0].AssigneeID != owner.ID {
		t.Fatal("recurrence lost workflow history")
	}
	if err = s.UpdateIncident(ctx, owner.ID, id, "resolve", ""); err != nil {
		t.Fatal(err)
	}
	syncSource("rule:wan", true)
	entries, _ = s.ListIncidents(ctx, owner.ID)
	if entries[0].Status != "resolved" {
		t.Fatal("repeated observation reopened manual resolution")
	}
	if err = s.SavePermissionPolicy(ctx, admin.ID, owner.ID, PermissionPolicy{Permissions: []string{"view"}}); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateIncident(ctx, owner.ID, id, "comment", "not permitted"); !errors.Is(err, ErrFleetAccess) {
		t.Fatal("comment bypassed permissions")
	}
}

func TestGuardedRolloutWaitsThenPausesOnLostContact(t *testing.T) {
	s, _, _, ids := managementFixture(t)
	ctx := context.Background()
	targets := []model.RolloutDevice{}
	for _, id := range ids {
		targets = append(targets, model.RolloutDevice{DeviceID: id, FeedURL: "https://example.test/feed/", PackageManager: "opkg", PackageVersion: "1.0.0", ManifestURL: "https://example.test/manifest.json", SignatureURL: "https://example.test/manifest.sig"})
	}
	r, err := s.CreateGuardedAgentRollout(ctx, "stable", "1.0.0", 2, targets, model.RolloutGuard{RequestKey: "wave-idempotency-test", CanaryIDs: []string{ids[3]}, WaveSizes: []int{2, 3}, ObservationSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := s.CreateGuardedAgentRollout(ctx, "stable", "1.0.0", 2, targets, model.RolloutGuard{RequestKey: "wave-idempotency-test", CanaryIDs: []string{ids[3]}, WaveSizes: []int{2, 3}, ObservationSeconds: 60})
	if err != nil || repeated.ID != r.ID {
		t.Fatalf("rollout retry: %+v %v", repeated, err)
	}
	if _, err = s.CreateGuardedAgentRollout(ctx, "stable", "1.0.0", 2, targets, model.RolloutGuard{RequestKey: "wave-idempotency-test", CanaryIDs: []string{ids[3]}, WaveSizes: []int{3}, ObservationSeconds: 60}); !errors.Is(err, ErrFleetConflict) {
		t.Fatalf("changed request key accepted: %v", err)
	}
	firstCommand := ""
	for _, d := range r.Devices {
		if d.Status == "queued" {
			if d.DeviceID != ids[3] || firstCommand != "" {
				t.Fatal("initial wave ignored explicit canary")
			}
			firstCommand = d.CommandID
		}
	}
	if firstCommand == "" {
		t.Fatal("canary not queued")
	}
	if _, err = s.SetAgentRolloutStatus(ctx, r.ID, "paused"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := s.ClaimNextCommand(ctx, ids[3]); err != nil || found {
		t.Fatalf("paused guard delivered unclaimed command: %v", err)
	}
	if _, err = s.SetAgentRolloutStatus(ctx, r.ID, "running"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveCommandResult(ctx, firstCommand, ids[3], "completed", 0, "installed", nil); err != nil {
		t.Fatal(err)
	}
	if err = s.HandleAgentRolloutResult(ctx, firstCommand, "completed", "installed"); err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceGuardedRollouts(ctx); err != nil {
		t.Fatal(err)
	}
	r, _ = s.GetAgentRollout(ctx, r.ID)
	for _, d := range r.Devices {
		if d.DeviceID != ids[3] && d.CommandID != "" {
			t.Fatal("next wave queued before reconnect")
		}
	}
	if _, err = s.SaveHeartbeat(ctx, ids[3], json.RawMessage(`{"agent_version":"1.0.0"}`), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceGuardedRollouts(ctx); err != nil {
		t.Fatal(err)
	}
	r, _ = s.GetAgentRollout(ctx, r.ID)
	if r.Guard.HealthySince == "" {
		t.Fatal("observation did not start")
	}
	if _, err = s.db.ExecContext(ctx, "UPDATE management_rollout_guards SET healthy_since=? WHERE rollout_id=?", time.Now().Add(-61*time.Second).UTC().Format(time.RFC3339Nano), r.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceGuardedRollouts(ctx); err != nil {
		t.Fatal(err)
	}
	r, _ = s.GetAgentRollout(ctx, r.ID)
	wave2 := 0
	for _, d := range r.Devices {
		if d.Batch == 2 {
			wave2++
		}
	}
	if wave2 != 2 {
		t.Fatalf("expected two targets in second wave: %d", wave2)
	}
	for _, d := range r.Devices {
		if d.Batch == 2 {
			if _, err = s.db.ExecContext(ctx, "UPDATE agent_rollout_devices SET status='completed' WHERE rollout_id=? AND device_id=?", r.ID, d.DeviceID); err != nil {
				t.Fatal(err)
			}
			if _, err = s.db.ExecContext(ctx, "UPDATE devices SET inventory_json=?,last_seen_at=? WHERE id=?", `{"agent_version":"1.0.0"}`, time.Now().Add(-3*time.Minute).UTC().Format(time.RFC3339Nano), d.DeviceID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err = s.AdvanceGuardedRollouts(ctx); err != nil {
		t.Fatal(err)
	}
	r, _ = s.GetAgentRollout(ctx, r.ID)
	if r.Status != "paused" || r.Guard.PauseReason == "" {
		t.Fatal("lost contact did not pause rollout")
	}
	for _, d := range r.Devices {
		if d.Batch > 2 {
			t.Fatal("third wave queued after loss of contact")
		}
	}
}

func TestPermissionEditClosesRemoteSessionsAndGuardsLegacyAuditCommands(t *testing.T) {
	s, admin, owner, ids := managementFixture(t)
	ctx := context.Background()
	session, found, err := s.CreateRemoteSession(ctx, model.RemoteSession{RequesterUserID: owner.ID, DeviceID: ids[0], RemotePort: 22010, LuCIPort: 22110, ExpiresAt: time.Now().Add(time.Minute)})
	if err != nil || !found {
		t.Fatal(err)
	}
	startup, found, err := s.CreateManagedRemoteCommand(ctx, owner.ID, ids[0], session.ID, json.RawMessage(`{}`))
	if err != nil || !found {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, "UPDATE remote_sessions SET status='active' WHERE id=?", session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.ExecContext(ctx, "UPDATE commands SET status='completed' WHERE id=?", startup.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.CreateDeviceAccessGrant(ctx, TokenHash("synthetic-access-grant"), owner.ID, ids[0], session.ID, time.Now().Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	old, found, err := s.CreateCommand(ctx, ids[0], "uci_commit", json.RawMessage(`{}`))
	if err != nil || !found {
		t.Fatal(err)
	}
	if _, err = s.AddAuditEvent(ctx, owner.Username, "command.create", ids[0], old.ID, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err = s.SavePermissionPolicy(ctx, admin.ID, owner.ID, PermissionPolicy{Permissions: []string{"view"}}); err != nil {
		t.Fatal(err)
	}
	session, found, err = s.GetRemoteSession(ctx, ids[0], session.ID)
	if err != nil || !found || session.Status != "closed" {
		t.Fatal("permission change left user's remote session open")
	}
	var count int
	if err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM device_access_grants WHERE user_id=?", owner.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("permission change left LuCI grant valid")
	}
	if _, found, err = s.ClaimNextCommand(ctx, ids[0]); err != nil || found {
		t.Fatalf("legacy audited command bypassed revoked permission: %v", err)
	}
	var status string
	if err = s.db.QueryRowContext(ctx, "SELECT status FROM commands WHERE id=?", old.ID).Scan(&status); err != nil || status != "cancelled" {
		t.Fatal("legacy command not cancelled")
	}
}

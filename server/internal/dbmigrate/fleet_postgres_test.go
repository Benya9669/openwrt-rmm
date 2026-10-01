package dbmigrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"rmm-openwrt/internal/fleetprofile"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
	"sync"
	"testing"
	"time"
)

func testFleetPostgres(t *testing.T, ctx context.Context, a, b *store.Store, db *sql.DB, userID string) {
	t.Helper()
	ids := []string{}
	for i := 0; i < 4; i++ {
		device, err := a.EnrollDevice(ctx, "fleet-router", "24.10")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, device.DeviceID)
	}
	input := store.FleetOperationInput{RequestKey: "postgres-fleet-request", Title: "Concurrent fleet", DeviceIDs: ids, Type: "ping", Args: json.RawMessage(`{"target":"127.0.0.1"}`), Parallelism: 2}
	operation, err := a.CreateFleetOperation(ctx, userID, input)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			st := a
			if i%2 != 0 {
				st = b
			}
			if err := st.AdvanceFleetOperations(ctx); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	operations, err := b.ListFleetOperations(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	active := 0
	for _, op := range operations {
		if op.ID == operation.ID {
			for _, item := range op.Items {
				if item.Status == "queued" {
					active++
				}
			}
		}
	}
	if active != 2 {
		t.Fatalf("independent pools exceeded parallelism: %d", active)
	}
	if err = b.SetFleetOperationAction(ctx, userID, operation.ID, "cancel"); err != nil {
		t.Fatal(err)
	}
	if err = a.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	asset := store.FleetAsset{DeviceID: ids[0], Model: "Test model", Site: "Lab"}
	if err = a.SaveFleetAsset(ctx, userID, asset); err != nil {
		t.Fatal(err)
	}
	assets, err := b.ListFleetAssets(ctx, userID, "Test model", "Lab")
	if err != nil || len(assets) != 1 {
		t.Fatalf("PostgreSQL asset: %v %v", assets, err)
	}
	profile, err := a.SaveFleetProfile(ctx, userID, store.FleetProfile{Title: "Test system", Definition: fleetprofile.Profile{Config: "system", Options: []fleetprofile.Option{{Section: "core", Option: "hostname", Value: "lab"}}}})
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := b.ListFleetProfiles(ctx, userID)
	if err != nil || len(profiles) != 1 || profiles[0].ID != profile.ID {
		t.Fatalf("PostgreSQL profile: %v %v", profiles, err)
	}
	scheduleInput := store.FleetSchedule{RequestKey: "postgres-schedule-retry", Title: "Test schedule", Timezone: "Europe/Moscow", Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, MinuteOfDay: 60, WindowMinutes: 30, Enabled: true, Operation: store.FleetOperationInput{DeviceIDs: ids, Type: "ping", Args: json.RawMessage(`{"target":"127.0.0.1"}`), Parallelism: 1}}
	schedule, err := a.SaveFleetSchedule(ctx, userID, scheduleInput)
	if err != nil {
		t.Fatal(err)
	}
	// Make one isolated test reservation due; no production scheduler is involved.
	repeatedSchedule, err := b.SaveFleetSchedule(ctx, userID, scheduleInput)
	if err != nil || repeatedSchedule.ID != schedule.ID || repeatedSchedule.NextRunAt != schedule.NextRunAt {
		t.Fatalf("PostgreSQL schedule retry: %+v %v", repeatedSchedule, err)
	}
	if _, err = db.ExecContext(ctx, "UPDATE fleet_schedules SET next_run_at=$1 WHERE id=$2", time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), schedule.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			st := a
			if i%2 != 0 {
				st = b
			}
			if err := st.AdvanceFleetSchedules(ctx); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	runs, err := a.ListFleetScheduleRuns(ctx, userID)
	if err != nil || len(runs) != 1 || runs[0]["operation_id"] == "" {
		t.Fatalf("PostgreSQL reservation duplicated: %v %v", runs, err)
	}
	policy := store.FleetAccessPolicy{DeviceID: ids[0], SSHAllowed: true, LuCIAllowed: false, MaxTTLSeconds: 60}
	if err = a.SaveFleetAccessPolicy(ctx, userID, policy); err != nil {
		t.Fatal(err)
	}
	loaded, err := b.GetFleetAccessPolicy(ctx, userID, ids[0])
	if err != nil || loaded.LuCIAllowed || loaded.MaxTTLSeconds != 60 {
		t.Fatalf("PostgreSQL policy: %v %v", loaded, err)
	}
	ruleInput := store.FleetRule{RequestKey: "postgres-rule-retry", Title: "Test rule", DeviceIDs: []string{ids[0]}, Kind: "memory_high", Threshold: 80, Consecutive: 2, CooldownSeconds: 300, MaxActionsPerDay: 1, Action: "notify", Enabled: true}
	rule, err := a.SaveFleetRule(ctx, userID, ruleInput)
	if err != nil {
		t.Fatal(err)
	}
	repeatedRule, err := b.SaveFleetRule(ctx, userID, ruleInput)
	if err != nil || repeatedRule.ID != rule.ID {
		t.Fatalf("PostgreSQL rule retry: %+v %v", repeatedRule, err)
	}
	rules, err := b.ListFleetRules(ctx, userID)
	if err != nil || len(rules) != 1 || rules[0].ID != rule.ID {
		t.Fatalf("PostgreSQL rule: %v %v", rules, err)
	}
	for i := 0; i < 2; i++ {
		if _, err = a.SaveHeartbeat(ctx, ids[0], json.RawMessage(`{}`), json.RawMessage(`{"memory":{"total_kb":100,"used_kb":90}}`)); err != nil {
			t.Fatal(err)
		}
		if err = b.AdvanceFleetRules(ctx); err != nil {
			t.Fatal(err)
		}
	}
	events, err := a.ListFleetRuleEvents(ctx, userID)
	if err != nil || len(events) != 1 {
		t.Fatalf("PostgreSQL reaction: %v %v", events, err)
	}
	owner, err := a.CreateUser(ctx, "management-owner", "Management owner", "", "test-hash", "user")
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := a.TransferDevice(ctx, ids[2], owner.ID, userID, true); err != nil || !found {
		t.Fatal(err)
	}
	if err = a.SavePermissionPolicy(ctx, userID, owner.ID, store.PermissionPolicy{Permissions: []string{"view", "diagnostics", "incidents"}}); err != nil {
		t.Fatal(err)
	}
	queued, found, err := a.CreateAuthorizedCommand(ctx, owner.ID, ids[2], "ping", json.RawMessage(`{"target":"127.0.0.1"}`))
	if err != nil || !found {
		t.Fatal(err)
	}
	if err = b.SavePermissionPolicy(ctx, userID, owner.ID, store.PermissionPolicy{Permissions: []string{"view", "incidents"}}); err != nil {
		t.Fatal(err)
	}
	if _, found, err = b.ClaimNextCommand(ctx, ids[2]); err != nil || found {
		t.Fatalf("PostgreSQL permission revocation: %v", err)
	}
	var commandStatus string
	if err = db.QueryRowContext(ctx, "SELECT status FROM commands WHERE id=$1", queued.ID).Scan(&commandStatus); err != nil || commandStatus != "cancelled" {
		t.Fatal("PostgreSQL permission guard did not cancel command")
	}
	if _, _, err = a.SyncDeviceAlerts(ctx, ids[2], []model.Alert{{ID: "synthetic-management-alert", Type: "offline", Message: "Synthetic management incident"}}); err != nil {
		t.Fatal(err)
	}
	incidents, err := b.ListIncidents(ctx, owner.ID)
	if err != nil || len(incidents) != 1 {
		t.Fatalf("PostgreSQL incidents: %+v %v", incidents, err)
	}
	if err = b.UpdateIncident(ctx, owner.ID, incidents[0].ID, "comment", "Synthetic comment"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = a.SyncDeviceAlerts(ctx, ids[2], nil); err != nil {
		t.Fatal(err)
	}
	incidents, err = b.ListIncidents(ctx, owner.ID)
	if err != nil || incidents[0].Status != "resolved" {
		t.Fatal("PostgreSQL incident did not recover")
	}
	targets := []model.RolloutDevice{{DeviceID: ids[1], FeedURL: "https://example.test/feed", PackageManager: "opkg", PackageVersion: "1.0.0", ManifestURL: "https://example.test/manifest.json", SignatureURL: "https://example.test/manifest.sig"}, {DeviceID: ids[3], FeedURL: "https://example.test/feed", PackageManager: "opkg", PackageVersion: "1.0.0", ManifestURL: "https://example.test/manifest.json", SignatureURL: "https://example.test/manifest.sig"}}
	guard := model.RolloutGuard{RequestKey: "postgres-management-waves", CanaryIDs: []string{ids[3]}, WaveSizes: []int{1}, ObservationSeconds: 60}
	rollout, err := a.CreateGuardedAgentRollout(ctx, "stable", "1.0.0", 1, targets, guard)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := b.CreateGuardedAgentRollout(ctx, "stable", "1.0.0", 1, targets, guard)
	if err != nil || repeated.ID != rollout.ID {
		t.Fatalf("PostgreSQL rollout retry: %v", err)
	}
	guard.WaveSizes = []int{2}
	if _, err = b.CreateGuardedAgentRollout(ctx, "stable", "1.0.0", 1, targets, guard); !errors.Is(err, store.ErrFleetConflict) {
		t.Fatal("PostgreSQL rollout allowed changed retry")
	}
	for _, device := range rollout.Devices {
		if device.DeviceID != ids[3] && device.CommandID != "" {
			t.Fatal("PostgreSQL wave ignored canary")
		}
	}

}

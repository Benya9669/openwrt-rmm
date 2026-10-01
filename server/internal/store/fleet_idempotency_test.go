package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestFleetDefinitionRetriesDoNotCreateDuplicateAutomation(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "dedupe.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	user, err := s.EnsureBootstrapUser(ctx, "dedupe-admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	device, err := s.EnrollDevice(ctx, "test", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	input := FleetSchedule{RequestKey: "schedule-network-retry", Title: "Backup", Timezone: "UTC", Weekdays: []int{1}, MinuteOfDay: 180, WindowMinutes: 60, Operation: FleetOperationInput{DeviceIDs: []string{device.DeviceID}, Type: "system_backup_create", Parallelism: 1}}
	first, err := s.SaveFleetSchedule(ctx, user.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := s.SaveFleetSchedule(ctx, user.ID, input)
	if err != nil || first.ID != repeated.ID || first.NextRunAt != repeated.NextRunAt {
		t.Fatalf("duplicate schedule: %+v %v", repeated, err)
	}
	input.Enabled = true
	if _, err = s.SaveFleetSchedule(ctx, user.ID, input); !errors.Is(err, ErrFleetConflict) {
		t.Fatalf("retry changed schedule: %v", err)
	}
	rule := FleetRule{RequestKey: "rule-network-retry", Title: "Notify", DeviceIDs: []string{device.DeviceID}, Kind: "offline", Consecutive: 2, CooldownSeconds: 300, MaxActionsPerDay: 1, Action: "notify"}
	saved, err := s.SaveFleetRule(ctx, user.ID, rule)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.SaveFleetRule(ctx, user.ID, rule)
	if err != nil || saved.ID != again.ID {
		t.Fatalf("duplicate rule: %+v %v", again, err)
	}
	rule.Enabled = true
	if _, err = s.SaveFleetRule(ctx, user.ID, rule); !errors.Is(err, ErrFleetConflict) {
		t.Fatalf("retry changed rule: %v", err)
	}
	schedules, err := s.ListFleetSchedules(ctx, user.ID)
	if err != nil || len(schedules) != 1 {
		t.Fatal("duplicate definitions persisted")
	}
	rules, err := s.ListFleetRules(ctx, user.ID)
	if err != nil || len(rules) != 1 {
		t.Fatal("duplicate rules persisted")
	}
}

package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFleetCalendarDSTAndValidation(t *testing.T) {
	days := []int{0, 1, 2, 3, 4, 5, 6}
	after, _ := time.Parse(time.RFC3339, "2026-03-29T00:00:00Z")
	slot, err := nextFleetSlot(after, "Europe/Berlin", days, 150)
	if err != nil || slot.Format(time.RFC3339) != "2026-03-30T00:30:00Z" {
		t.Fatalf("nonexistent time: %s %v", slot, err)
	}
	after, _ = time.Parse(time.RFC3339, "2026-10-24T23:00:00Z")
	slot, err = nextFleetSlot(after, "Europe/Berlin", days, 150)
	if err != nil {
		t.Fatal(err)
	}
	next, err := nextFleetSlot(slot, "Europe/Berlin", days, 150)
	if err != nil || next.In(slot.Location()).Sub(slot) < 23*time.Hour {
		t.Fatalf("repeated calendar slot: %s %s %v", slot, next, err)
	}
	for _, zone := range []string{"unknown/timezone", ""} {
		if _, err := nextFleetSlot(after, zone, nil, 150); err == nil {
			t.Fatal("invalid calendar accepted")
		}
	}
}

func TestFleetScheduleRestartDedupMissedAndWindow(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "schedules.db")
	s, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	user, err := s.EnsureBootstrapUser(ctx, "schedule-admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	device, err := s.EnrollDevice(ctx, "schedule-router", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	schedule, err := s.SaveFleetSchedule(ctx, user.ID, FleetSchedule{Title: "Daily diagnostics", Timezone: "UTC", Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, MinuteOfDay: 60, WindowMinutes: 10, Enabled: true, Operation: FleetOperationInput{DeviceIDs: []string{device.DeviceID}, Type: "ping", Args: json.RawMessage(`{"target":"127.0.0.1"}`), Parallelism: 1}})
	if err != nil {
		t.Fatal(err)
	}
	slot := time.Now().UTC().Add(-time.Minute)
	if _, err = s.db.ExecContext(ctx, "UPDATE fleet_schedules SET next_run_at=? WHERE id=?", slot.Format(time.RFC3339Nano), schedule.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.reserveFleetSchedules(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	var runID, frozen string
	if err = s.db.QueryRowContext(ctx, "SELECT id,operation_json FROM fleet_schedule_runs WHERE schedule_id=?", schedule.ID).Scan(&runID, &frozen); err != nil {
		t.Fatal(err)
	}
	var input FleetOperationInput
	if err = json.Unmarshal([]byte(frozen), &input); err != nil {
		t.Fatal(err)
	}
	// Crash after creating an operation but before the worker acknowledges its run.
	op, err := s.CreateFleetOperation(ctx, user.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.AdvanceFleetSchedules(ctx); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	operations, err := s.ListFleetOperations(ctx, user.ID)
	if err != nil || len(operations) != 1 || operations[0].ID != op.ID {
		t.Fatalf("duplicate after crash: %v %v", operations, err)
	}
	runs, err := s.ListFleetScheduleRuns(ctx, user.ID)
	if err != nil || len(runs) != 1 || runs[0]["status"] != "queued" {
		t.Fatalf("run acknowledgement: %v %v", runs, err)
	}
	if _, err = s.db.ExecContext(ctx, "UPDATE fleet_schedules SET next_run_at=? WHERE id=?", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano), schedule.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceFleetSchedules(ctx); err != nil {
		t.Fatal(err)
	}
	var missed int
	if err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fleet_schedule_runs WHERE status='missed'").Scan(&missed); err != nil || missed != 1 {
		t.Fatalf("missed maintenance window: %d %v", missed, err)
	}
	if _, err = s.db.ExecContext(ctx, "UPDATE fleet_operations SET deadline_at=? WHERE id=?", time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano), op.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	operations, err = s.ListFleetOperations(ctx, user.ID)
	if err != nil || operations[0].Status != "failed" || operations[0].Items[0].CommandID != "" {
		t.Fatalf("dispatch outside window: %v %v", operations, err)
	}
}

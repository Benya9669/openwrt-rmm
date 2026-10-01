package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"rmm-openwrt/internal/commandsig"
)

func TestFleetOperationDurableConcurrencyAndCancellation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fleet.db")
	key, err := commandsig.LoadOrCreatePrivateKey(filepath.Join(dir, "signing.pem"))
	if err != nil {
		t.Fatal(err)
	}
	open := func() *Store {
		t.Helper()
		s, err := OpenSQLite(ctx, dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetCommandSigningKey(key); err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := open()
	user, err := s.EnsureBootstrapUser(ctx, "operator", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := 0; i < 3; i++ {
		d, err := s.EnrollDevice(ctx, "test-router", "24.10")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, d.DeviceID)
	}
	input := FleetOperationInput{RequestKey: "durable-test-request", Title: "Ping selected", DeviceIDs: ids, Type: "ping", Args: json.RawMessage(`{"target":"127.0.0.1"}`), Parallelism: 1}
	op, err := s.CreateFleetOperation(ctx, user.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := s.CreateFleetOperation(ctx, user.ID, input)
	if err != nil || repeated.ID != op.ID {
		t.Fatalf("duplicate request: %v", err)
	}
	changed := input
	changed.Parallelism = 2
	if _, err := s.CreateFleetOperation(ctx, user.ID, changed); !errors.Is(err, ErrFleetConflict) {
		t.Fatalf("request key reuse accepted: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.AdvanceFleetOperations(ctx); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	operations, err := s.ListFleetOperations(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	var commandID, deviceID string
	queued := 0
	for _, item := range operations[0].Items {
		if item.Status == "queued" {
			queued++
			commandID = item.CommandID
			deviceID = item.DeviceID
		}
	}
	if queued != 1 {
		t.Fatalf("parallelism exceeded: %d", queued)
	}
	claimed, found, err := s.ClaimNextCommand(ctx, deviceID)
	if err != nil || !found || claimed.ID != commandID {
		t.Fatalf("claim: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = open()
	defer s.Close()
	if err := s.SetFleetOperationAction(ctx, user.ID, op.ID, "cancel"); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	operations, err = s.ListFleetOperations(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if operations[0].Status != "cancelling" {
		t.Fatalf("claimed command was treated as stopped: %s", operations[0].Status)
	}
	if _, err := s.SaveCommandResult(ctx, commandID, deviceID, "completed", 0, "test passed", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	operations, err = s.ListFleetOperations(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if operations[0].Status != "cancelled" {
		t.Fatalf("cancel did not finish: %s", operations[0].Status)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM commands").Scan(&count); err != nil || count != 1 {
		t.Fatalf("pending work executed after cancellation: %d %v", count, err)
	}
}

func TestFleetOperationAccessIsAtomicAndRechecked(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "fleet.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	admin, err := s.EnsureBootstrapUser(ctx, "admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.CreateUser(ctx, "owner", "Owner", "", "test-hash", "user")
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.EnrollDevice(ctx, "a", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.EnrollDevice(ctx, "b", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := s.TransferDevice(ctx, a.DeviceID, owner.ID, admin.ID, true); err != nil || !found {
		t.Fatal(err)
	}
	input := FleetOperationInput{RequestKey: "ownership-test-request", DeviceIDs: []string{a.DeviceID, b.DeviceID}, Type: "ping", Parallelism: 1}
	if _, err := s.CreateFleetOperation(ctx, owner.ID, input); !errors.Is(err, ErrFleetAccess) {
		t.Fatalf("foreign target accepted: %v", err)
	}
	operations, err := s.ListFleetOperations(ctx, owner.ID)
	if err != nil || len(operations) != 0 {
		t.Fatal("unauthorized request created partial work")
	}
	input.DeviceIDs = []string{a.DeviceID}
	if _, err := s.CreateFleetOperation(ctx, owner.ID, input); err != nil {
		t.Fatal(err)
	}
	if _, found, err := s.TransferDevice(ctx, a.DeviceID, admin.ID, owner.ID, false); err != nil || !found {
		t.Fatal(err)
	}
	if err := s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	operations, err = s.ListFleetOperations(ctx, owner.ID)
	if err != nil || operations[0].Status != "failed" {
		t.Fatalf("transfer did not stop pending work: %v", err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM commands").Scan(&count); err != nil || count != 0 {
		t.Fatalf("queued after loss of ownership: %d %v", count, err)
	}
}

func TestFleetOperationAccessRecheckedBeforeDelivery(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "fleet.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	admin, err := s.EnsureBootstrapUser(ctx, "admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.CreateUser(ctx, "owner", "Owner", "", "test-hash", "user")
	if err != nil {
		t.Fatal(err)
	}
	device, err := s.EnrollDevice(ctx, "offline-router", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := s.TransferDevice(ctx, device.DeviceID, owner.ID, admin.ID, true); err != nil || !found {
		t.Fatalf("initial transfer: %v", err)
	}
	_, err = s.CreateFleetOperation(ctx, owner.ID, FleetOperationInput{RequestKey: "recheck-before-delivery", DeviceIDs: []string{device.DeviceID}, Type: "ping", Parallelism: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	var commandID string
	if err := s.db.QueryRowContext(ctx, "SELECT id FROM commands WHERE device_id=? AND status='queued'", device.DeviceID).Scan(&commandID); err != nil {
		t.Fatal(err)
	}
	if _, found, err := s.TransferDevice(ctx, device.DeviceID, admin.ID, owner.ID, false); err != nil || !found {
		t.Fatalf("subsequent transfer: %v", err)
	}
	if _, found, err := s.ClaimNextCommand(ctx, device.DeviceID); err != nil || found {
		t.Fatalf("command delivered after access revoked: found=%v error=%v", found, err)
	}
	var status string
	if err := s.db.QueryRowContext(ctx, "SELECT status FROM commands WHERE id=?", commandID).Scan(&status); err != nil || status != "cancelled" {
		t.Fatalf("revoked command: status=%s error=%v", status, err)
	}
}

func TestFleetOperationFailurePausesBeforeNextTarget(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "fleet.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	user, err := s.EnsureBootstrapUser(ctx, "admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.EnrollDevice(ctx, "a", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.EnrollDevice(ctx, "b", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	input := FleetOperationInput{RequestKey: "pause-on-failure", DeviceIDs: []string{a.DeviceID, b.DeviceID}, Type: "ping", Parallelism: 1, StopOnFailure: true}
	op, err := s.CreateFleetOperation(ctx, user.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	operations, err := s.ListFleetOperations(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range operations[0].Items {
		if item.CommandID != "" {
			if _, err := s.SaveCommandResult(ctx, item.CommandID, item.DeviceID, "failed", 1, "diagnostic failure", nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	operations, err = s.ListFleetOperations(ctx, user.ID)
	if err != nil || operations[0].Status != "paused" {
		t.Fatalf("failure did not pause: %v", err)
	}
	if err := s.SetFleetOperationAction(ctx, user.ID, op.ID, "resume"); err != nil {
		t.Fatal(err)
	}
	if err := s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM commands").Scan(&count); err != nil || count != 2 {
		t.Fatalf("explicit resume did not queue next target: %d %v", count, err)
	}
}

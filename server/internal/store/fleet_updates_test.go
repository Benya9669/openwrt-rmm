package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestFleetUpdateWaitsForReconnectAndSkipsInstalledVersion(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "updates.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	user, err := s.EnsureBootstrapUser(ctx, "updates-admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	device, err := s.EnrollDevice(ctx, "update-router", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	inventory := map[string]string{"agent_runtime": "go", "agent_package": "rmm-agent-go-production", "agent_version": "0.9.0", "openwrt_release": "24.10.3", "target": "x86-64", "package_manager": "opkg"}
	heartbeat := func(version string) {
		t.Helper()
		inventory["agent_version"] = version
		raw, _ := json.Marshal(inventory)
		if _, err = s.SaveHeartbeat(ctx, device.DeviceID, raw, nil); err != nil {
			t.Fatal(err)
		}
	}
	heartbeat("0.9.0")
	args := map[string]string{"target_version": "0.9.1", "package_version": "0.9.1-1", "package": "rmm-agent-go-production", "package_manager": "opkg", "expected_release": "24.10.3", "expected_target": "x86-64", "feed_url": "https://updates.example.test/releases/v1/", "manifest_url": "https://updates.example.test/releases/v1/manifest.json", "signature_url": "https://updates.example.test/releases/v1/manifest.sig"}
	raw, _ := json.Marshal(FleetUpdateArgs{Targets: map[string]map[string]string{device.DeviceID: args}})
	input := FleetOperationInput{RequestKey: "scheduled-update-test", Title: "Update", DeviceIDs: []string{device.DeviceID}, Type: "agent_update", Args: raw, Parallelism: 1, StopOnFailure: true}
	operation, err := s.CreateFleetOperation(ctx, user.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	advance := func() FleetOperation {
		t.Helper()
		if err = s.AdvanceFleetOperations(ctx); err != nil {
			t.Fatal(err)
		}
		ops, err := s.ListFleetOperations(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, op := range ops {
			if op.ID == operation.ID {
				return op
			}
		}
		t.Fatal("operation missing")
		return FleetOperation{}
	}
	op := advance()
	commandID := op.Items[0].CommandID
	if commandID == "" {
		t.Fatal("update not queued")
	}
	if _, err = s.SaveCommandResult(ctx, commandID, device.DeviceID, "completed", 0, "package installed", nil); err != nil {
		t.Fatal(err)
	}
	op = advance()
	if op.Status != "running" || op.Items[0].Status != "claimed" {
		t.Fatal("package result treated as healthy reconnect")
	}
	heartbeat("0.9.1")
	op = advance()
	if op.Status != "completed" {
		t.Fatalf("reconnect not recognized: %+v", op)
	}
	input.RequestKey = "already-installed-test"
	operation, err = s.CreateFleetOperation(ctx, user.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	op = advance()
	if op.Status != "completed" || op.Items[0].CommandID != "" {
		t.Fatalf("installed version not skipped: %+v", op)
	}
	args["manifest_url"] = "http://updates.example.test/manifest.json"
	unsafe, _ := json.Marshal(FleetUpdateArgs{Targets: map[string]map[string]string{device.DeviceID: args}})
	if _, err = fleetUpdateDeviceArgs(unsafe, input.DeviceIDs); err == nil {
		t.Fatal("unsigned transport accepted")
	}
}

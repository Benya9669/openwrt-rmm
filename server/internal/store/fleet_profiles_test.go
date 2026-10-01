package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"rmm-openwrt/internal/fleetprofile"
	"testing"
)

func TestProfileCanaryAndPreviewRequirement(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "profiles.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	user, err := s.EnsureBootstrapUser(ctx, "profiles-admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := s.SaveFleetProfile(ctx, user.ID, FleetProfile{Title: "Hostnames", Definition: fleetprofile.Profile{Config: "system", Options: []fleetprofile.Option{{Section: "core", Option: "hostname", Value: "branch"}}}})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for i := 0; i < 3; i++ {
		device, err := s.EnrollDevice(ctx, "test", "24.10")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, device.DeviceID)
		if _, err = s.SaveHeartbeat(ctx, device.DeviceID, json.RawMessage(`{"rmm_features":["uci_profile_preview","uci_profile_apply"]}`), nil); err != nil {
			t.Fatal(err)
		}
	}
	input := FleetProfileOperation{ProfileID: profile.ID, DeviceIDs: ids, RequestKey: "profile-apply-request", Action: "apply", Parallelism: 3}
	if _, err = s.CreateFleetProfileOperation(ctx, user.ID, input); err == nil {
		t.Fatal("apply without preview accepted")
	}
	preview, err := s.CreateFleetProfileOperation(ctx, user.ID, FleetProfileOperation{ProfileID: profile.ID, DeviceIDs: ids, RequestKey: "profile-preview-request", Action: "preview", Parallelism: 3})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	ops, err := s.ListFleetOperations(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		if op.ID == preview.ID {
			for _, item := range op.Items {
				if _, err = s.SaveCommandResult(ctx, item.CommandID, item.DeviceID, "completed", 0, `{"changes":[]}`, nil); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	input.PreviewOperationID = preview.ID
	apply, err := s.CreateFleetProfileOperation(ctx, user.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	ops, err = s.ListFleetOperations(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	queued := 0
	for _, op := range ops {
		if op.ID == apply.ID {
			for _, item := range op.Items {
				if item.Status == "queued" {
					queued++
					if _, err = s.SaveCommandResult(ctx, item.CommandID, item.DeviceID, "completed", 0, "confirmed", nil); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
	if queued != 1 {
		t.Fatalf("canary dispatched %d devices", queued)
	}
	if err = s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	ops, err = s.ListFleetOperations(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		if op.ID == apply.ID && (op.Status != "paused" || op.CanaryPending) {
			t.Fatalf("canary did not require approval: %+v", op)
		}
	}
	if err = s.SetFleetOperationAction(ctx, user.ID, apply.ID, "resume"); err != nil {
		t.Fatal(err)
	}
	if err = s.AdvanceFleetOperations(ctx); err != nil {
		t.Fatal(err)
	}
	changed := input
	changed.Parallelism = 2
	if _, err = s.CreateFleetProfileOperation(ctx, user.ID, changed); !errors.Is(err, ErrFleetConflict) {
		t.Fatalf("mutated idempotent apply accepted: %v", err)
	}
}

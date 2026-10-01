package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"rmm-openwrt/server/internal/model"
	"testing"
	"time"
)

func TestFleetAccessPolicyEnforcementAndRevocation(t *testing.T) {
	ctx := context.Background()
	s, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "policy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	admin, err := s.EnsureBootstrapUser(ctx, "admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	user, err := s.CreateUser(ctx, "owner", "Owner", "", "test-hash", "user")
	if err != nil {
		t.Fatal(err)
	}
	device, err := s.EnrollDevice(ctx, "policy-router", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	policy := FleetAccessPolicy{DeviceID: device.DeviceID, SSHAllowed: true, LuCIAllowed: false, MaxTTLSeconds: 120, RestrictUsers: true, UserIDs: []string{admin.ID}}
	if err = s.SaveFleetAccessPolicy(ctx, user.ID, policy); !errors.Is(err, ErrFleetAccess) {
		t.Fatalf("non-admin policy change: %v", err)
	}
	if err = s.SaveFleetAccessPolicy(ctx, admin.ID, policy); err != nil {
		t.Fatal(err)
	}
	base := model.RemoteSession{RequesterUserID: admin.ID, DeviceID: device.DeviceID, RemotePort: 22010, ExpiresAt: time.Now().Add(time.Minute)}
	luci := base
	luci.LuCIPort = 22110
	if _, _, err = s.CreateRemoteSession(ctx, luci); !errors.Is(err, ErrFleetAccess) {
		t.Fatalf("forbidden LuCI listener: %v", err)
	}
	tooLong := base
	tooLong.ExpiresAt = time.Now().Add(5 * time.Minute)
	if _, _, err = s.CreateRemoteSession(ctx, tooLong); !errors.Is(err, ErrFleetAccess) {
		t.Fatalf("TTL ignored: %v", err)
	}
	anonymous := base
	anonymous.RequesterUserID = ""
	if _, _, err = s.CreateRemoteSession(ctx, anonymous); !errors.Is(err, ErrFleetAccess) {
		t.Fatalf("identity bypass: %v", err)
	}
	session, found, err := s.CreateRemoteSession(ctx, base)
	if err != nil || !found {
		t.Fatalf("allowed SSH session: %v", err)
	}
	command, found, err := s.CreateManagedRemoteCommand(ctx, admin.ID, device.DeviceID, session.ID, json.RawMessage(`{}`))
	if err != nil || !found {
		t.Fatalf("queue managed session: %v", err)
	}
	if command.ExpiresAt.After(session.ExpiresAt.Add(time.Second)) {
		t.Fatal("command outlives access session")
	}
	if err = s.CreateDeviceAccessGrant(ctx, "synthetic-grant", admin.ID, device.DeviceID, session.ID, time.Now().Add(time.Minute)); !errors.Is(err, ErrFleetAccess) {
		t.Fatalf("LuCI grant bypass: %v", err)
	}
	policy.SSHAllowed = false
	if err = s.SaveFleetAccessPolicy(ctx, admin.ID, policy); err != nil {
		t.Fatal(err)
	}
	stored, _, err := s.GetRemoteSession(ctx, device.DeviceID, session.ID)
	if err != nil || stored.Status != "closed" {
		t.Fatalf("session not revoked: %v %v", stored, err)
	}
	var state string
	if err = s.db.QueryRowContext(ctx, "SELECT status FROM commands WHERE id=?", command.ID).Scan(&state); err != nil || state != "cancelled" {
		t.Fatalf("queued tunnel survived revoke: %s %v", state, err)
	}
	ports, err := s.ActiveTunnelPorts(ctx, time.Now())
	if err != nil || len(ports) != 0 {
		t.Fatalf("listener remains authorized: %v %v", ports, err)
	}
	if _, _, err = s.CreateManagedRemoteCommand(ctx, admin.ID, device.DeviceID, session.ID, json.RawMessage(`{}`)); !errors.Is(err, ErrFleetAccess) {
		t.Fatalf("startup race bypass: %v", err)
	}
}

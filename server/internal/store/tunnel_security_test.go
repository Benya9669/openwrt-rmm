package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"rmm-openwrt/server/internal/model"
)

func TestTunnelCredentialEpochAndPortAuthorization(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "tunnel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	device, err := st.EnrollDevice(ctx, "router", "OpenWrt")
	if err != nil {
		t.Fatal(err)
	}

	const publicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKU3gL7P9G8HXhHhux6VxWnM1xgztI0ePjMUL0X29Yz1"
	const fingerprint = "SHA256:test-device-key"
	if expected, updated, err := st.SyncTunnelCredential(ctx, device.DeviceID, 2, publicKey, fingerprint); err != nil || expected != 1 || updated {
		t.Fatalf("mismatched epoch accepted: expected=%d updated=%v err=%v", expected, updated, err)
	}
	if expected, updated, err := st.SyncTunnelCredential(ctx, device.DeviceID, 1, publicKey, fingerprint); err != nil || expected != 1 || !updated {
		t.Fatalf("credential was not registered: expected=%d updated=%v err=%v", expected, updated, err)
	}
	if ready, err := st.TunnelCredentialReady(ctx, device.DeviceID); err != nil || !ready {
		t.Fatalf("credential ready=%v err=%v", ready, err)
	}

	session, found, err := st.CreateRemoteSession(ctx, model.RemoteSession{
		DeviceID: device.DeviceID, Status: "requested", RemotePort: 22040, LuCIPort: 22140,
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	})
	if err != nil || !found {
		t.Fatalf("create remote session: found=%v err=%v", found, err)
	}
	auth, found, err := st.TunnelAuthorization(ctx, fingerprint, time.Now().UTC())
	if err != nil || !found || auth.DeviceID != device.DeviceID || len(auth.Ports) != 2 || auth.Ports[0] != 22040 || auth.Ports[1] != 22140 {
		t.Fatalf("unexpected tunnel authorization: found=%v auth=%#v err=%v", found, auth, err)
	}
	if _, _, err := st.CreateRemoteSession(ctx, model.RemoteSession{
		DeviceID: device.DeviceID, Status: "requested", RemotePort: session.RemotePort, LuCIPort: 22141,
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}); !errors.Is(err, ErrTunnelPortUnavailable) {
		t.Fatalf("duplicate tunnel port error = %v", err)
	}
}

func TestTunnelCredentialRevokedOnDeviceTransfer(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "transfer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	passwordHash := "$argon2id$not-used-in-store-test"
	owner, err := st.CreateUser(ctx, "owner", "", "", passwordHash, "user")
	if err != nil {
		t.Fatal(err)
	}
	target, err := st.CreateUser(ctx, "target", "", "", passwordHash, "user")
	if err != nil {
		t.Fatal(err)
	}
	grant, err := st.CreateEnrollmentGrant(ctx, owner.ID, "router", TokenHash("grant"), time.Now().UTC().Add(time.Hour))
	if err != nil || grant.ID == "" {
		t.Fatal(err)
	}
	device, found, err := st.EnrollDeviceWithGrant(ctx, TokenHash("grant"), "router", "OpenWrt")
	if err != nil || !found {
		t.Fatalf("enroll: found=%v err=%v", found, err)
	}
	if _, updated, err := st.SyncTunnelCredential(ctx, device.DeviceID, 1, "ssh-ed25519 AAAAtest", "SHA256:old"); err != nil || !updated {
		t.Fatalf("register old credential: updated=%v err=%v", updated, err)
	}
	if _, transferred, err := st.TransferDevice(ctx, device.DeviceID, target.ID, owner.ID, false); err != nil || !transferred {
		t.Fatalf("transfer: transferred=%v err=%v", transferred, err)
	}
	if ready, err := st.TunnelCredentialReady(ctx, device.DeviceID); err != nil || ready {
		t.Fatalf("old credential remained ready=%v err=%v", ready, err)
	}
	if epoch, err := st.TunnelKeyEpoch(ctx, device.DeviceID); err != nil || epoch != 2 {
		t.Fatalf("tunnel key epoch=%d err=%v", epoch, err)
	}
	if _, found, err := st.TunnelAuthorization(ctx, "SHA256:old", time.Now().UTC()); err != nil || found {
		t.Fatalf("revoked credential found=%v err=%v", found, err)
	}
}

func TestTunnelSessionConcurrencyAndRateLimits(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "limits.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	device, err := st.EnrollDevice(ctx, "router", "OpenWrt")
	if err != nil {
		t.Fatal(err)
	}
	create := func(n int) (model.RemoteSession, error) {
		session, _, err := st.CreateRemoteSession(ctx, model.RemoteSession{
			DeviceID: device.DeviceID, Status: "requested", RemotePort: 22000 + n, LuCIPort: 22100 + n,
			ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		})
		return session, err
	}
	first, err := create(1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := create(2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := create(3); !errors.Is(err, ErrTunnelSessionLimit) {
		t.Fatalf("third concurrent session error=%v", err)
	}
	if _, _, err := st.CloseRemoteSession(ctx, device.DeviceID, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.CloseRemoteSession(ctx, device.DeviceID, second.ID); err != nil {
		t.Fatal(err)
	}
	for n := 3; n <= 10; n++ {
		session, err := create(n)
		if err != nil {
			t.Fatalf("create session %d: %v", n, err)
		}
		if _, _, err := st.CloseRemoteSession(ctx, device.DeviceID, session.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := create(11); !errors.Is(err, ErrTunnelRateLimit) {
		t.Fatalf("eleventh recent session error=%v", err)
	}
}

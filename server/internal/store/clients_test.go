package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestLANClientPresencePersistsLastSeen(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "clients.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	device, err := st.EnrollDevice(ctx, "router", "OpenWrt")
	if err != nil {
		t.Fatal(err)
	}
	inventory, _ := json.Marshal(map[string]any{
		"dhcp_leases":   []map[string]string{{"mac": "10:ff:e0:21:bc:b9", "ip": "10.10.10.2", "hostname": "desktop"}},
		"client_probes": []map[string]string{{"mac": "10:ff:e0:21:bc:b9", "ip": "10.10.10.2", "reachable": "true"}},
	})
	if _, err := st.SaveHeartbeat(ctx, device.DeviceID, inventory, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	clients, found, err := st.ListLANClients(ctx, device.DeviceID, 30*time.Minute)
	if err != nil || !found || len(clients) != 1 {
		t.Fatalf("unexpected clients: found=%v clients=%#v err=%v", found, clients, err)
	}
	if clients[0].Status != "online" || clients[0].LastSeenAt == nil || clients[0].Confirmation != "active_probe" {
		t.Fatalf("client was not confirmed: %#v", clients[0])
	}
	inventory, _ = json.Marshal(map[string]any{
		"dhcp_leases":   []map[string]string{{"mac": "10:ff:e0:21:bc:b9", "ip": "10.10.10.2", "hostname": "desktop"}},
		"client_probes": []map[string]string{{"mac": "10:ff:e0:21:bc:b9", "ip": "10.10.10.2", "reachable": "false"}},
	})
	if _, err := st.SaveHeartbeat(ctx, device.DeviceID, inventory, json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	clients, _, err = st.ListLANClients(ctx, device.DeviceID, 30*time.Minute)
	if err != nil || clients[0].LastSeenAt == nil {
		t.Fatalf("last_seen was lost: %#v err=%v", clients, err)
	}
}

func TestLANClientSyncIgnoresUnconfirmedNeighborNoise(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "clients.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	device, err := st.EnrollDevice(ctx, "router", "OpenWrt")
	if err != nil {
		t.Fatal(err)
	}
	checkedAt := time.Now().UTC()
	inventory, _ := json.Marshal(map[string]any{
		"neighbors": []map[string]string{
			{"ip": "10.10.10.104", "interface": "br-lan", "state": "FAILED"},
			{"ip": "fe80::1234", "mac": "aa:bb:cc:dd:ee:ff", "interface": "br-lan", "state": "STALE"},
		},
	})
	if err := st.SyncLANClients(ctx, device.DeviceID, inventory, checkedAt); err != nil {
		t.Fatal(err)
	}
	clients, found, err := st.ListLANClients(ctx, device.DeviceID, 30*time.Minute)
	if err != nil || !found {
		t.Fatalf("list clients: found=%v err=%v", found, err)
	}
	if len(clients) != 0 {
		t.Fatalf("unconfirmed kernel neighbours became clients: %#v", clients)
	}
}

func TestLANClientSyncKeepsLeaseAndPrefersIPv4(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "clients.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	device, err := st.EnrollDevice(ctx, "router", "OpenWrt")
	if err != nil {
		t.Fatal(err)
	}
	inventory, _ := json.Marshal(map[string]any{
		"dhcp_leases": []map[string]string{
			{"mac": "aa:bb:cc:dd:ee:ff", "ip": "10.10.10.2", "hostname": "desktop"},
		},
		"neighbors": []map[string]string{
			{"ip": "fe80::1234", "mac": "aa:bb:cc:dd:ee:ff", "interface": "br-lan", "state": "STALE"},
		},
	})
	if err := st.SyncLANClients(ctx, device.DeviceID, inventory, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	clients, _, err := st.ListLANClients(ctx, device.DeviceID, 30*time.Minute)
	if err != nil || len(clients) != 1 {
		t.Fatalf("unexpected clients: %#v err=%v", clients, err)
	}
	if clients[0].IP != "10.10.10.2" || clients[0].Status != "unconfirmed" || clients[0].Confirmation != "lease" {
		t.Fatalf("lease was not preserved correctly: %#v", clients[0])
	}
}

func TestLANClientSyncRemovesLegacyUnconfirmedNeighborRows(t *testing.T) {
	ctx := context.Background()
	st, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "clients.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	device, err := st.EnrollDevice(ctx, "router", "OpenWrt")
	if err != nil {
		t.Fatal(err)
	}
	now := notificationTimeText(time.Now().UTC())
	if _, err := st.db.ExecContext(ctx, `
INSERT INTO lan_clients (
  device_id, client_key, mac, ip, hostname, interface, connection, confirmation,
  first_seen_at, last_seen_at, last_checked_at
) VALUES (?, ?, '', ?, '', 'br-lan', '', '', ?, NULL, ?)
`, device.DeviceID, "ip:10.10.10.104", "10.10.10.104", now, now); err != nil {
		t.Fatal(err)
	}
	if err := st.SyncLANClients(ctx, device.DeviceID, json.RawMessage(`{}`), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	clients, _, err := st.ListLANClients(ctx, device.DeviceID, 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 0 {
		t.Fatalf("legacy unconfirmed neighbour row was not removed: %#v", clients)
	}
}

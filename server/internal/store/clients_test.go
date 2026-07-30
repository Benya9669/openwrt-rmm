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

package httpapi

import (
	"encoding/json"
	"rmm-openwrt/server/internal/model"
	"testing"
	"time"
)

func TestTopologyObservationsAndAmbiguousAddresses(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-3 * time.Minute)
	devices := []model.Device{
		{ID: "a", Hostname: "A", LastSeenAt: &now, Inventory: json.RawMessage(`{"interfaces":[{"name":"br-lan","address":"192.168.1.1/24"}],"neighbors":[{"ip":"192.168.1.2","interface":"br-lan","mac":"02:00:00:00:00:02","state":"REACHABLE"}]}`)},
		{ID: "b", Hostname: "B", LastSeenAt: &old, Inventory: json.RawMessage(`{"interfaces":[{"name":"eth0","address":"192.168.1.2/24"}]}`)},
	}
	result := deriveTopology(devices, now)
	if len(result.Links) != 1 || result.Links[0].To != "b" || result.Nodes[1].Stale != true {
		t.Fatalf("observed map: %+v", result)
	}
	devices = append(devices, model.Device{ID: "c", Hostname: "C", Inventory: devices[1].Inventory})
	result = deriveTopology(devices, now)
	if !result.Links[0].Ambiguous || result.Links[0].To == "b" || result.Links[0].To == "c" {
		t.Fatal("duplicate IP was presented as a verified router link")
	}
	// Only visible devices are passed into derivation. Removing a foreign router
	// must produce an anonymous observed neighbor rather than its private identity.
	result = deriveTopology(devices[:1], now)
	if result.Links[0].To == "b" || len(result.Nodes) != 2 || result.Nodes[1].DeviceID != "" {
		t.Fatal("hidden router identity exposed")
	}
}

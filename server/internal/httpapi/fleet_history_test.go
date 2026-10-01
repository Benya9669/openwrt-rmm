package httpapi

import (
	"encoding/json"
	"rmm-openwrt/server/internal/model"
	"testing"
	"time"
)

func TestFleetHistoryDoesNotInventMeasurementsAndDetectsEvents(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	empty := deriveFleetHistory(nil, now)
	if empty.ObservedAvailability != nil || len(empty.Points) != 0 {
		t.Fatal("availability invented without samples")
	}
	sample := func(at time.Time, ip, uptime string, rx int) model.MetricSample {
		inventory, _ := json.Marshal(map[string]string{"wan_ip": ip, "default_route": "default via 192.0.2.1 dev eth0"})
		metrics, _ := json.Marshal(map[string]any{"loadavg": "1.5 0.5 0.2", "uptime": uptime, "memory": map[string]int{"total_kb": 100, "used_kb": 25}, "interface_counters": []map[string]any{{"name": "eth0", "rx_bytes": rx, "tx_bytes": rx * 2}}})
		return model.MetricSample{CreatedAt: at, Inventory: inventory, Metrics: metrics}
	}
	history := deriveFleetHistory([]model.MetricSample{sample(now.Add(-time.Minute), "192.0.2.3", "20 0", 100), sample(now.Add(-5*time.Minute), "192.0.2.2", "500 0", 1500), sample(now.Add(-6*time.Minute), "192.0.2.2", "440 0", 900)}, now)
	if len(history.Points) != 3 || history.Points[0].WANRXBPS != nil || history.Points[1].WANRXBPS == nil || *history.Points[1].WANRXBPS != 80 || history.Points[2].WANRXBPS != nil {
		t.Fatalf("counter derivation: %+v", history.Points)
	}
	events := map[string]bool{}
	for _, event := range history.Events {
		events[event.Type] = true
	}
	for _, kind := range []string{"reboot_observed", "wan_ip_changed", "telemetry_gap", "heartbeat_restored"} {
		if !events[kind] {
			t.Fatalf("missing %s", kind)
		}
	}
	if history.ObservedAvailability == nil || *history.ObservedAvailability < 66 || *history.ObservedAvailability > 67 {
		t.Fatalf("gap availability: %+v", history.ObservedAvailability)
	}
	if finiteFloat("NaN") != nil || finiteFloat("-1") != nil || finiteFloat("0") == nil {
		t.Fatal("unsafe numeric sample accepted")
	}
}

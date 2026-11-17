package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	"rmm-openwrt/server/internal/model"
)

func TestHTTPSConnectivityAlertsRequireTwoFailedCycles(t *testing.T) {
	settings := model.NotificationSettings{PacketLossPercent: 20, LatencyThresholdMS: 100}
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"first cycle", `{"connectivity_method":"https","connectivity_failed_cycles":1,"connectivity_checks":[{"reachable":false},{"reachable":false}]}`, ""},
		{"both down", `{"connectivity_method":"https","connectivity_failed_cycles":2,"connectivity_checks":[{"reachable":false},{"reachable":false}]}`, "wan_down"},
		{"one healthy", `{"connectivity_method":"https","connectivity_failed_cycles":0,"connectivity_checks":[{"reachable":false},{"reachable":true,"response_time_ms":1100}]}`, ""},
		{"legacy ICMP", `{"connectivity_checks":[{"reachable":true,"packet_loss_percent":33.3,"latency_ms":5}]}`, "packet_loss_high"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			alerts := connectivityAlerts("device", json.RawMessage(tc.raw), settings, time.Now())
			if tc.want == "" {
				if len(alerts) != 0 {
					t.Fatalf("unexpected alerts: %+v", alerts)
				}
				return
			}
			if len(alerts) != 1 || alerts[0].Type != tc.want {
				t.Fatalf("got %+v, want only %q", alerts, tc.want)
			}
		})
	}
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAgentVersionIsStable(t *testing.T) {
	if agentVersion != "0.6.0" {
		t.Fatalf("unexpected agent version %q", agentVersion)
	}
}

func TestLoadConfigDirectDNS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rmm-agent.conf")
	data := `SERVER_URL="https://rmm.example.test"
DIRECT_DNS_ENABLED="1"
DNS_UPDATE_INTERVAL_SECONDS="600"
PUBLIC_IPV4_URL="https://ipv4.example.test"
PUBLIC_IPV6_URL=""
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DirectDNSEnabled || cfg.DNSUpdateSeconds != 600 || cfg.PublicIPv4URL != "https://ipv4.example.test" || cfg.PublicIPv6URL != "" {
		t.Fatalf("unexpected direct DNS config: %#v", cfg)
	}
}

func TestUpdateDirectDNS(t *testing.T) {
	updates := make(chan map[string]string, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v4":
			_, _ = w.Write([]byte("8.8.8.8\n"))
		case "/v6":
			_, _ = w.Write([]byte("2606:4700:4700::1111\n"))
		case "/api/agent/dns/update":
			if r.Header.Get("Authorization") != "Bearer device-secret" {
				t.Errorf("unexpected authorization header: %q", r.Header.Get("Authorization"))
			}
			var request map[string]string
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode update: %v", err)
			}
			updates <- request
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"changed":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cfg := config{
		ServerURL:     server.URL,
		DeviceID:      "dev_test",
		DeviceToken:   "device-secret",
		PublicIPv4URL: server.URL + "/v4",
		PublicIPv6URL: server.URL + "/v6",
	}
	if err := updateDirectDNS(context.Background(), server.Client(), cfg); err != nil {
		t.Fatal(err)
	}
	request := <-updates
	if request["device_id"] != "dev_test" || request["ipv4"] != "8.8.8.8" || request["ipv6"] != "2606:4700:4700::1111" {
		t.Fatalf("unexpected DNS update: %#v", request)
	}
}

func TestPublicIPAddressValidation(t *testing.T) {
	if _, err := normalizePublicAddress("192.168.1.1", true); err == nil {
		t.Fatal("private IPv4 address was accepted")
	}
	if _, err := normalizePublicAddress("2001:db8::1", false); err == nil {
		t.Fatal("documentation IPv6 address was accepted")
	}
	if got, err := normalizePublicAddress("8.8.8.8", true); err != nil || got != "8.8.8.8" {
		t.Fatalf("public IPv4 address rejected: got=%q err=%v", got, err)
	}
	if _, err := fetchPublicIPAddress(context.Background(), &http.Client{}, "http://example.test", true); err == nil {
		t.Fatal("insecure discovery URL was accepted")
	}
}

func TestDirectDNSUpdateSchedule(t *testing.T) {
	now := time.Unix(1000, 0)
	state := directDNSUpdateState{}
	if !state.due(now) {
		t.Fatal("initial direct DNS update is not due")
	}
	state.schedule(now, 5*time.Minute, nil)
	if state.due(now.Add(4*time.Minute)) || !state.due(now.Add(5*time.Minute)) {
		t.Fatal("successful direct DNS interval was not respected")
	}
	state.schedule(now, 5*time.Minute, errors.New("temporary failure"))
	if state.due(now.Add(59*time.Second)) || !state.due(now.Add(time.Minute)) {
		t.Fatal("failed direct DNS update retry interval was not respected")
	}
}

func TestAgentRuntimeHealthSnapshot(t *testing.T) {
	spoolDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(spoolDir, "pending.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	health := &agentRuntimeHealth{StartedAt: time.Unix(100, 0).UTC()}
	health.recordFailure(errors.New("temporary connection failure"))
	snapshot := health.snapshot(spoolDir)
	if snapshot["consecutive_failures"] != 1 || snapshot["pending_results"] != 1 {
		t.Fatalf("unexpected health snapshot: %#v", snapshot)
	}
	if snapshot["last_heartbeat_error"] != "temporary connection failure" {
		t.Fatalf("unexpected heartbeat error: %#v", snapshot)
	}
	health.recordSuccess()
	if health.ConsecutiveFailures != 0 || health.LastHeartbeatSuccess.IsZero() {
		t.Fatalf("recordSuccess did not reset runtime state: %#v", health)
	}
	health.recordDirectDNSUpdate(errors.New("discovery unavailable"))
	if health.snapshot(spoolDir)["last_dns_update_error"] != "discovery unavailable" {
		t.Fatalf("unexpected direct DNS health: %#v", health.snapshot(spoolDir))
	}
	health.recordDirectDNSUpdate(nil)
	if health.LastDNSUpdateSuccess.IsZero() || health.LastDNSUpdateError != "" {
		t.Fatalf("direct DNS recovery was not recorded: %#v", health)
	}
}

func TestAcquireLockRemovesDirectoryOnUnlock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "rmm-agent.lock")
	unlock, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("acquireLock() error: %v", err)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock directory was not created: %v", err)
	}
	if _, err := acquireLock(lockPath); err == nil {
		t.Fatal("second acquireLock() unexpectedly succeeded")
	}

	unlock()
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock directory still exists after unlock: %v", err)
	}
}

func TestParseIwinfoAssocList(t *testing.T) {
	output := `AA:BB:CC:DD:EE:FF  -49 dBm / -95 dBm (SNR 46)  210 ms ago
	RX: 72.2 MBit/s, MCS 7, 20MHz
	TX: 135.0 MBit/s, MCS 6, 40MHz

11:22:33:44:55:66  -62 dBm / -95 dBm (SNR 33)  30 ms ago
	RX: 6.0 MBit/s
	TX: 54.0 MBit/s`

	clients := parseIwinfoAssocList("phy0-ap0", output)
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(clients))
	}
	if clients[0]["interface"] != "phy0-ap0" {
		t.Fatalf("unexpected interface: %q", clients[0]["interface"])
	}
	if clients[0]["mac"] != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("unexpected mac: %q", clients[0]["mac"])
	}
	if clients[0]["signal_dbm"] != "-49" {
		t.Fatalf("unexpected signal: %q", clients[0]["signal_dbm"])
	}
	if clients[0]["rx_rate"] == "" || clients[0]["tx_rate"] == "" {
		t.Fatalf("expected RX/TX rates, got %#v", clients[0])
	}
	if clients[1]["mac"] != "11:22:33:44:55:66" {
		t.Fatalf("unexpected second mac: %q", clients[1]["mac"])
	}
}

func TestLooksLikeMAC(t *testing.T) {
	valid := []string{"aa:bb:cc:dd:ee:ff", "AA:BB:CC:DD:EE:FF", "11:22:33:44:55:66,"}
	for _, value := range valid {
		if !looksLikeMAC(value) {
			t.Fatalf("expected %q to be a MAC", value)
		}
	}

	invalid := []string{"", "aa:bb:cc:dd:ee", "aa-bb-cc-dd-ee-ff", "not-a-mac-address"}
	for _, value := range invalid {
		if looksLikeMAC(value) {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}

func TestParseIPNeighbors(t *testing.T) {
	output := `10.10.10.2 dev br-lan lladdr 10:ff:e0:21:bc:b9 REACHABLE
10.10.10.190 dev br-lan FAILED
fe80::1234 dev br-lan lladdr aa:bb:cc:dd:ee:ff router STALE`

	neighbors := parseIPNeighbors(output)
	if len(neighbors) != 3 {
		t.Fatalf("expected 3 neighbors, got %d", len(neighbors))
	}
	if neighbors[0]["mac"] != "10:FF:E0:21:BC:B9" || neighbors[0]["state"] != "REACHABLE" {
		t.Fatalf("unexpected reachable neighbor: %#v", neighbors[0])
	}
	if neighbors[1]["ip"] != "10.10.10.190" || neighbors[1]["state"] != "FAILED" {
		t.Fatalf("unexpected failed neighbor: %#v", neighbors[1])
	}
	if neighbors[2]["interface"] != "br-lan" || neighbors[2]["state"] != "STALE" {
		t.Fatalf("unexpected stale neighbor: %#v", neighbors[2])
	}
}

func TestParsePacketLossBusyBox(t *testing.T) {
	output := `PING 10.10.10.10 (10.10.10.10): 56 data bytes
64 bytes from 10.10.10.10: seq=0 ttl=64 time=1.157 ms
64 bytes from 10.10.10.10: seq=1 ttl=64 time=0.883 ms
64 bytes from 10.10.10.10: seq=2 ttl=64 time=0.952 ms

--- 10.10.10.10 ping statistics ---
3 packets transmitted, 3 packets received, 0% packet loss
round-trip min/avg/max = 0.883/0.997/1.157 ms`

	if loss := parsePacketLoss(output); loss != 0 {
		t.Fatalf("expected 0%% packet loss, got %v", loss)
	}
	if latency := parseLatency(output); latency != 0.997 {
		t.Fatalf("expected avg latency 0.997, got %v", latency)
	}
}

func TestParsePacketLossGNU(t *testing.T) {
	output := `--- 1.1.1.1 ping statistics ---
3 packets transmitted, 2 received, 33.3333% packet loss, time 2002ms
rtt min/avg/max/mdev = 10.100/11.200/12.300/0.100 ms`

	if loss := parsePacketLoss(output); loss != 33.3333 {
		t.Fatalf("expected 33.3333%% packet loss, got %v", loss)
	}
	if latency := parseLatency(output); latency != 11.2 {
		t.Fatalf("expected avg latency 11.2, got %v", latency)
	}
}

func TestServerCheckTarget(t *testing.T) {
	tests := map[string]string{
		"http://10.10.10.10:18082":   "10.10.10.10",
		"https://rmm.daemonlord.ru":  "rmm.daemonlord.ru",
		"http://[2001:db8::1]:18082": "2001:db8::1",
		"http://127.0.0.1:8080/path": "127.0.0.1",
		"not a valid url":            "",
	}
	for input, want := range tests {
		if got := serverCheckTarget(input); got != want {
			t.Fatalf("serverCheckTarget(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEffectiveCheckTargetsAddsServerTarget(t *testing.T) {
	got := effectiveCheckTargets([]string{"1.1.1.1", "10.10.10.10", "1.1.1.1"}, "10.10.10.10")
	if len(got) != 2 || got[0] != "1.1.1.1" || got[1] != "10.10.10.10" {
		t.Fatalf("unexpected targets: %#v", got)
	}

	got = effectiveCheckTargets([]string{"1.1.1.1"}, "10.10.10.10")
	if len(got) != 2 || got[1] != "10.10.10.10" {
		t.Fatalf("expected server target to be appended, got %#v", got)
	}
}

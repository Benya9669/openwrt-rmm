package main

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestAgentVersionIsStable(t *testing.T) {
	if agentVersion != "0.6.8" {
		t.Fatalf("unexpected agent version %q", agentVersion)
	}
}

func TestParseLocalInterfaceIPv4CandidatesPrefersLAN(t *testing.T) {
	output := `1: lo    inet 127.0.0.1/8 scope host lo
2: eth0    inet 203.0.113.10/24 brd 203.0.113.255 scope global eth0
5: br-lan    inet 10.10.10.1/24 brd 10.10.10.255 scope global br-lan
6: guest    inet 192.168.50.1/24 brd 192.168.50.255 scope global guest`
	got := parseLocalInterfaceIPv4Candidates(output)
	want := []string{"10.10.10.1", "203.0.113.10", "192.168.50.1"}
	if len(got) != len(want) {
		t.Fatalf("unexpected candidates: %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidate %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSelectRemoteSSHLocalHostFallsBackFromLoopbackToLAN(t *testing.T) {
	reachable := func(host string, port int) bool {
		return host == "10.10.10.1" && port == 22
	}
	got, err := selectRemoteSSHLocalHost("127.0.0.1", 22, []string{"10.10.10.1", "192.168.50.1"}, reachable)
	if err != nil {
		t.Fatalf("selectRemoteSSHLocalHost() error: %v", err)
	}
	if got != "10.10.10.1" {
		t.Fatalf("selected host %q, want LAN address", got)
	}
}

func TestSelectRemoteSSHLocalHostDoesNotReplaceExplicitHost(t *testing.T) {
	reachable := func(host string, port int) bool {
		return host == "10.10.10.1"
	}
	if _, err := selectRemoteSSHLocalHost("192.0.2.10", 22, []string{"10.10.10.1"}, reachable); err == nil {
		t.Fatal("explicit unreachable host unexpectedly fell back to another interface")
	}
}

func TestRemoteSSHTargetReachableRequiresSSHBanner(t *testing.T) {
	host, port := startBannerServer(t, "SSH-2.0-dropbear_2025.88\r\n")
	if !remoteSSHTargetReachable(host, port) {
		t.Fatal("SSH banner was not accepted")
	}

	host, port = startBannerServer(t, "HTTP/1.1 200 OK\r\n")
	if remoteSSHTargetReachable(host, port) {
		t.Fatal("non-SSH TCP service was accepted")
	}
}

func startBannerServer(t *testing.T, banner string) (string, int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		_, _ = io.WriteString(connection, banner)
	}()
	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
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

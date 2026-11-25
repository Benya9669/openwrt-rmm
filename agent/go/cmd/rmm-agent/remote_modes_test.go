package main

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestSelectLuCIListener(t *testing.T) {
	local := map[string]bool{"192.168.1.1": true}
	for _, tc := range []struct {
		bindings string
		want     luciListener
	}{
		{"[::]:443 0.0.0.0:8443", luciListener{"127.0.0.1", 8443}},
		{"[::]:8443", luciListener{"::1", 8443}},
		{"192.168.1.1:443", luciListener{"192.168.1.1", 443}},
		{"80", luciListener{"127.0.0.1", 80}},
		{"192.168.1.2:443 garbage 0.0.0.0:65536", luciListener{}},
	} {
		if got := selectLuCIListener(tc.bindings, local); got != tc.want {
			t.Errorf("bindings %q: got %v, want %v", tc.bindings, got, tc.want)
		}
	}
}

func TestLuCIOnlyReportsUnavailableWebListener(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	listener.Close()
	args := map[string]string{
		"session_id": "ras-test", "server_host": "example.test", "remote_port": "0",
		"luci_port": "22110", "luci_local_port": port, "credential_mode": "legacy",
	}
	output, code := remoteSSHReverseOutput(context.Background(), config{TunnelStateDir: t.TempDir()}, args)
	if code != 1 || !strings.Contains(output, "LuCI listener 127.0.0.1:"+port+" unavailable") {
		t.Fatalf("unexpected result: code=%d output=%q", code, output)
	}
}

func TestRemoteAccessModesRejectNoListener(t *testing.T) {
	args := map[string]string{"session_id": "ras-test", "server_host": "example.test", "remote_port": "22010", "luci_port": "0", "credential_mode": "legacy"}
	parsed, output, ok := parseRemoteSSHArgs(args)
	if !ok || parsed.RemotePort != 22010 || parsed.LuCIPort != 0 {
		t.Fatalf("SSH-only: %v %s", parsed, output)
	}
	args["remote_port"] = "0"
	args["luci_port"] = "22110"
	parsed, output, ok = parseRemoteSSHArgs(args)
	if !ok || parsed.RemotePort != 0 || parsed.LuCIPort != 22110 {
		t.Fatalf("LuCI-only: %v %s", parsed, output)
	}
	args["luci_port"] = "0"
	if _, _, ok = parseRemoteSSHArgs(args); ok {
		t.Fatal("empty listener set accepted")
	}
	args["remote_port"] = "-1"
	if _, _, ok = parseRemoteSSHArgs(args); ok {
		t.Fatal("negative forwarding port accepted")
	}
}

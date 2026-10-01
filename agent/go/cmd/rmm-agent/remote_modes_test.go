package main

import "testing"

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

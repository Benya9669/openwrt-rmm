package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"rmm-openwrt/internal/commandsig"
)

func TestAgentVersionIsStable(t *testing.T) {
	if agentVersion != "0.8.0" {
		t.Fatalf("unexpected agent version %q", agentVersion)
	}
}

func TestSignedCommandValidation(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	expires := now.Add(time.Hour)
	args := json.RawMessage(`{"target":"1.1.1.1"}`)
	cfg := config{DeviceID: "dev_test", CommandSigningPublicKey: commandsig.EncodePublicKey(publicKey), CommandSigningKeyID: commandsig.KeyID(publicKey)}
	cmd := command{ID: "cmd_test", DeviceID: cfg.DeviceID, Type: "ping", Args: args, CreatedAt: now, ExpiresAt: &expires, Nonce: "unique-nonce", SignatureKeyID: cfg.CommandSigningKeyID}
	cmd.Signature, err = commandsig.Sign(privateKey, commandsig.Envelope{ID: cmd.ID, DeviceID: cmd.DeviceID, Type: cmd.Type, Args: cmd.Args, CreatedAt: now, ExpiresAt: expires, Nonce: cmd.Nonce})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSignedCommand(cfg, cmd, now); err != nil {
		t.Fatalf("valid command was rejected: %v", err)
	}
	cmd.DeviceID = "dev_other"
	if err := validateSignedCommand(cfg, cmd, now); err == nil {
		t.Fatal("command bound to another device was accepted")
	}
}

func TestRestoreGuardRequiresANewerSuccessfulHeartbeat(t *testing.T) {
	dir := t.TempDir()
	createdAt := time.Now().UTC().Truncate(time.Second)
	guard := restoreGuard{EmergencyPath: filepath.Join(dir, "emergency.tar.gz"), CreatedAt: createdAt, Deadline: createdAt.Add(time.Minute)}
	if err := writeRestoreGuard(dir, guard); err != nil {
		t.Fatal(err)
	}
	if err := confirmPendingRestore(dir, createdAt.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, found, err := readRestoreGuard(dir); err != nil || !found {
		t.Fatalf("restore guard was confirmed by the heartbeat that applied it: found=%v err=%v", found, err)
	}
	if err := confirmPendingRestore(dir, createdAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, found, err := readRestoreGuard(dir); err != nil || found {
		t.Fatalf("restore guard remained after a later successful heartbeat: found=%v err=%v", found, err)
	}
}

func TestExecCommandWithEnvPassesSelfUpdateMarker(t *testing.T) {
	output, code := execCommandWithEnv(context.Background(), 5*time.Second,
		[]string{"GO_WANT_AGENT_ENV_HELPER=1", "RMM_AGENT_SELF_UPDATE=1"},
		os.Args[0], "-test.run=TestAgentEnvHelperProcess")
	if code != 0 || strings.TrimSpace(output) != "1" {
		t.Fatalf("self-update environment marker was not passed: output=%q code=%d", output, code)
	}
}

func TestAgentEnvHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_AGENT_ENV_HELPER") != "1" {
		return
	}
	_, _ = os.Stdout.WriteString(os.Getenv("RMM_AGENT_SELF_UPDATE"))
	os.Exit(0)
}

func TestSafeAgentFeedURL(t *testing.T) {
	for _, value := range []string{"https://packages.example.test/stable/24.10.7/x86-64", "https://packages.example.test/feed"} {
		if !safeAgentFeedURL(value) {
			t.Fatalf("expected %q to be accepted", value)
		}
	}
	for _, value := range []string{"http://packages.example.test/feed", "https://packages.example.test/feed?next=x", "https://packages.example.test/a/../feed", ""} {
		if safeAgentFeedURL(value) {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestValidAgentPackageOperationArgs(t *testing.T) {
	valid := map[string]string{"package": "rmm-agent-go-production", "target_version": "0.6.10", "package_version": "0.6.10-r1", "feed_url": "https://packages.example.test/feed", "manifest_url": "https://packages.example.test/update-manifest.json", "signature_url": "https://packages.example.test/update-manifest.sig", "package_manager": "apk"}
	if !validAgentPackageOperationArgs(valid) {
		t.Fatal("expected exact package operation arguments to be accepted")
	}
	valid["unexpected"] = "value"
	if validAgentPackageOperationArgs(valid) {
		t.Fatal("unexpected package operation field was accepted")
	}
}

func TestValidateAgentUpdateManifestRequiresExactSignedFeed(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	manifest := []byte(`{"schema":1,"channel":"stable","agent":{"version":"0.6.10"},"packages":[{"openwrt_release":"25.12.4","target":"ramips-mt7621","format":"apk","feed_url":"https://packages.example.test/feed","package_version":"0.6.10-r1"}]}`)
	digest := sha256.Sum256(manifest)
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	args := map[string]string{"target_version": "0.6.10", "feed_url": "https://packages.example.test/feed", "package_version": "0.6.10-r1", "package_manager": "apk"}
	if err := validateAgentUpdateManifest(manifest, signature, publicPEM, args, "25.12.4", "ramips-mt7621"); err != nil {
		t.Fatalf("valid signed manifest rejected: %v", err)
	}
	args["feed_url"] = "https://packages.example.test/other"
	if err := validateAgentUpdateManifest(manifest, signature, publicPEM, args, "25.12.4", "ramips-mt7621"); err == nil {
		t.Fatal("feed not present in the signed manifest was accepted")
	}
	manifest[0] ^= 1
	if err := validateAgentUpdateManifest(manifest, signature, publicPEM, args, "25.12.4", "ramips-mt7621"); err == nil {
		t.Fatal("tampered manifest was accepted")
	}
}

func TestParseInstalledAgentPackageVersion(t *testing.T) {
	if got, ok := parseInstalledPackageVersion("opkg", "rmm-agent-go-production", "Package: rmm-agent-go-production\nVersion: 0.6.10-1\nStatus: install ok installed\n"); !ok || got != "0.6.10-1" {
		t.Fatalf("unexpected opkg version %q, %v", got, ok)
	}
	if got, ok := parseInstalledPackageVersion("apk", "rmm-agent-go-production", "rmm-agent-go-production-0.6.10-r1\n"); !ok || got != "0.6.10-r1" {
		t.Fatalf("unexpected apk version %q, %v", got, ok)
	}
}

func TestOpenWrtTargetManifestFormat(t *testing.T) {
	if strings.ReplaceAll("x86/64", "/", "-") != "x86-64" {
		t.Fatal("OpenWrt target was not normalized")
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

func TestSecureRemoteSSHArgsRequirePinnedHostKey(t *testing.T) {
	base := map[string]string{
		"session_id": "ras_secure123", "server_host": "rmm.example.test", "server_port": "2222",
		"remote_port": "22040", "luci_port": "22140", "credential_mode": "device",
	}
	if _, _, ok := parseRemoteSSHArgs(base); ok {
		t.Fatal("device credential mode accepted without pinned host key")
	}
	base["server_host_key"] = testSSHHostPublicKey(t)
	parsed, output, ok := parseRemoteSSHArgs(base)
	if !ok || output != "" || parsed.CredentialMode != "device" {
		t.Fatalf("secure tunnel args rejected: parsed=%#v output=%q ok=%v", parsed, output, ok)
	}
}

func TestSecureRemoteSSHCommandPinsHostAndRequestsExplicitBind(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("OpenSSH client is unavailable")
	}
	dir := t.TempDir()
	identity := filepath.Join(dir, "device-key")
	if err := os.WriteFile(identity, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config{TunnelIdentity: identity, TunnelStateDir: dir}
	args := remoteSSHArgs{
		SessionID: "ras_secure123", ServerHost: "rmm.example.test", ServerPort: 2222,
		RemotePort: 22040, LuCIPort: 22140, LuCILocalPort: 80, LocalHost: "127.0.0.1", LocalPort: 22,
		ServerUser: "rmm-tunnel", CredentialMode: "device", ServerHostKey: testSSHHostPublicKey(t),
	}
	_, commandArgs, err := remoteSSHCommand(cfg, args)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(commandArgs, " ")
	for _, expected := range []string{
		"StrictHostKeyChecking=yes", "UserKnownHostsFile=", "0.0.0.0:22040:127.0.0.1:22", "0.0.0.0:22140:127.0.0.1:80",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("secure SSH command misses %q: %s", expected, joined)
		}
	}
	knownHosts, err := os.ReadFile(remoteSSHKnownHostsFile(cfg, args.SessionID))
	if err != nil || !strings.Contains(string(knownHosts), "[rmm.example.test]:2222 ssh-ed25519") {
		t.Fatalf("pinned known_hosts = %q err=%v", knownHosts, err)
	}
}

func testSSHHostPublicKey(t *testing.T) string {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey)))
}

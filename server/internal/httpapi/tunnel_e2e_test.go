package httpapi_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"rmm-openwrt/internal/commandsig"
	"rmm-openwrt/server/internal/httpapi"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

// Opt-in: executes the real Go agent and production OpenSSH image. No router,
// public DNS, production volume or credentials are used.
func TestReverseTunnelCloudLuCIE2E(t *testing.T) {
	agentPath := os.Getenv("RMM_TEST_AGENT_BINARY")
	if agentPath == "" {
		t.Skip("set RMM_TEST_AGENT_BINARY and build rmm-tunnel-e2e image to run real SSH E2E")
	}
	if runtime.GOOS != "linux" {
		t.Fatal("real-agent tunnel E2E requires Linux")
	}
	host := os.Getenv("RMM_TEST_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	bind := os.Getenv("RMM_TEST_API_BIND")
	if bind == "" {
		bind = "0.0.0.0:0"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	dir := t.TempDir()
	st, err := store.OpenSQLite(ctx, filepath.Join(dir, "tunnel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	signing, err := commandsig.LoadOrCreatePrivateKey(filepath.Join(dir, "command.pem"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetCommandSigningKey(signing); err != nil {
		t.Fatal(err)
	}
	user, err := st.EnsureBootstrapUser(ctx, "admin", "unused-hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateEnrollmentGrant(ctx, user.ID, "office", "e2e-grant", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	device, found, err := st.EnrollDeviceWithGrant(ctx, "e2e-grant", "office-router", "24.10")
	if err != nil || !found {
		t.Fatalf("enrollment: %v %v", found, err)
	}
	token := "test-only-tunnel-authorization-1234567890"
	// The authorization server listens on all interfaces only inside this test.
	api := httptest.NewUnstartedServer(nil)
	api.Listener.Close()
	api.Listener, err = net.Listen("tcp", bind)
	if err != nil {
		t.Fatal(err)
	}
	apiPort := api.Listener.Addr().(*net.TCPAddr).Port
	wait := func(label string, probe func() bool) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		if strings.Contains(label, "reaped") {
			deadline = time.Now().Add(12 * time.Second)
		}
		for ctx.Err() == nil && time.Now().Before(deadline) {
			if probe() {
				return
			}
			time.Sleep(150 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %s", label)
	}
	hostKey := os.Getenv("RMM_TEST_TUNNEL_HOST_KEY")
	sshPort := os.Getenv("RMM_TEST_TUNNEL_SSH_PORT")
	if hostKey == "" || sshPort == "" {
		t.Fatal("test SSH host key and port are required")
	}
	handler := httpapi.NewHandler(st, httpapi.Config{
		OperatorUsername: "admin", OperatorPassword: "correct-horse-battery-staple", OperatorToken: "e2e-operator",
		TunnelAuthToken: token, TunnelHostPublicKey: hostKey, TunnelHTTPHost: host,
		DeviceDomain: "routers.example.test", PublicScheme: "https", CookieSecure: true,
		CommandSigningPublicKey: commandsig.EncodePublicKey(st.CommandSigningPublicKey()), CommandSigningKeyID: commandsig.KeyID(st.CommandSigningPublicKey()),
	})
	api.Config.Handler = handler
	api.Start()
	defer api.Close()
	defer handler.(interface{ Shutdown(context.Context) error }).Shutdown(context.Background())
	apiURL := fmt.Sprintf("http://127.0.0.1:%d", apiPort)
	var upstreamDown atomic.Bool
	luci := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if upstreamDown.Load() {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close()
			}
			return
		}
		if r.Header.Get("Cookie") != "" {
			t.Error("RMM credential cookie leaked to LuCI")
		}
		if r.URL.Path == "/cgi-bin/luci/" && r.Host != "127.0.0.1" {
			t.Errorf("unexpected LuCI Host: %s", r.Host)
		}
		fmt.Fprint(w, "test LuCI over reverse SSH")
	}))
	defer luci.Close()
	_, luciPort, _ := net.SplitHostPort(strings.TrimPrefix(luci.URL, "http://"))
	localSSH, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer localSSH.Close()
	go func() {
		for {
			conn, err := localSSH.Accept()
			if err != nil {
				return
			}
			fmt.Fprint(conn, "SSH-2.0-RMM-test\r\n")
			conn.Close()
		}
	}()
	_, localPort, _ := net.SplitHostPort(localSSH.Addr().String())
	agent := exec.CommandContext(ctx, agentPath, "-config", filepath.Join(dir, "agent.conf"))
	agent.Env = append(os.Environ(),
		"SERVER_URL="+apiURL, "DEVICE_ID="+device.DeviceID, "DEVICE_TOKEN="+device.DeviceToken, "INTERVAL_SECONDS=1", "CHECK_TARGETS=127.0.0.1",
		"LOCK_FILE="+filepath.Join(dir, "agent.lock"), "SPOOL_DIR="+filepath.Join(dir, "spool"), "BACKUP_DIR="+filepath.Join(dir, "backups"),
		"TUNNEL_DEVICE_IDENTITY_FILE="+filepath.Join(dir, "identity"), "TUNNEL_STATE_DIR="+filepath.Join(dir, "tunnels"),
		"COMMAND_STATE_DIR="+filepath.Join(dir, "commands"), "RECOVERY_DIR="+filepath.Join(dir, "recovery"),
		"COMMAND_SIGNING_PUBLIC_KEY="+commandsig.EncodePublicKey(st.CommandSigningPublicKey()), "COMMAND_SIGNING_KEY_ID="+commandsig.KeyID(st.CommandSigningPublicKey()))
	logFile, err := os.Create(filepath.Join(dir, "agent.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	agent.Stdout, agent.Stderr = logFile, logFile
	if err := agent.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = agent.Process.Signal(os.Interrupt)
		_ = agent.Wait()
		if t.Failed() {
			data, _ := os.ReadFile(logFile.Name())
			t.Logf("agent log: %s", data)
		}
	}()
	wait("real agent heartbeat and device key", func() bool { ready, err := st.TunnelCredentialReady(ctx, device.DeviceID); return err == nil && ready })
	startTunnel := func() model.RemoteSession {
		t.Helper()
		session, found, err := st.CreateRemoteSession(ctx, model.RemoteSession{DeviceID: device.DeviceID, Target: "ssh", Status: "requested", ServerHost: host, RemotePort: 22055, LuCIPort: 22155, LuCIScheme: "http", ExpiresAt: time.Now().Add(time.Minute)})
		if err != nil || !found {
			t.Fatalf("session: %v %v", found, err)
		}
		args, _ := json.Marshal(map[string]string{"session_id": session.ID, "server_host": host, "server_port": sshPort, "remote_port": "22055", "luci_port": "22155", "luci_local_port": luciPort, "local_host": "127.0.0.1", "local_port": localPort, "server_user": "rmm-tunnel", "duration_seconds": "60", "server_host_key": hostKey, "credential_mode": "device"})
		command, found, err := st.CreateCommand(ctx, device.DeviceID, "remote_ssh_reverse", args)
		if err != nil || !found {
			t.Fatalf("command: %v %v", found, err)
		}
		if _, found, err := st.AttachRemoteSessionCommand(ctx, device.DeviceID, session.ID, command.ID); err != nil || !found {
			t.Fatalf("attach: %v %v", found, err)
		}
		wait("agent-confirmed reverse tunnel", func() bool {
			result, found, err := st.GetCommand(ctx, device.DeviceID, command.ID)
			if err == nil && found && result.Status == "failed" {
				t.Fatalf("agent rejected tunnel command: %s", result.Output)
			}
			sessions, _, err := st.ListRemoteSessions(ctx, device.DeviceID, store.RemoteSessionListOptions{})
			return err == nil && containsActiveSession(sessions, session.ID)
		})

		return session
	}
	session := startTunnel()

	// Use a trusted wildcard certificate and real TLS SNI, with DNS resolved only
	// by this test transport. TLS verification remains enabled.
	browserServer := httptest.NewUnstartedServer(handler)
	certificate, root := testWildcardCertificate(t)
	browserServer.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}
	browserServer.StartTLS()
	defer browserServer.Close()
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: root, MinVersion: tls.VersionTLS12}, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, browserServer.Listener.Addr().String())
	}}
	defer transport.CloseIdleConnections()
	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Transport: transport, Jar: jar, Timeout: 5 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	operator := &http.Client{Timeout: 5 * time.Second}
	access := func() string {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, apiURL+"/api/devices/"+device.DeviceID+"/remote-sessions/"+session.ID+"/access", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer e2e-operator")
		req.Header.Set("Content-Type", "application/json")
		resp, err := operator.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var result struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusCreated || result.URL == "" {
			t.Fatalf("access grant: %d", resp.StatusCode)
		}
		return result.URL
	}
	get := func(address string, want int) string {
		t.Helper()
		resp, err := browser.Get(address)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("cloud response: %d, want %d: %s", resp.StatusCode, want, body)
		}
		return string(body)
	}
	grantURL := access()
	get(strings.Replace(grantURL, "office.routers", "wrong.routers", 1), http.StatusUnauthorized)
	get(grantURL, http.StatusSeeOther)
	get(grantURL, http.StatusUnauthorized)
	cloudURL := "https://office.routers.example.test/cgi-bin/luci/"
	if body := get(cloudURL, http.StatusOK); body != "test LuCI over reverse SSH" {
		t.Fatalf("wrong upstream: %s", body)
	}
	upstreamDown.Store(true)
	get(cloudURL, http.StatusBadGateway)
	upstreamDown.Store(false)
	get(cloudURL, http.StatusOK)
	if resp, err := browser.Get("https://unrelated.example.test/"); err == nil {
		resp.Body.Close()
		t.Fatal("wildcard TLS accepted a hostname outside its certificate")
	}
	// A cookie must stop authorizing immediately when its remote session expires.
	fixtureDB, err := sql.Open("sqlite", filepath.Join(dir, "tunnel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fixtureDB.Close()
	if _, err := fixtureDB.ExecContext(ctx, `UPDATE remote_sessions SET expires_at = ? WHERE id = ?`, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), session.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.ExpireRemoteSessions(ctx); err != nil {
		t.Fatal(err)
	}
	get(cloudURL, http.StatusUnauthorized)
	wait("expired SSH listeners reaped", func() bool {
		client := &http.Client{Timeout: 500 * time.Millisecond}
		resp, err := client.Get("http://" + net.JoinHostPort(host, "22155") + "/")
		if resp != nil {
			resp.Body.Close()
		}
		return err != nil
	})
	session = startTunnel()
	get(access(), http.StatusSeeOther)
	get(cloudURL, http.StatusOK)
	// Emergency revoke must also prevent a fresh SSH authentication using the same key.
	if _, err := st.RevokeDeviceCredential(ctx, device.DeviceID); err != nil {
		t.Fatal(err)
	}
	get(cloudURL, http.StatusUnauthorized)
	wait("revoked SSH listener reaped", func() bool {
		client := &http.Client{Timeout: 500 * time.Millisecond}
		resp, err := client.Get("http://" + net.JoinHostPort(host, "22155") + "/")
		if resp != nil {
			resp.Body.Close()
		}
		return err != nil
	})
	ready, err := st.TunnelCredentialReady(ctx, device.DeviceID)
	if err != nil || ready {
		t.Fatalf("revoked credential remains ready: %v %v", ready, err)
	}
	privateData, err := os.ReadFile(filepath.Join(dir, "identity"))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.ParsePrivateKey(privateData)
	if err != nil {
		t.Fatal(err)
	}
	pinnedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hostKey))
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := ssh.Dial("tcp", net.JoinHostPort(host, sshPort), &ssh.ClientConfig{User: "rmm-tunnel", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: ssh.FixedHostKey(pinnedKey), HostKeyAlgorithms: []string{pinnedKey.Type()}, Timeout: 3 * time.Second})
	if rejected != nil {
		rejected.Close()
		t.Fatal("revoked key authenticated to real sshd")
	}
	if err == nil || !strings.Contains(err.Error(), "unable to authenticate") {
		t.Fatalf("expected authentication rejection: %v", err)
	}
}

func testWildcardCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "RMM test"}, DNSNames: []string{"*.routers.example.test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	cert, err := tls.X509KeyPair(certPEM, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateDER}))
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)
	return cert, pool
}

func containsActiveSession(sessions []model.RemoteSession, id string) bool {
	for _, session := range sessions {
		if session.ID == id && session.Status == "active" {
			return true
		}
	}
	return false
}

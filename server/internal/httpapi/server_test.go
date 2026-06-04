package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"rmm-openwrt/server/internal/httpapi"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

func TestAgentOperatorSmokeFlow(t *testing.T) {
	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{
		EnrollmentToken: "enroll-test",
		OperatorToken:   "operator-test",
	}))
	defer srv.Close()

	var enrolled struct {
		DeviceID    string `json:"device_id"`
		DeviceToken string `json:"device_token"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/agent/enroll", "", map[string]any{
		"enrollment_token": "enroll-test",
		"hostname":         "test-openwrt",
		"openwrt_version":  "OpenWrt test",
	}, http.StatusCreated, &enrolled)
	if enrolled.DeviceID == "" || enrolled.DeviceToken == "" {
		t.Fatalf("expected enrollment credentials, got %#v", enrolled)
	}

	var heartbeat struct {
		Commands []any `json:"commands"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/agent/heartbeat", enrolled.DeviceToken, map[string]any{
		"device_id": enrolled.DeviceID,
		"inventory": map[string]any{
			"hostname": "test-openwrt",
		},
		"metrics": map[string]any{
			"loadavg": "0.00 0.01 0.02",
		},
	}, http.StatusOK, &heartbeat)
	if len(heartbeat.Commands) != 0 {
		t.Fatalf("expected no commands, got %d", len(heartbeat.Commands))
	}

	var history struct {
		Samples []struct {
			DeviceID string `json:"device_id"`
		} `json:"samples"`
	}
	requestJSON(t, http.MethodGet, srv.URL+"/api/devices/"+enrolled.DeviceID+"/metrics-history", "operator-test", nil, http.StatusOK, &history)
	if len(history.Samples) != 1 || history.Samples[0].DeviceID != enrolled.DeviceID {
		t.Fatalf("unexpected metrics history: %#v", history)
	}

	var alerts struct {
		Alerts []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Severity string `json:"severity"`
			Status   string `json:"status"`
		} `json:"alerts"`
	}
	requestJSON(t, http.MethodGet, srv.URL+"/api/devices/"+enrolled.DeviceID+"/alerts", "operator-test", nil, http.StatusOK, &alerts)
	if len(alerts.Alerts) != 0 {
		t.Fatalf("expected no active alerts, got %#v", alerts.Alerts)
	}

	var fleetDevice struct {
		ID    string   `json:"id"`
		Group string   `json:"group"`
		Tags  []string `json:"tags"`
	}
	requestJSON(t, http.MethodPatch, srv.URL+"/api/devices/"+enrolled.DeviceID+"/fleet", "operator-test", map[string]any{
		"group": "lab",
		"tags":  []string{"edge", "vpn", "edge"},
	}, http.StatusOK, &fleetDevice)
	if fleetDevice.ID != enrolled.DeviceID || fleetDevice.Group != "lab" || len(fleetDevice.Tags) != 2 {
		t.Fatalf("unexpected fleet metadata: %#v", fleetDevice)
	}

	requestJSON(t, http.MethodGet, srv.URL+"/api/devices", "", nil, http.StatusUnauthorized, nil)

	var created struct {
		ID     string          `json:"id"`
		Type   string          `json:"type"`
		Args   json.RawMessage `json:"args"`
		Status string          `json:"status"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands", "operator-test", map[string]any{
		"type": "ping",
		"args": map[string]any{"target": "1.1.1.1"},
	}, http.StatusCreated, &created)
	if created.ID == "" || created.Type != "ping" || created.Status != "queued" {
		t.Fatalf("unexpected created command: %#v", created)
	}
	if string(created.Args) != `{"target":"1.1.1.1"}` {
		t.Fatalf("expected compact args, got %s", created.Args)
	}

	nextReq := map[string]any{"device_id": enrolled.DeviceID}
	nextLine := requestText(t, http.MethodPost, srv.URL+"/api/agent/commands/next", enrolled.DeviceToken, nextReq, http.StatusOK)
	expectedPrefix := created.ID + "\tping\t"
	if !strings.HasPrefix(nextLine, expectedPrefix) {
		t.Fatalf("expected next command prefix %q, got %q", expectedPrefix, nextLine)
	}
	if !strings.Contains(nextLine, `{"target":"1.1.1.1"}`) {
		t.Fatalf("expected compact next command args, got %q", nextLine)
	}
	if err := st.ExpireClaimedCommands(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	nextLine = requestText(t, http.MethodPost, srv.URL+"/api/agent/commands/next", enrolled.DeviceToken, nextReq, http.StatusOK)
	if !strings.HasPrefix(nextLine, expectedPrefix) {
		t.Fatalf("expected retried command prefix %q, got %q", expectedPrefix, nextLine)
	}

	var result struct {
		Status string `json:"status"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/agent/commands/"+created.ID+"/result", enrolled.DeviceToken, map[string]any{
		"device_id": enrolled.DeviceID,
		"status":    "completed",
		"exit_code": 0,
		"output":    "network.wg0.private_key='super-secret'\nnetwork.lan.ipaddr='10.0.0.1'",
		"result":    map[string]any{},
	}, http.StatusOK, &result)
	if result.Status != "ok" {
		t.Fatalf("unexpected result status: %q", result.Status)
	}

	var commandDetail struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		Output       string `json:"output"`
		AttemptCount int    `json:"attempt_count"`
	}
	requestJSON(t, http.MethodGet, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands/"+created.ID, "operator-test", nil, http.StatusOK, &commandDetail)
	if commandDetail.ID != created.ID || commandDetail.Status != "completed" || commandDetail.AttemptCount != 2 {
		t.Fatalf("unexpected command detail: %#v", commandDetail)
	}
	if !strings.Contains(commandDetail.Output, "private_key='[redacted]'") || strings.Contains(commandDetail.Output, "super-secret") {
		t.Fatalf("expected redacted command output, got %q", commandDetail.Output)
	}

	var expiring struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands", "operator-test", map[string]any{
		"type": "opkg_list_installed",
		"args": map[string]any{},
	}, http.StatusCreated, &expiring)
	for i := 0; i < 3; i++ {
		nextLine = requestText(t, http.MethodPost, srv.URL+"/api/agent/commands/next", enrolled.DeviceToken, nextReq, http.StatusOK)
		if !strings.HasPrefix(nextLine, expiring.ID+"\topkg_list_installed\t") {
			t.Fatalf("expected expiring command attempt %d, got %q", i+1, nextLine)
		}
		if err := st.ExpireClaimedCommands(context.Background(), 0); err != nil {
			t.Fatal(err)
		}
	}
	requestText(t, http.MethodPost, srv.URL+"/api/agent/commands/next", enrolled.DeviceToken, nextReq, http.StatusNoContent)
	requestJSON(t, http.MethodGet, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands/"+expiring.ID, "operator-test", nil, http.StatusOK, &expiring)
	if expiring.Status != "expired" {
		t.Fatalf("expected expired command, got %#v", expiring)
	}

	requestJSON(t, http.MethodGet, srv.URL+"/api/devices/"+enrolled.DeviceID+"/alerts", "operator-test", nil, http.StatusOK, &alerts)
	foundCommandAlert := false
	commandAlertID := ""
	for _, alert := range alerts.Alerts {
		if alert.Type == "command_attention" && alert.Severity == "warning" {
			foundCommandAlert = true
			commandAlertID = alert.ID
		}
	}
	if !foundCommandAlert {
		t.Fatalf("expected command attention alert, got %#v", alerts.Alerts)
	}
	var acknowledgedAlert struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/alerts/"+commandAlertID+"/acknowledge", "operator-test", nil, http.StatusOK, &acknowledgedAlert)
	if acknowledgedAlert.ID != commandAlertID || acknowledgedAlert.Status != "acknowledged" {
		t.Fatalf("unexpected acknowledged alert: %#v", acknowledgedAlert)
	}

	var commandHistory struct {
		Commands []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"commands"`
	}
	requestJSON(t, http.MethodGet, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands", "operator-test", nil, http.StatusOK, &commandHistory)
	foundCreated := false
	for _, c := range commandHistory.Commands {
		if c.ID == created.ID && c.Status == "completed" {
			foundCreated = true
		}
	}
	if !foundCreated {
		t.Fatalf("unexpected command history: %#v", commandHistory)
	}

	var cancellable struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands", "operator-test", map[string]any{
		"type": "traceroute",
		"args": map[string]any{"target": "1.1.1.1"},
	}, http.StatusCreated, &cancellable)
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands/"+cancellable.ID+"/cancel", "operator-test", nil, http.StatusOK, &cancellable)
	if cancellable.Status != "cancelled" {
		t.Fatalf("expected cancelled command, got %#v", cancellable)
	}

	var packageCommand struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Status string `json:"status"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands", "operator-test", map[string]any{
		"type": "pkg_list_upgradable",
		"args": map[string]any{},
	}, http.StatusCreated, &packageCommand)
	if packageCommand.Type != "pkg_list_upgradable" || packageCommand.Status != "queued" {
		t.Fatalf("unexpected package command: %#v", packageCommand)
	}

	var bulk struct {
		Commands []struct {
			ID     string `json:"id"`
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"commands"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/bulk-commands", "operator-test", map[string]any{
		"device_ids": []string{enrolled.DeviceID, enrolled.DeviceID},
		"type":       "ping",
		"args":       map[string]any{"target": "1.1.1.1"},
	}, http.StatusCreated, &bulk)
	if len(bulk.Commands) != 1 || bulk.Commands[0].Type != "ping" || bulk.Commands[0].Status != "queued" {
		t.Fatalf("unexpected bulk commands: %#v", bulk.Commands)
	}

	var uciCommand struct {
		ID     string          `json:"id"`
		Type   string          `json:"type"`
		Status string          `json:"status"`
		Args   json.RawMessage `json:"args"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands", "operator-test", map[string]any{
		"type": "uci_show",
		"args": map[string]any{"config": "network"},
	}, http.StatusCreated, &uciCommand)
	if uciCommand.Type != "uci_show" || uciCommand.Status != "queued" {
		t.Fatalf("unexpected uci command: %#v", uciCommand)
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands", "operator-test", map[string]any{
		"type": "uci_backup",
		"args": map[string]any{"config": "network"},
	}, http.StatusCreated, &uciCommand)
	if uciCommand.Type != "uci_backup" || uciCommand.Status != "queued" {
		t.Fatalf("unexpected uci backup command: %#v", uciCommand)
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands", "operator-test", map[string]any{
		"type": "uci_preview",
		"args": map[string]any{"config": "network", "section": "lan", "option": "ipaddr", "value": "10.10.10.1/24"},
	}, http.StatusCreated, &uciCommand)
	if uciCommand.Type != "uci_preview" || uciCommand.Status != "queued" {
		t.Fatalf("unexpected uci preview command: %#v", uciCommand)
	}
	for _, commandType := range []string{"uci_commit_confirmed", "uci_restore"} {
		requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands", "operator-test", map[string]any{
			"type": commandType,
			"args": map[string]any{"config": "network"},
		}, http.StatusCreated, &uciCommand)
		if uciCommand.Type != commandType || uciCommand.Status != "queued" {
			t.Fatalf("unexpected %s command: %#v", commandType, uciCommand)
		}
	}

	var remoteSession struct {
		ID         string `json:"id"`
		Status     string `json:"status"`
		CommandID  string `json:"command_id"`
		ServerHost string `json:"server_host"`
		ServerPort int    `json:"server_port"`
		RemotePort int    `json:"remote_port"`
		LuCIPort   int    `json:"luci_port"`
		LocalPort  int    `json:"local_port"`
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/remote-sessions", "operator-test", map[string]any{
		"target":           "ssh",
		"server_host":      "10.10.10.2",
		"server_port":      22,
		"remote_port":      22022,
		"local_port":       22,
		"luci_scheme":      "https",
		"duration_seconds": 900,
	}, http.StatusCreated, &remoteSession)
	if remoteSession.ID == "" || remoteSession.Status != "queued" || remoteSession.CommandID == "" || remoteSession.RemotePort != 22022 || remoteSession.LuCIPort < 22100 || remoteSession.LuCIPort > 22199 {
		t.Fatalf("unexpected remote session: %#v", remoteSession)
	}
	var remoteSessions struct {
		Sessions []struct {
			ID        string `json:"id"`
			CommandID string `json:"command_id"`
			Status    string `json:"status"`
		} `json:"remote_sessions"`
	}
	requestJSON(t, http.MethodGet, srv.URL+"/api/devices/"+enrolled.DeviceID+"/remote-sessions", "operator-test", nil, http.StatusOK, &remoteSessions)
	if len(remoteSessions.Sessions) != 1 || remoteSessions.Sessions[0].ID != remoteSession.ID || remoteSessions.Sessions[0].CommandID == "" {
		t.Fatalf("unexpected remote sessions: %#v", remoteSessions.Sessions)
	}
	requestJSON(t, http.MethodGet, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands/"+remoteSession.CommandID, "operator-test", nil, http.StatusOK, &uciCommand)
	if uciCommand.Type != "remote_ssh_reverse" || uciCommand.Status != "queued" {
		t.Fatalf("unexpected remote command: %#v", uciCommand)
	}
	if !strings.Contains(string(uciCommand.Args), `"luci_local_port":"443"`) || !strings.Contains(string(uciCommand.Args), `"luci_port":"`) {
		t.Fatalf("remote command does not include LuCI HTTPS forward: %s", uciCommand.Args)
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/agent/commands/"+remoteSession.CommandID+"/result", enrolled.DeviceToken, map[string]any{
		"device_id": enrolled.DeviceID,
		"status":    "completed",
		"exit_code": 0,
		"output":    "remote ssh reverse started",
		"result":    map[string]any{},
	}, http.StatusOK, &result)
	requestJSON(t, http.MethodGet, srv.URL+"/api/devices/"+enrolled.DeviceID+"/remote-sessions", "operator-test", nil, http.StatusOK, &remoteSessions)
	if len(remoteSessions.Sessions) != 1 || remoteSessions.Sessions[0].Status != "active" {
		t.Fatalf("expected active remote session, got %#v", remoteSessions.Sessions)
	}
	requestJSON(t, http.MethodPost, srv.URL+"/api/devices/"+enrolled.DeviceID+"/remote-sessions/"+remoteSession.ID+"/close", "operator-test", nil, http.StatusOK, &remoteSession)
	if remoteSession.Status != "closed" {
		t.Fatalf("expected closed remote session, got %#v", remoteSession)
	}
	var commandsAfterClose struct {
		Commands []struct {
			Type string          `json:"type"`
			Args json.RawMessage `json:"args"`
		} `json:"commands"`
	}
	requestJSON(t, http.MethodGet, srv.URL+"/api/devices/"+enrolled.DeviceID+"/commands", "operator-test", nil, http.StatusOK, &commandsAfterClose)
	foundRemoteClose := false
	for _, command := range commandsAfterClose.Commands {
		if command.Type == "remote_ssh_close" && strings.Contains(string(command.Args), remoteSession.ID) {
			foundRemoteClose = true
		}
	}
	if !foundRemoteClose {
		t.Fatalf("expected remote_ssh_close command, got %#v", commandsAfterClose.Commands)
	}

	var devices struct {
		Devices []struct {
			ID           string `json:"id"`
			Online       bool   `json:"online"`
			ActiveAlerts int    `json:"active_alerts"`
		} `json:"devices"`
	}
	requestJSON(t, http.MethodGet, srv.URL+"/api/devices", "operator-test", nil, http.StatusOK, &devices)
	if len(devices.Devices) != 1 {
		t.Fatalf("expected one device, got %d", len(devices.Devices))
	}
	if devices.Devices[0].ID != enrolled.DeviceID || !devices.Devices[0].Online {
		t.Fatalf("unexpected device list: %#v", devices.Devices)
	}
	if devices.Devices[0].ActiveAlerts < 1 {
		t.Fatalf("expected open alert count, got %#v", devices.Devices)
	}

	var audit struct {
		Events []struct {
			Action    string `json:"action"`
			DeviceID  string `json:"device_id"`
			CommandID string `json:"command_id"`
		} `json:"audit_events"`
	}
	requestJSON(t, http.MethodGet, srv.URL+"/api/audit-events?device_id="+enrolled.DeviceID, "operator-test", nil, http.StatusOK, &audit)
	if len(audit.Events) < 2 {
		t.Fatalf("expected audit events, got %#v", audit.Events)
	}
}

func TestLuCIProxyRequiresActiveSessionAndRewritesPaths(t *testing.T) {
	var upstreamPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "127.0.0.1" {
			http.Error(w, "rejected host", http.StatusForbidden)
			return
		}
		for _, header := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Port", "X-Forwarded-Proto", "X-Real-IP", "Origin", "Referer"} {
			if r.Header.Get(header) != "" {
				http.Error(w, "rejected forwarded request", http.StatusForbidden)
				return
			}
		}
		if strings.Contains(r.Header.Get("Cookie"), "rmm_operator_session=") {
			t.Fatal("operator session cookie leaked to LuCI upstream")
		}
		if strings.Contains(r.Header.Get("Cookie"), "rmm_luci_route=") {
			t.Fatal("LuCI route cookie leaked to LuCI upstream")
		}
		upstreamPath = r.URL.Path
		if r.URL.Path == "/luci-static/resources/luci.js" {
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = io.WriteString(w, `const untouched = '/cgi-bin/luci';`)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<a href="/cgi-bin/luci/admin">LuCI</a><link href="/luci-static/test.css"><script>L = new LuCI({ "resource": "\/luci-static\/resources", "scriptname": "\/cgi-bin\/luci", "ubuspath": "\/ubus\/" });</script>`)
	}))
	defer upstream.Close()
	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, portText, _ := strings.Cut(upstreamURL.Host, ":")
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}

	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "luci.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	enrolled, err := st.EnrollDevice(context.Background(), "router", "OpenWrt")
	if err != nil {
		t.Fatal(err)
	}
	session, _, err := st.CreateRemoteSession(context.Background(), model.RemoteSession{
		DeviceID:   enrolled.DeviceID,
		Target:     "ssh",
		Status:     "active",
		ServerHost: "tunnel",
		ServerPort: 2222,
		RemotePort: 22000,
		LuCIPort:   port,
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{
		OperatorToken:  "operator-test",
		TunnelHTTPHost: "127.0.0.1",
	}))
	defer srv.Close()

	body := requestText(t, http.MethodGet, srv.URL+"/luci/"+enrolled.DeviceID+"/"+session.ID+"/", "operator-test", nil, http.StatusOK)
	prefix := "/luci/" + enrolled.DeviceID + "/" + session.ID
	if !strings.Contains(body, `href="/cgi-bin/luci/admin"`) || !strings.Contains(body, `href="/luci-static/test.css"`) {
		t.Fatalf("LuCI body was unexpectedly modified: %s", body)
	}
	for _, path := range []string{`\/luci-static\/resources`, `\/cgi-bin\/luci`, `\/ubus\/`} {
		if !strings.Contains(body, path) {
			t.Fatalf("escaped LuCI path %s was unexpectedly modified: %s", path, body)
		}
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	for _, path := range []string{prefix + "/", "/cgi-bin/luci/admin"} {
		req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer operator-test")
		req.Header.Set("Forwarded", "for=203.0.113.10;proto=https")
		req.Header.Set("X-Forwarded-For", "203.0.113.10")
		req.Header.Set("X-Forwarded-Host", "rmm.example.test")
		req.Header.Set("X-Forwarded-Port", "443")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Real-IP", "203.0.113.10")
		req.Header.Set("Origin", "https://rmm.example.test")
		req.Header.Set("Referer", "https://rmm.example.test/luci/")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("LuCI route %s returned %d", path, resp.StatusCode)
		}
	}
	if upstreamPath != "/cgi-bin/luci/admin" {
		t.Fatalf("LuCI fallback used upstream path %q", upstreamPath)
	}
	script := requestText(t, http.MethodGet, srv.URL+prefix+"/luci-static/resources/luci.js", "operator-test", nil, http.StatusOK)
	if script != `const untouched = '/cgi-bin/luci';` {
		t.Fatalf("LuCI JavaScript was unexpectedly rewritten: %s", script)
	}
}

func TestServesStaticWebUI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>RMM UI</title>"), 0644); err != nil {
		t.Fatal(err)
	}

	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{
		EnrollmentToken: "enroll-test",
		OperatorToken:   "operator-test",
		StaticDir:       dir,
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(data), "RMM UI") {
		t.Fatalf("unexpected static response %d: %s", resp.StatusCode, data)
	}
}

func TestOperatorCookieAuth(t *testing.T) {
	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{
		EnrollmentToken:  "enroll-test",
		OperatorToken:    "operator-test",
		OperatorUsername: "admin",
		OperatorPassword: "correct-horse-battery-staple",
		SessionSecret:    "test-session-secret-with-enough-entropy",
	}))
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	authRequestJSON(t, client, http.MethodGet, srv.URL+"/api/devices", nil, http.StatusUnauthorized, nil)
	authRequestJSON(t, client, http.MethodPost, srv.URL+"/api/auth/login", map[string]any{
		"username": "admin",
		"password": "wrong",
	}, http.StatusUnauthorized, nil)
	var me struct {
		Username string `json:"username"`
	}
	authRequestJSON(t, client, http.MethodPost, srv.URL+"/api/auth/login", map[string]any{
		"username": "admin",
		"password": "correct-horse-battery-staple",
	}, http.StatusOK, &me)
	if me.Username != "admin" {
		t.Fatalf("unexpected login response: %#v", me)
	}
	authRequestJSON(t, client, http.MethodGet, srv.URL+"/api/auth/me", nil, http.StatusOK, &me)
	authRequestJSON(t, client, http.MethodGet, srv.URL+"/api/devices", nil, http.StatusOK, nil)
	authRequestJSON(t, client, http.MethodPost, srv.URL+"/api/auth/logout", nil, http.StatusOK, nil)
	authRequestJSON(t, client, http.MethodGet, srv.URL+"/api/devices", nil, http.StatusUnauthorized, nil)
}

func authRequestJSON(t *testing.T, client *http.Client, method, url string, body any, wantStatus int, out any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d: %s", wantStatus, resp.StatusCode, data)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
}

func requestJSON(t *testing.T, method, url, token string, body any, wantStatus int, out any) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d: %s", wantStatus, resp.StatusCode, data)
	}
	if out == nil {
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}

func requestText(t *testing.T, method, url, token string, body any, wantStatus int) string {
	t.Helper()

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("expected status %d, got %d: %s", wantStatus, resp.StatusCode, data)
	}
	return string(data)
}

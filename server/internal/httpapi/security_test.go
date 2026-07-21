package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rmm-openwrt/server/internal/httpapi"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

func TestMultiUserEnrollmentAndDeviceIsolation(t *testing.T) {
	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "security.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{
		OperatorUsername: "admin",
		OperatorPassword: "correct-horse-battery-staple",
		DeviceDomain:     "routers.example.test",
		PublicScheme:     "https",
	}))
	defer srv.Close()

	admin := authenticatedClient(t, srv.URL, "admin", "correct-horse-battery-staple")
	var aliceUser model.User
	authRequestJSON(t, admin, http.MethodPost, srv.URL+"/api/users", map[string]any{
		"username": "alice", "password": "alice-password-long", "role": "user",
	}, http.StatusCreated, &aliceUser)
	var bobUser model.User
	authRequestJSON(t, admin, http.MethodPost, srv.URL+"/api/users", map[string]any{
		"username": "bob", "password": "bob-password-long", "role": "user",
	}, http.StatusCreated, &bobUser)

	alice := authenticatedClient(t, srv.URL, "alice", "alice-password-long")
	bob := authenticatedClient(t, srv.URL, "bob", "bob-password-long")
	authRequestJSON(t, alice, http.MethodGet, srv.URL+"/api/users", nil, http.StatusForbidden, nil)

	aliceDevice := enrollForClient(t, srv.URL, alice, "alice-router", "alice-edge")
	bobDevice := enrollForClient(t, srv.URL, bob, "bob-router", "bob-edge")

	var aliceDevices struct {
		Devices []model.Device `json:"devices"`
	}
	authRequestJSON(t, alice, http.MethodGet, srv.URL+"/api/devices", nil, http.StatusOK, &aliceDevices)
	if len(aliceDevices.Devices) != 1 || aliceDevices.Devices[0].ID != aliceDevice.DeviceID {
		t.Fatalf("alice received devices outside her account: %#v", aliceDevices.Devices)
	}
	if aliceDevices.Devices[0].DomainName != "alice-edge.routers.example.test" {
		t.Fatalf("unexpected device domain: %q", aliceDevices.Devices[0].DomainName)
	}
	authRequestJSON(t, alice, http.MethodGet, srv.URL+"/api/devices/"+bobDevice.DeviceID, nil, http.StatusNotFound, nil)
	authRequestJSON(t, bob, http.MethodPost, srv.URL+"/api/devices/bulk-commands", map[string]any{
		"device_ids": []string{aliceDevice.DeviceID}, "type": "ping", "args": map[string]any{"target": "1.1.1.1"},
	}, http.StatusNotFound, nil)
	authRequestJSON(t, alice, http.MethodPost, srv.URL+"/api/devices/"+aliceDevice.DeviceID+"/transfer", map[string]any{
		"target_username": "bob", "current_password": "alice-password-long",
	}, http.StatusOK, nil)
	authRequestJSON(t, alice, http.MethodGet, srv.URL+"/api/devices/"+aliceDevice.DeviceID, nil, http.StatusNotFound, nil)
	authRequestJSON(t, bob, http.MethodGet, srv.URL+"/api/devices/"+aliceDevice.DeviceID, nil, http.StatusOK, nil)
	authRequestJSON(t, admin, http.MethodPatch, srv.URL+"/api/users/"+aliceUser.ID, map[string]any{
		"role": "admin",
	}, http.StatusOK, nil)
	authRequestJSON(t, admin, http.MethodPatch, srv.URL+"/api/users/"+aliceUser.ID, map[string]any{
		"role": "user",
	}, http.StatusOK, nil)
	alice = authenticatedClient(t, srv.URL, "alice", "alice-password-long")

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/enrollment-grants", jsonBody(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	copyClientCookies(req, alice, srv.URL)
	resp, err := alice.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected cross-origin cookie request to be forbidden, got %d", resp.StatusCode)
	}
	authRequestJSON(t, admin, http.MethodPatch, srv.URL+"/api/users/"+bobUser.ID, map[string]any{
		"disabled": true,
	}, http.StatusOK, nil)
	authRequestJSON(t, bob, http.MethodGet, srv.URL+"/api/devices", nil, http.StatusUnauthorized, nil)
}

type capturedResetMessage struct {
	recipient string
	resetURL  string
}

type captureResetSender struct {
	messages chan capturedResetMessage
}

func (s *captureResetSender) SendPasswordReset(_ context.Context, recipient, resetURL string) error {
	s.messages <- capturedResetMessage{recipient: recipient, resetURL: resetURL}
	return nil
}

func TestPasswordResetIsOneTimeAndRevokesSessions(t *testing.T) {
	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "password-reset.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sender := &captureResetSender{messages: make(chan capturedResetMessage, 1)}
	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{
		OperatorUsername:    "admin",
		OperatorPassword:    "initial-password-123",
		PublicURL:           "https://rmm.example.test",
		PasswordResetSender: sender,
	}))
	defer srv.Close()
	client := authenticatedClient(t, srv.URL, "admin", "initial-password-123")
	authRequestJSON(t, client, http.MethodPatch, srv.URL+"/api/auth/profile", map[string]any{
		"display_name": "Owner", "email": "owner@example.test",
	}, http.StatusOK, nil)
	authRequestJSON(t, client, http.MethodPost, srv.URL+"/api/users", map[string]any{
		"username": "duplicate-email", "email": "owner@example.test", "password": "temporary-password-123", "role": "user",
	}, http.StatusConflict, nil)

	authRequestJSON(t, &http.Client{}, http.MethodPost, srv.URL+"/api/auth/password-reset/request", map[string]any{
		"identifier": "owner@example.test",
	}, http.StatusAccepted, nil)
	var message capturedResetMessage
	select {
	case message = <-sender.messages:
	case <-time.After(2 * time.Second):
		t.Fatal("password reset email was not sent")
	}
	parsed, err := url.Parse(message.resetURL)
	if err != nil {
		t.Fatal(err)
	}
	if message.recipient != "owner@example.test" || parsed.Path != "/login" || !strings.HasPrefix(parsed.Fragment, "password-reset=") {
		t.Fatalf("unexpected password reset message: %#v", message)
	}
	token, err := url.QueryUnescape(strings.TrimPrefix(parsed.Fragment, "password-reset="))
	if err != nil {
		t.Fatal(err)
	}
	authRequestJSON(t, &http.Client{}, http.MethodPost, srv.URL+"/api/auth/password-reset/confirm", map[string]any{
		"token": token, "new_password": "recovered-password-123",
	}, http.StatusOK, nil)
	authRequestJSON(t, client, http.MethodGet, srv.URL+"/api/auth/me", nil, http.StatusUnauthorized, nil)
	authRequestJSON(t, &http.Client{}, http.MethodPost, srv.URL+"/api/auth/password-reset/confirm", map[string]any{
		"token": token, "new_password": "another-password-123",
	}, http.StatusBadRequest, nil)
	authRequestJSON(t, &http.Client{}, http.MethodPost, srv.URL+"/api/auth/login", map[string]any{
		"username": "admin", "password": "initial-password-123",
	}, http.StatusUnauthorized, nil)
	authRequestJSON(t, &http.Client{}, http.MethodPost, srv.URL+"/api/auth/login", map[string]any{
		"username": "admin", "password": "recovered-password-123",
	}, http.StatusOK, nil)
}

func TestDirectDNSRoutesAreRemoved(t *testing.T) {
	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "cloud-only.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{
		OperatorUsername: "admin",
		OperatorPassword: "correct-horse-battery-staple",
	}))
	defer srv.Close()

	requestJSON(t, http.MethodPost, srv.URL+"/api/agent/dns/update", "unused", map[string]any{}, http.StatusNotFound, nil)
	requestJSON(t, http.MethodGet, srv.URL+"/api/internal/dns/records", "unused", nil, http.StatusNotFound, nil)
}

func TestDeviceDomainUsesOneTimeLuCIAccessGrant(t *testing.T) {
	st, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "luci-security.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{
		OperatorUsername: "admin",
		OperatorPassword: "correct-horse-battery-staple",
		DeviceDomain:     "routers.example.test",
		PublicScheme:     "https",
	}))
	defer srv.Close()
	admin := authenticatedClient(t, srv.URL, "admin", "correct-horse-battery-staple")
	device := enrollForClient(t, srv.URL, admin, "office-router", "office")

	remote, found, err := st.CreateRemoteSession(context.Background(), model.RemoteSession{
		DeviceID:   device.DeviceID,
		Target:     "luci",
		Status:     "active",
		LuCIPort:   22101,
		LuCIScheme: "http",
		ExpiresAt:  time.Now().UTC().Add(time.Hour),
	})
	if err != nil || !found {
		t.Fatalf("create remote session: found=%v err=%v", found, err)
	}
	var access struct {
		URL string `json:"url"`
	}
	authRequestJSON(t, admin, http.MethodPost, srv.URL+"/api/devices/"+device.DeviceID+"/remote-sessions/"+remote.ID+"/access", nil, http.StatusCreated, &access)
	parsed, err := url.Parse(access.URL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "office.routers.example.test" || parsed.Query().Get("token") == "" {
		t.Fatalf("unexpected access URL: %s", access.URL)
	}

	noRedirect := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	consume := func(host string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, srv.URL+parsed.RequestURI(), nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = host
		resp, err := noRedirect.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	wrongHost := consume("wrong.routers.example.test")
	wrongHost.Body.Close()
	if wrongHost.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected wrong device host to be rejected, got %d", wrongHost.StatusCode)
	}
	valid := consume(parsed.Host)
	defer valid.Body.Close()
	if valid.StatusCode != http.StatusSeeOther || valid.Header.Get("Location") != "/cgi-bin/luci/" {
		body, _ := io.ReadAll(valid.Body)
		t.Fatalf("expected one-time access redirect, got %d %q: %s", valid.StatusCode, valid.Header.Get("Location"), body)
	}
	var deviceCookie *http.Cookie
	for _, cookie := range valid.Cookies() {
		if cookie.Name == "rmm_device_access" {
			deviceCookie = cookie
		}
	}
	if deviceCookie == nil || deviceCookie.Domain != "" || !deviceCookie.HttpOnly || deviceCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected device access cookie: %#v", deviceCookie)
	}
	if route, ok, err := st.AuthorizeDeviceAccessSession(context.Background(), store.TokenHash(deviceCookie.Value), "office"); err != nil || !ok || route.DeviceID != device.DeviceID {
		t.Fatalf("device access session was not persisted: route=%#v ok=%v err=%v", route, ok, err)
	}

	reused := consume(parsed.Host)
	reused.Body.Close()
	if reused.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected reused access grant to be rejected, got %d", reused.StatusCode)
	}
}

type enrolledCredentials struct {
	DeviceID    string `json:"device_id"`
	DeviceToken string `json:"device_token"`
}

func authenticatedClient(t *testing.T, baseURL, username, password string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	authRequestJSON(t, client, http.MethodPost, baseURL+"/api/auth/login", map[string]any{
		"username": username, "password": password,
	}, http.StatusOK, nil)
	return client
}

func enrollForClient(t *testing.T, baseURL string, client *http.Client, hostname, dnsLabel string) enrolledCredentials {
	t.Helper()
	var grant struct {
		EnrollmentToken string `json:"enrollment_token"`
	}
	authRequestJSON(t, client, http.MethodPost, baseURL+"/api/enrollment-grants", map[string]any{
		"dns_label": dnsLabel,
	}, http.StatusCreated, &grant)
	var enrolled enrolledCredentials
	requestJSON(t, http.MethodPost, baseURL+"/api/agent/enroll", "", map[string]any{
		"enrollment_token": grant.EnrollmentToken,
		"hostname":         hostname,
		"openwrt_version":  "OpenWrt test",
	}, http.StatusCreated, &enrolled)
	requestJSON(t, http.MethodPost, baseURL+"/api/agent/enroll", "", map[string]any{
		"enrollment_token": grant.EnrollmentToken,
		"hostname":         hostname,
		"openwrt_version":  "OpenWrt test",
	}, http.StatusUnauthorized, nil)
	return enrolled
}

func jsonBody(value any) io.Reader {
	data, _ := json.Marshal(value)
	return strings.NewReader(string(data))
}

func copyClientCookies(req *http.Request, client *http.Client, rawURL string) {
	parsed, _ := url.Parse(rawURL)
	for _, cookie := range client.Jar.Cookies(parsed) {
		req.AddCookie(cookie)
	}
}

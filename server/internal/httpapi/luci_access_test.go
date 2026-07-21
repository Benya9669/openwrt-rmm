package httpapi

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"rmm-openwrt/server/internal/model"
)

func TestWriteLuCIErrorRendersSafeBrowserPage(t *testing.T) {
	app := &App{deviceDomain: "routers.example", publicScheme: "https", publicURL: "https://rmm.example"}
	req := httptest.NewRequest(http.MethodGet, "https://router.rmm.example/cgi-bin/luci/?token=must-not-leak", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req = req.WithContext(context.WithValue(req.Context(), requestIDContextKey, "req-test-123"))
	recorder := httptest.NewRecorder()

	app.writeLuCIError(recorder, req, http.StatusGatewayTimeout, "internal proxy details")

	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusGatewayTimeout)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content type = %q, want HTML", got)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"LuCI не отвечает", "req-test-123", `href="/cgi-bin/luci/"`, `href="https://rmm.example/"`} {
		if !strings.Contains(body, expected) {
			t.Errorf("response does not contain %q", expected)
		}
	}
	for _, unexpected := range []string{"must-not-leak", "internal proxy details"} {
		if strings.Contains(body, unexpected) {
			t.Errorf("response leaked %q", unexpected)
		}
	}
}

func TestRemoteSessionAccessStateChecksTunnelPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	now := time.Now().UTC()
	app := &App{tunnelHTTPHost: "127.0.0.1"}
	session := model.RemoteSession{Status: "active", LuCIPort: port, StartedAt: &now, ExpiresAt: now.Add(time.Minute)}
	if got := app.remoteSessionAccessState(session); got != "ready" {
		t.Fatalf("access state = %q, want ready", got)
	}
	_ = listener.Close()
	if got := app.remoteSessionAccessState(session); got != "starting" {
		t.Fatalf("fresh failed tunnel state = %q, want starting", got)
	}
	old := now.Add(-time.Minute)
	session.StartedAt = &old
	if got := app.remoteSessionAccessState(session); got != "unavailable" {
		t.Fatalf("stale failed tunnel state = %q, want unavailable", got)
	}
}

func TestWriteLuCIErrorKeepsJSONForAPIClients(t *testing.T) {
	app := &App{deviceDomain: "rmm.example", publicScheme: "https"}
	req := httptest.NewRequest(http.MethodPost, "/api/devices/device-1/access", nil)
	req.Header.Set("Accept", "application/json")
	recorder := httptest.NewRecorder()

	app.writeLuCIError(recorder, req, http.StatusConflict, "LuCI session is not active")

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
	if !strings.Contains(recorder.Body.String(), `"error":"LuCI session is not active"`) {
		t.Fatalf("unexpected JSON response: %s", recorder.Body.String())
	}
}

func TestLuCIErrorCopyCoversUserFacingFailures(t *testing.T) {
	for _, status := range []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusConflict,
		http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusGatewayTimeout,
		http.StatusInternalServerError,
	} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			title, description, action := luciErrorCopy(status)
			if title == "" || description == "" || action == "" {
				t.Fatalf("status %d has incomplete UI copy", status)
			}
		})
	}
}

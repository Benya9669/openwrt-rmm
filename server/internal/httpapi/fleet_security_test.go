package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"rmm-openwrt/server/internal/httpapi"
	"rmm-openwrt/server/internal/store"
	"testing"
	"time"
)

func TestFleetHTTPKeepsTenantAdminAndOriginBoundaries(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenSQLite(ctx, filepath.Join(t.TempDir(), "fleet-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	admin, err := st.EnsureBootstrapUser(ctx, "fleet-admin", "test-hash")
	if err != nil {
		t.Fatal(err)
	}
	owner, err := st.CreateUser(ctx, "fleet-owner", "Owner", "", "test-hash", "user")
	if err != nil {
		t.Fatal(err)
	}
	first, err := st.EnrollDevice(ctx, "owned", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := st.EnrollDevice(ctx, "foreign", "24.10")
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := st.TransferDevice(ctx, first.DeviceID, owner.ID, admin.ID, true); err != nil || !found {
		t.Fatal("fixture transfer failed")
	}
	for raw, id := range map[string]string{"fleet-owner-cookie": owner.ID, "fleet-admin-cookie": admin.ID} {
		if err = st.CreateOperatorSession(ctx, store.TokenHash(raw), id, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(httpapi.NewHandler(st, httpapi.Config{OperatorToken: "fleet-admin-token", OperatorUsername: "fleet-admin"}))
	defer srv.Close()
	call := func(method, path, cookie, origin string, payload any, status int) {
		t.Helper()
		var body []byte
		if payload != nil {
			body, _ = json.Marshal(payload)
		}
		req, _ := http.NewRequest(method, srv.URL+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: "rmm_operator_session", Value: cookie})
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != status {
			t.Fatalf("%s %s: got %d want %d", method, path, response.StatusCode, status)
		}
	}
	for _, path := range []string{"/api/fleet/permissions", "/api/fleet/incidents", "/api/fleet/topology", "/api/fleet/operations", "/api/fleet/profiles", "/api/fleet/schedules", "/api/fleet/rules", "/api/fleet/assets", "/api/fleet/history/" + first.DeviceID, "/api/fleet/access-policies/" + first.DeviceID} {
		call("GET", path, "", "", nil, http.StatusUnauthorized)
	}
	call("GET", "/api/fleet/history/"+foreign.DeviceID, "fleet-owner-cookie", "", nil, http.StatusNotFound)
	call("PUT", "/api/fleet/assets/"+foreign.DeviceID, "fleet-owner-cookie", "", map[string]string{"model": "forbidden"}, http.StatusNotFound)
	operation := map[string]any{"request_key": "foreign-fleet-request", "type": "ping", "device_ids": []string{first.DeviceID, foreign.DeviceID}, "parallelism": 1}
	call("POST", "/api/fleet/operations", "fleet-owner-cookie", "", operation, http.StatusNotFound)
	operation["device_ids"] = []string{first.DeviceID}
	call("POST", "/api/fleet/operations", "fleet-owner-cookie", "https://foreign.example.test", operation, http.StatusForbidden)
	call("POST", "/api/fleet/operations", "fleet-owner-cookie", srv.URL, operation, http.StatusAccepted)
	policy := map[string]any{"ssh_allowed": true, "luci_allowed": false, "max_ttl_seconds": 60, "restrict_users": true, "user_ids": []string{admin.ID}}
	call("PUT", "/api/fleet/access-policies/"+first.DeviceID, "fleet-owner-cookie", "", policy, http.StatusForbidden)
	call("PUT", "/api/fleet/access-policies/"+first.DeviceID, "fleet-admin-cookie", "", policy, http.StatusOK)
	call("POST", "/api/devices/"+first.DeviceID+"/remote-sessions", "fleet-owner-cookie", "", map[string]any{"target": "ssh", "server_host": "tunnel.example.test", "duration_seconds": 60}, http.StatusForbidden)
	call("POST", "/api/devices/"+first.DeviceID+"/commands", "fleet-owner-cookie", "", map[string]any{"type": "remote_ssh_reverse", "args": map[string]string{}}, http.StatusBadRequest)
	call("GET", "/api/devices/"+foreign.DeviceID+"/backups/bkp-test/compare?against=bkp-other", "fleet-owner-cookie", "", nil, http.StatusNotFound)
	if err = st.SavePermissionPolicy(ctx, admin.ID, owner.ID, store.PermissionPolicy{Permissions: []string{"view", "diagnostics"}}); err != nil {
		t.Fatal(err)
	}
	call("POST", "/api/devices/"+first.DeviceID+"/commands", "fleet-owner-cookie", "", map[string]any{"type": "uci_commit", "args": map[string]string{}}, http.StatusForbidden)
	call("POST", "/api/devices/bulk-commands", "fleet-owner-cookie", "", map[string]any{"type": "ping", "device_ids": []string{first.DeviceID}, "args": map[string]string{"target": "127.0.0.1"}}, http.StatusCreated)
	call("POST", "/api/devices/bulk-commands", "fleet-owner-cookie", "", map[string]any{"type": "uci_commit", "device_ids": []string{first.DeviceID}, "args": map[string]string{}}, http.StatusForbidden)
	call("PUT", "/api/fleet/permissions/"+owner.ID, "fleet-owner-cookie", "", map[string]any{"permissions": []string{"view", "uci"}}, http.StatusForbidden)
	call("GET", "/api/fleet/topology", "fleet-owner-cookie", "", nil, http.StatusOK)

	call("GET", "/api/devices/"+first.DeviceID+"/backups", "fleet-owner-cookie", "", nil, http.StatusForbidden)
	call("GET", "/api/fleet/profiles", "fleet-owner-cookie", "", nil, http.StatusForbidden)

}

package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

func (a *App) handlePermissions(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	id := r.PathValue("id")
	if id == "" {
		id = principal.User.ID
	}
	if id != principal.User.ID && !principal.IsAdmin() {
		writeError(w, 403, "administrator required")
		return
	}
	if r.Method == http.MethodGet {
		p, err := a.store.GetPermissionPolicy(r.Context(), id)
		if err != nil {
			writeError(w, 500, "failed to load permissions")
			return
		}
		writeJSON(w, 200, p)
		return
	}
	var p store.PermissionPolicy
	if !decodeJSON(w, r, &p) {
		return
	}
	if err := a.store.SavePermissionPolicy(r.Context(), principal.User.ID, id, p); err != nil {
		writeError(w, 400, "invalid permissions, scope or target user")
		return
	}
	writeJSON(w, 200, p)
}

// Check permissions centrally for legacy and fleet routes. Ownership checks
// remain in their existing handlers and store transactions.
func (a *App) authorizePermissionRequest(w http.ResponseWriter, r *http.Request, p authPrincipal) bool {
	if p.IsAdmin() {
		return true
	}
	path := r.URL.Path
	if path == "/api/fleet/permissions" && r.Method == http.MethodGet {
		return true
	}
	if !(strings.HasPrefix(path, "/api/devices") || strings.HasPrefix(path, "/api/fleet") || strings.HasPrefix(path, "/luci/") || strings.HasPrefix(path, "/api/commands")) {
		return true
	}
	permission := "view"
	if unsafeMethod(r.Method) {
		permission = "maintenance"
		if strings.Contains(path, "/operations/") {
			permission = "view"
		}
		switch {
		case strings.Contains(path, "/incidents"):
			permission = "incidents"
		case strings.Contains(path, "/profiles") || strings.Contains(path, "/profile-operations") || strings.Contains(path, "/uci"):
			permission = "uci"
		case strings.Contains(path, "remote") || strings.Contains(path, "access-grant"):
			permission = "remote"
		case strings.Contains(path, "backup"):
			permission = "backups"
		case strings.Contains(path, "agent-update") || strings.Contains(path, "agent-rollback"):
			permission = "updates"
		}
		if strings.Contains(path, "commands") || strings.HasSuffix(path, "/operations") || strings.Contains(path, "/schedules") || strings.Contains(path, "/rules") {
			raw, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024+1))
			if err != nil || len(raw) > 1024*1024 {
				writeError(w, 400, "invalid request body")
				return false
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
			var body struct {
				Type      string `json:"type"`
				Action    string `json:"action"`
				Operation struct {
					Type string `json:"type"`
				} `json:"operation"`
			}
			if json.Unmarshal(raw, &body) == nil {
				if body.Operation.Type != "" {
					body.Type = body.Operation.Type
				}
				if body.Type != "" {
					permission = store.CommandPermission(body.Type)
				} else if strings.Contains(path, "/rules") && body.Action == "notify" {
					permission = "incidents"
				}
			}
		}
	}
	if strings.Contains(path, "/backups") {
		permission = "backups"
	}
	if strings.Contains(path, "/fleet/profiles") || strings.Contains(path, "/profile-operations") {
		permission = "uci"
	}
	if strings.Contains(path, "remote-sessions") {
		permission = "remote"
	}
	if strings.HasPrefix(path, "/luci/") {
		permission = "remote"
	}
	deviceID := ""
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "devices" && parts[2] != "bulk-commands" {
		deviceID = parts[2]
	}
	if len(parts) >= 2 && parts[0] == "luci" {
		deviceID = parts[1]
	}
	if deviceID != "" {
		if err := a.store.CheckPermission(r.Context(), p.User.ID, "", permission); err != nil {
			if errors.Is(err, store.ErrFleetAccess) {
				writeError(w, 403, "operation denied by permissions")
			} else {
				writeError(w, 500, "failed to check permissions")
			}
			return false
		}
	}
	if err := a.store.CheckPermission(r.Context(), p.User.ID, deviceID, permission); err != nil {
		if errors.Is(err, store.ErrFleetAccess) {
			if deviceID != "" {
				writeError(w, 404, "device unavailable")
			} else {
				writeError(w, 403, "operation denied by permissions")
			}
		} else {
			writeError(w, 500, "failed to check permissions")
		}
		return false
	}
	return true
}

func (a *App) createOperatorCommand(r *http.Request, deviceID, kind string, args json.RawMessage) (model.Command, bool, error) {
	p, _ := principalFromContext(r.Context())
	return a.store.CreateAuthorizedCommand(r.Context(), p.User.ID, deviceID, kind, args)
}

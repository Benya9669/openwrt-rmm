package httpapi

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"rmm-openwrt/server/internal/authn"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

var (
	usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,64}$`)
	dnsLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
)

func (a *App) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if req.Role == "" {
		req.Role = "user"
	}
	if !usernamePattern.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "username must be 3-64 characters and contain only letters, digits, '.', '_' or '-'")
		return
	}
	if req.Role != "user" && req.Role != "admin" {
		writeError(w, http.StatusBadRequest, "role must be user or admin")
		return
	}
	passwordHash, err := authn.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := a.store.CreateUser(r.Context(), req.Username, passwordHash, req.Role)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, "username is already in use")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	principal, _ := principalFromContext(r.Context())
	_, _ = a.store.AddAuditEvent(r.Context(), principal.User.Username, "user.create", "", "", mustJSON(map[string]any{
		"created_user_id":  user.ID,
		"created_username": user.Username,
		"role":             user.Role,
		"request_id":       requestID(r.Context()),
	}))
	writeJSON(w, http.StatusCreated, user)
}

func (a *App) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "api" || parts[1] != "users" || parts[2] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var req updateUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Disabled == nil && req.Password == "" {
		writeError(w, http.StatusBadRequest, "disabled or password is required")
		return
	}
	principal, _ := principalFromContext(r.Context())
	if parts[2] == principal.User.ID && req.Disabled != nil && *req.Disabled {
		writeError(w, http.StatusBadRequest, "you cannot disable your current account")
		return
	}
	passwordHash := ""
	var err error
	if req.Password != "" {
		passwordHash, err = authn.HashPassword(req.Password)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	user, found, err := a.store.UpdateUserSecurity(r.Context(), parts[2], req.Disabled, passwordHash)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "last active") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), principal.User.Username, "user.security_update", "", "", mustJSON(map[string]any{
		"updated_user_id": user.ID,
		"disabled":        req.Disabled,
		"password_reset":  passwordHash != "",
		"request_id":      requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, user)
}

func (a *App) handleCreateEnrollmentGrant(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var req enrollmentGrantRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	label := strings.ToLower(strings.TrimSpace(req.DNSLabel))
	if label != "" && !dnsLabelPattern.MatchString(label) {
		writeError(w, http.StatusBadRequest, "dns_label must be a valid single DNS label")
		return
	}
	duration := 15 * time.Minute
	if req.ExpiresSeconds != 0 {
		if req.ExpiresSeconds < 60 || req.ExpiresSeconds > 24*60*60 {
			writeError(w, http.StatusBadRequest, "expires_seconds must be between 60 and 86400")
			return
		}
		duration = time.Duration(req.ExpiresSeconds) * time.Second
	}
	token, err := randomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create enrollment grant")
		return
	}
	principal, _ := principalFromContext(r.Context())
	grant, err := a.store.CreateEnrollmentGrant(r.Context(), principal.User.ID, label, store.TokenHash(token), time.Now().UTC().Add(duration))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "limit") {
			writeError(w, http.StatusTooManyRequests, "active enrollment grant limit reached")
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "already") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, "dns label is already reserved")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create enrollment grant")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), principal.User.Username, "enrollment_grant.create", "", "", mustJSON(map[string]any{
		"grant_id":   grant.ID,
		"dns_label":  label,
		"expires_at": grant.ExpiresAt,
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusCreated, map[string]any{
		"grant":            grant,
		"enrollment_token": token,
	})
}

func (a *App) handleUpdateDeviceDNS(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var req dnsLabelRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	label := strings.ToLower(strings.TrimSpace(req.DNSLabel))
	if !dnsLabelPattern.MatchString(label) {
		writeError(w, http.StatusBadRequest, "dns_label must be a valid single DNS label")
		return
	}
	device, found, err := a.store.UpdateDeviceDNSLabel(r.Context(), parts[2], label)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "reserved") {
			writeError(w, http.StatusConflict, "dns label is already in use")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update dns label")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	a.decorateDevice(&device)
	principal, _ := principalFromContext(r.Context())
	_, _ = a.store.AddAuditEvent(r.Context(), principal.User.Username, "device.dns_update", device.ID, "", mustJSON(map[string]string{
		"dns_label":  label,
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, device)
}

func (a *App) decorateDevices(devices []model.Device) {
	for index := range devices {
		a.decorateDevice(&devices[index])
	}
}

func (a *App) decorateDevice(device *model.Device) {
	if a.deviceDomain != "" && device.DNSLabel != "" {
		device.DomainName = device.DNSLabel + "." + a.deviceDomain
	}
}

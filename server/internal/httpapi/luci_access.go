package httpapi

import (
	"net/http"
	"strings"
	"time"

	"rmm-openwrt/server/internal/store"
)

const (
	deviceAccessGrantTTL   = 60 * time.Second
	deviceAccessSessionTTL = 2 * time.Hour
)

func (a *App) handleCreateDeviceAccess(w http.ResponseWriter, r *http.Request) {
	if a.deviceDomain == "" {
		writeError(w, http.StatusConflict, "device domain is not configured")
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 6 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	deviceID, remoteSessionID := parts[2], parts[4]
	device, found, err := a.store.GetDevice(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load device")
		return
	}
	if !found || device.DNSLabel == "" {
		writeError(w, http.StatusNotFound, "device DNS name is unavailable")
		return
	}
	session, found, err := a.store.GetRemoteSession(r.Context(), deviceID, remoteSessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load remote session")
		return
	}
	if !found || session.Status != "active" || session.LuCIPort <= 0 || !session.ExpiresAt.After(time.Now().UTC()) {
		writeError(w, http.StatusConflict, "LuCI remote session is not active")
		return
	}
	rawToken, err := randomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create LuCI access grant")
		return
	}
	principal, _ := principalFromContext(r.Context())
	expiresAt := time.Now().UTC().Add(deviceAccessGrantTTL)
	if err := a.store.CreateDeviceAccessGrant(r.Context(), store.TokenHash(rawToken), principal.User.ID, deviceID, remoteSessionID, expiresAt); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "limit") {
			writeError(w, http.StatusTooManyRequests, "active LuCI access grant limit reached")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create LuCI access grant")
		return
	}
	domainName := device.DNSLabel + "." + a.deviceDomain
	accessURL := a.publicScheme + "://" + domainName + "/_rmm/access?token=" + rawToken
	_, _ = a.store.AddAuditEvent(r.Context(), principal.User.Username, "luci.access_grant", deviceID, "", mustJSON(map[string]any{
		"remote_session_id": remoteSessionID,
		"expires_at":        expiresAt,
		"request_id":        requestID(r.Context()),
	}))
	writeJSON(w, http.StatusCreated, map[string]any{"url": accessURL, "expires_at": expiresAt})
}

func (a *App) routeByHost(control http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if label, ok := a.deviceLabelFromHost(r.Host); ok {
			a.handleDeviceHost(w, r, label)
			return
		}
		control.ServeHTTP(w, r)
	})
}

func (a *App) deviceLabelFromHost(host string) (string, bool) {
	if a.deviceDomain == "" {
		return "", false
	}
	host = strings.ToLower(hostWithoutPort(host))
	suffix := "." + a.deviceDomain
	if !strings.HasSuffix(host, suffix) {
		return "", false
	}
	label := strings.TrimSuffix(host, suffix)
	return label, dnsLabelPattern.MatchString(label)
}

func (a *App) handleDeviceHost(w http.ResponseWriter, r *http.Request, dnsLabel string) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/_rmm/access" {
		a.consumeDeviceAccess(w, r, dnsLabel)
		return
	}
	cookie, err := r.Cookie(deviceAccessCookie)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "LuCI access grant is required")
		return
	}
	route, found, err := a.store.AuthorizeDeviceAccessSession(r.Context(), store.TokenHash(cookie.Value), dnsLabel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to authorize LuCI access")
		return
	}
	if !found {
		writeError(w, http.StatusUnauthorized, "LuCI access session has expired")
		return
	}
	if unsafeMethod(r.Method) && !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	a.proxyLuCIWithPrefix(w, r, route.DeviceID, route.RemoteSessionID, r.URL.Path, "")
}

func (a *App) consumeDeviceAccess(w http.ResponseWriter, r *http.Request, dnsLabel string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rawGrant := strings.TrimSpace(r.URL.Query().Get("token"))
	if rawGrant == "" {
		writeError(w, http.StatusBadRequest, "access token is required")
		return
	}
	rawSession, err := randomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create LuCI access session")
		return
	}
	route, found, err := a.store.ConsumeDeviceAccessGrant(r.Context(), store.TokenHash(rawGrant), store.TokenHash(rawSession), dnsLabel, time.Now().UTC().Add(deviceAccessSessionTTL))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to consume LuCI access grant")
		return
	}
	if !found || route.DNSLabel != dnsLabel {
		writeError(w, http.StatusUnauthorized, "invalid or expired LuCI access grant")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     deviceAccessCookie,
		Value:    rawSession,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		Expires:  route.ExpiresAt,
		MaxAge:   int(time.Until(route.ExpiresAt).Seconds()),
	})
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Location", "/cgi-bin/luci/")
	w.WriteHeader(http.StatusSeeOther)
}

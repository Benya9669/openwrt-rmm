package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"

	"rmm-openwrt/server/internal/authn"
	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

const (
	operatorSessionCookie = "rmm_operator_session"
	operatorSessionTTL    = 12 * time.Hour
)

const principalContextKey contextKey = "auth_principal"

type authPrincipal struct {
	User        model.User
	Method      string
	SessionHash string
}

func (p authPrincipal) IsAdmin() bool { return p.User.Role == "admin" }

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	select {
	case a.loginSlots <- struct{}{}:
		defer func() { <-a.loginSlots }()
	default:
		w.Header().Set("Retry-After", "2")
		writeError(w, http.StatusTooManyRequests, "authentication service is busy")
		return
	}
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	key := clientIP(r) + "|" + strings.ToLower(strings.TrimSpace(req.Username))
	if !a.loginLimiter.Allow(key) {
		w.Header().Set("Retry-After", "300")
		writeError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	if len(req.Username) > 64 || len(req.Password) > 1024 {
		a.loginLimiter.Fail(key)
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	user, passwordHash, found, err := a.store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to authenticate")
		return
	}
	if !found {
		passwordHash = a.dummyPasswordHash
	}
	passwordOK := authn.VerifyPassword(passwordHash, req.Password)
	if !found || user.Disabled || !passwordOK {
		a.loginLimiter.Fail(key)
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	token, err := randomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	expiresAt := time.Now().UTC().Add(operatorSessionTTL)
	if err := a.store.CreateOperatorSession(r.Context(), store.TokenHash(token), user.ID, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	a.loginLimiter.Success(key)
	http.SetCookie(w, &http.Cookie{
		Name:     operatorSessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
		MaxAge:   int(operatorSessionTTL.Seconds()),
	})
	_, _ = a.store.AddAuditEvent(r.Context(), user.Username, "auth.login", "", "", mustJSON(map[string]string{
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, map[string]any{"username": user.Username, "user": user, "expires_at": expiresAt})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	if principal.SessionHash != "" {
		_ = a.store.RevokeOperatorSession(r.Context(), principal.SessionHash)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     operatorSessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	_, _ = a.store.AddAuditEvent(r.Context(), principal.User.Username, "auth.logout", "", "", mustJSON(map[string]string{
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"username": principal.User.Username, "user": principal.User})
}

func (a *App) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req updateProfileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if len([]rune(req.DisplayName)) > 80 {
		writeError(w, http.StatusBadRequest, "display_name must not exceed 80 characters")
		return
	}
	if len(req.Email) > 254 {
		writeError(w, http.StatusBadRequest, "email must not exceed 254 characters")
		return
	}
	if req.Email != "" {
		address, err := mail.ParseAddress(req.Email)
		if err != nil || !strings.EqualFold(address.Address, req.Email) {
			writeError(w, http.StatusBadRequest, "email is invalid")
			return
		}
	}
	principal, _ := principalFromContext(r.Context())
	user, found, err := a.store.UpdateUserProfile(r.Context(), principal.User.ID, req.DisplayName, req.Email)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "email is already in use") {
			writeError(w, http.StatusConflict, "email is already in use")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), user.Username, "auth.profile_update", "", "", mustJSON(map[string]string{
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (a *App) handlePasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	if a.passwordResetSender == nil || a.publicURL == "" {
		writeError(w, http.StatusServiceUnavailable, "password recovery is not configured")
		return
	}
	var req passwordResetRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" || len(identifier) > 254 {
		writeError(w, http.StatusBadRequest, "username or email is required")
		return
	}
	limitKey := strings.ToLower(identifier)
	if !a.passwordResetLimiter.Allow(limitKey) {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, "too many password recovery attempts")
		return
	}
	a.passwordResetLimiter.Fail(limitKey)
	user, found, err := a.store.GetUserForPasswordReset(r.Context(), identifier)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start password recovery")
		return
	}
	if found {
		token, err := randomToken(32)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to start password recovery")
			return
		}
		if err := a.store.CreatePasswordReset(r.Context(), user.ID, store.TokenHash(token), time.Now().UTC().Add(30*time.Minute)); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to start password recovery")
			return
		}
		resetURL := a.publicURL + "/#password-reset=" + url.QueryEscape(token)
		go func(userID, recipient, targetURL string) {
			if err := a.passwordResetSender.SendPasswordReset(context.Background(), recipient, targetURL); err != nil {
				log.Printf("password reset email delivery failed for user %s: %v", userID, err)
			}
		}(user.ID, user.Email, resetURL)
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "accepted",
		"message": "if the account has a recovery email, a reset link will be sent",
	})
}

func (a *App) handlePasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	var req passwordResetConfirmRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Token) < 32 || len(req.Token) > 512 {
		writeError(w, http.StatusBadRequest, "password reset link is invalid or expired")
		return
	}
	passwordHash, err := authn.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	reset, err := a.store.ResetPassword(r.Context(), store.TokenHash(req.Token), passwordHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reset password")
		return
	}
	if !reset {
		writeError(w, http.StatusBadRequest, "password reset link is invalid or expired")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), "password-reset", "auth.password_reset", "", "", mustJSON(map[string]string{
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	principal, _ := principalFromContext(r.Context())
	_, currentHash, found, err := a.store.GetUserByUsername(r.Context(), principal.User.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify password")
		return
	}
	if !found || !authn.VerifyPassword(currentHash, req.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}
	if req.CurrentPassword == req.NewPassword {
		writeError(w, http.StatusBadRequest, "new password must be different")
		return
	}
	passwordHash, err := authn.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.store.UpdateOwnPassword(r.Context(), principal.User.ID, passwordHash, principal.SessionHash); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to change password")
		return
	}
	_, _ = a.store.AddAuditEvent(r.Context(), principal.User.Username, "auth.password_change", "", "", mustJSON(map[string]string{
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	principal, _ := principalFromContext(r.Context())
	if err := a.store.RevokeUserSessions(r.Context(), principal.User.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke sessions")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: operatorSessionCookie, Value: "", Path: "/", HttpOnly: true,
		Secure: a.cookieSecure, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(0, 0),
	})
	_, _ = a.store.AddAuditEvent(r.Context(), principal.User.Username, "auth.logout_all", "", "", mustJSON(map[string]string{
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) authenticateOperator(r *http.Request) (authPrincipal, bool) {
	if token, ok := bearerToken(r); ok && a.operatorToken != "" && constantTimeEqual(token, a.operatorToken) {
		user, _, found, err := a.store.GetUserByUsername(r.Context(), a.operatorUsername)
		if err == nil && found && !user.Disabled {
			return authPrincipal{User: user, Method: "bearer"}, true
		}
		return authPrincipal{}, false
	}
	cookie, err := r.Cookie(operatorSessionCookie)
	if err != nil || cookie.Value == "" {
		return authPrincipal{}, false
	}
	hash := store.TokenHash(cookie.Value)
	user, found, err := a.store.AuthorizeOperatorSession(r.Context(), hash)
	if err != nil || !found {
		return authPrincipal{}, false
	}
	return authPrincipal{User: user, Method: "cookie", SessionHash: hash}, true
}

func (a *App) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := principalFromContext(r.Context())
		if !ok || !principal.IsAdmin() {
			writeError(w, http.StatusForbidden, "administrator access is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func principalFromContext(ctx context.Context) (authPrincipal, bool) {
	principal, ok := ctx.Value(principalContextKey).(authPrincipal)
	return principal, ok
}

func constantTimeEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func randomToken(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}
	return strings.EqualFold(parsed.Scheme, scheme) && strings.EqualFold(parsed.Host, r.Host)
}

func unsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

type loginAttempt struct {
	count     int
	resetTime time.Time
}

type loginRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	attempts map[string]loginAttempt
}

func newLoginRateLimiter(limit int, window time.Duration) *loginRateLimiter {
	return &loginRateLimiter{limit: limit, window: window, attempts: make(map[string]loginAttempt)}
}

func (l *loginRateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	attempt, ok := l.attempts[key]
	if ok && now.After(attempt.resetTime) {
		delete(l.attempts, key)
		return true
	}
	return !ok || attempt.count < l.limit
}

func (l *loginRateLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.attempts) > 1024 {
		for candidate, value := range l.attempts {
			if now.After(value.resetTime) {
				delete(l.attempts, candidate)
			}
		}
	}
	attempt, ok := l.attempts[key]
	if !ok || now.After(attempt.resetTime) {
		attempt = loginAttempt{resetTime: now.Add(l.window)}
	}
	attempt.count++
	l.attempts[key] = attempt
}

func (l *loginRateLimiter) Success(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

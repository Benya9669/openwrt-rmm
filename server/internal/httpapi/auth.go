package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	operatorSessionCookie = "rmm_operator_session"
	operatorSessionTTL    = 12 * time.Hour
)

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !constantTimeEqual(strings.TrimSpace(req.Username), a.operatorUsername) || !constantTimeEqual(req.Password, a.operatorPassword) {
		time.Sleep(250 * time.Millisecond)
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	token, expiresAt, err := a.newOperatorSession(req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     operatorSessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(operatorSessionTTL.Seconds()),
	})
	_, _ = a.store.AddAuditEvent(r.Context(), req.Username, "auth.login", "", "", mustJSON(map[string]string{
		"request_id": requestID(r.Context()),
	}))
	writeJSON(w, http.StatusOK, map[string]any{"username": req.Username, "expires_at": expiresAt})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	username, _ := a.operatorSessionUsername(r)
	http.SetCookie(w, &http.Cookie{
		Name:     operatorSessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	if username != "" {
		_, _ = a.store.AddAuditEvent(r.Context(), username, "auth.logout", "", "", mustJSON(map[string]string{
			"request_id": requestID(r.Context()),
		}))
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	username, ok := a.operatorSessionUsername(r)
	if !ok {
		username = a.operatorUsername
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": username})
}

func (a *App) newOperatorSession(username string) (string, time.Time, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().UTC().Add(operatorSessionTTL)
	payload := strings.Join([]string{
		username,
		strconv.FormatInt(expiresAt.Unix(), 10),
		base64.RawURLEncoding.EncodeToString(nonce[:]),
	}, "|")
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signature := a.signSessionPayload(encodedPayload)
	return encodedPayload + "." + signature, expiresAt, nil
}

func (a *App) operatorSessionUsername(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(operatorSessionCookie)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	encodedPayload, signature, ok := strings.Cut(cookie.Value, ".")
	if !ok || !hmac.Equal([]byte(signature), []byte(a.signSessionPayload(encodedPayload))) {
		return "", false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return "", false
	}
	parts := strings.Split(string(payloadBytes), "|")
	if len(parts) != 3 || !constantTimeEqual(parts[0], a.operatorUsername) {
		return "", false
	}
	expiresUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().UTC().Unix() >= expiresUnix {
		return "", false
	}
	return parts[0], true
}

func (a *App) signSessionPayload(payload string) string {
	mac := hmac.New(sha256.New, a.sessionSecret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func constantTimeEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func (a *App) operatorAuthorized(r *http.Request) bool {
	if token, ok := bearerToken(r); ok && constantTimeEqual(token, a.operatorToken) {
		return true
	}
	_, ok := a.operatorSessionUsername(r)
	return ok
}

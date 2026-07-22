package httpapi

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"rmm-openwrt/server/internal/model"
	"rmm-openwrt/server/internal/store"
)

const (
	deviceAccessGrantTTL   = 60 * time.Second
	deviceAccessSessionTTL = 2 * time.Hour
)

type luciErrorPageData struct {
	Code        string
	Title       string
	Description string
	ActionLabel string
	ActionURL   string
	ControlURL  string
	RequestID   string
}

var luciErrorPageTemplate = template.Must(template.New("luci-error").Parse(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} · OpenWrt RMM</title>
  <style>
    :root { color-scheme: dark; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; --bg: #14191f; --surface: #1e252d; --raised: #242d36; --text: #f4f7fa; --muted: #9baaba; --line: #3a4652; --accent: #21a8d8; --warn: #f3b43f; }
    * { box-sizing: border-box; }
    body { min-height: 100dvh; margin: 0; padding: clamp(16px, 4vw, 40px); display: grid; place-items: center; color: var(--text); background: radial-gradient(circle at 16% 8%, rgb(39 182 230 / 15%), transparent 30rem), radial-gradient(circle at 88% 78%, rgb(67 96 246 / 10%), transparent 32rem), var(--bg); }
    main { width: min(100%, 760px); overflow: hidden; border: 1px solid var(--line); border-radius: 24px; background: rgb(30 37 45 / 96%); box-shadow: 0 28px 90px rgb(0 0 0 / 34%); animation: enter 260ms cubic-bezier(.2, .75, .25, 1) both; }
    header { min-height: 72px; padding: 16px clamp(20px, 5vw, 38px); display: flex; align-items: center; justify-content: space-between; gap: 18px; border-bottom: 1px solid var(--line); }
    .brand { display: flex; align-items: center; gap: 11px; color: var(--text); font-size: 14px; font-weight: 760; }
    .brand-mark { width: 36px; height: 36px; display: grid; place-items: center; border-radius: 11px; color: #fff; background: linear-gradient(145deg, #27b6e6, #435ff6); box-shadow: 0 8px 24px rgb(33 168 216 / 22%); }
    .route { display: flex; align-items: center; gap: 7px; color: var(--muted); font: 11px ui-monospace, SFMono-Regular, Consolas, monospace; }
    .route i { width: 4px; height: 4px; border-radius: 50%; background: var(--accent); box-shadow: 0 0 0 3px rgb(33 168 216 / 10%); }
    .content { padding: clamp(26px, 6vw, 50px); }
    .signal { width: 86px; height: 50px; display: flex; align-items: center; gap: 8px; margin-bottom: 30px; }
    .signal span { width: 24px; height: 24px; display: grid; place-items: center; border: 1px solid var(--line); border-radius: 8px; color: var(--muted); background: var(--raised); font-size: 11px; font-weight: 800; }
    .signal b { width: 20px; height: 1px; position: relative; background: var(--line); }
    .signal b::after { content: ""; position: absolute; top: -3px; left: 8px; width: 7px; height: 7px; border-radius: 50%; background: var(--warn); box-shadow: 0 0 0 4px rgb(243 180 63 / 10%); }
    .code { margin: 0 0 12px; color: var(--warn); font: 750 12px ui-monospace, SFMono-Regular, Consolas, monospace; letter-spacing: .08em; }
    h1 { max-width: 620px; margin: 0; font-size: clamp(30px, 7vw, 48px); line-height: 1.04; letter-spacing: -.045em; }
    p { max-width: 640px; margin: 18px 0 0; color: var(--muted); font-size: 16px; line-height: 1.65; }
    .notice { margin-top: 28px; padding: 16px 18px; border: 1px solid var(--line); border-radius: 14px; color: #c7d1da; background: var(--raised); font-size: 14px; line-height: 1.5; }
    .actions { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 32px; }
    a { min-height: 46px; padding: 0 18px; display: inline-flex; align-items: center; justify-content: center; border: 1px solid var(--line); border-radius: 12px; color: var(--text); text-decoration: none; font-weight: 750; transition: border-color 160ms ease, background-color 160ms ease, transform 160ms ease; }
    a.primary { border-color: var(--accent); color: #fff; background: var(--accent); }
    .request { margin-top: 26px; color: #718091; font: 11px ui-monospace, SFMono-Regular, Consolas, monospace; overflow-wrap: anywhere; }
    @keyframes enter { from { opacity: 0; transform: translateY(14px) scale(.985); } to { opacity: 1; transform: translateY(0) scale(1); } }
    @media (hover: hover) and (pointer: fine) { a:hover { border-color: rgb(33 168 216 / 64%); transform: translateY(-1px); } a.primary:hover { background: #29b6e7; } }
    @media (max-width: 560px) { body { padding: 0; align-items: end; } main { border-width: 1px 0 0; border-radius: 24px 24px 0 0; } header { min-height: 64px; padding: 14px 20px; } .route { display: none; } .content { padding: 28px 20px max(24px, env(safe-area-inset-bottom)); } .signal { margin-bottom: 24px; } .actions { display: grid; } .actions a { width: 100%; } }
    @media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation-duration: .01ms !important; transition-duration: .01ms !important; } }
  </style>
</head>
<body>
  <main>
    <header>
      <div class="brand"><span class="brand-mark">R</span> OpenWrt RMM</div>
      <div class="route"><span>RMM</span><i></i><span>облако</span><i></i><span>LuCI</span></div>
    </header>
    <div class="content">
      <div class="signal" aria-hidden="true"><span>R</span><b></b><span>L</span></div>
      <div class="code">{{.Code}}</div>
      <h1>{{.Title}}</h1>
      <p>{{.Description}}</p>
      <div class="notice">Настройки роутера не изменялись. Можно безопасно вернуться в RMM и проверить состояние агента или запустить диагностику.</div>
      <div class="actions">
        <a class="primary" href="{{.ActionURL}}">{{.ActionLabel}}</a>
        <a href="{{.ControlURL}}">Вернуться в RMM</a>
      </div>
      {{if .RequestID}}<div class="request">ID запроса: {{.RequestID}}</div>{{end}}
    </div>
  </main>
</body>
</html>`))

func luciErrorCopy(status int) (title, description, action string) {
	switch status {
	case http.StatusUnauthorized:
		return "Срок доступа истёк", "Временная ссылка или сессия LuCI больше не действует. Создайте новый безопасный доступ из RMM.", "Вернуться в RMM"
	case http.StatusForbidden:
		return "Доступ отклонён", "У вашей учётной записи нет разрешения на этот запрос к LuCI.", "Вернуться в RMM"
	case http.StatusNotFound:
		return "Сессия больше не активна", "Удалённая сессия закрыта или роутер ещё не подтвердил её запуск.", "Вернуться в RMM"
	case http.StatusConflict:
		return "Туннель ещё запускается", "Роутер получил команду, но защищённый канал пока не готов. Обычно это занимает несколько секунд.", "Проверить ещё раз"
	case http.StatusTooManyRequests:
		return "Слишком много запросов", "Лимит временных ссылок достигнут. Подождите минуту и повторите попытку.", "Вернуться в RMM"
	case http.StatusBadGateway:
		return "LuCI недоступен", "Туннель работает, но веб-интерфейс роутера не отвечает.", "Повторить подключение"
	case http.StatusGatewayTimeout:
		return "LuCI не отвечает", "Роутер на связи, но веб-интерфейс не ответил вовремя.", "Повторить подключение"
	default:
		return "Не удалось открыть LuCI", "На сервере произошла ошибка при подготовке удалённого доступа.", "Вернуться в RMM"
	}
}

func (a *App) writeLuCIError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if !strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/html") {
		writeError(w, status, message)
		return
	}
	title, description, actionLabel := luciErrorCopy(status)
	controlURL := "/"
	if a.publicURL != "" {
		controlURL = a.publicURL + "/"
	}
	actionURL := controlURL
	if status == http.StatusBadGateway || status == http.StatusGatewayTimeout || status == http.StatusConflict {
		actionURL = r.URL.Path
		if actionURL == "" {
			actionURL = "/"
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Language", "ru")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := luciErrorPageTemplate.Execute(w, luciErrorPageData{
		Code:        fmt.Sprintf("%d · %s", status, strings.ToUpper(http.StatusText(status))),
		Title:       title,
		Description: description,
		ActionLabel: actionLabel,
		ActionURL:   actionURL,
		ControlURL:  controlURL,
		RequestID:   requestID(r.Context()),
	}); err != nil {
		// Headers are already sent; keep the failure observable without exposing internals to the browser.
		logStructured(map[string]any{"event": "luci.error_page_failed", "request_id": requestID(r.Context()), "error": err.Error()})
	}
}

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
	principal, _ := principalFromContext(r.Context())
	accessURL, expiresAt, err := a.createDeviceAccessGrant(r.Context(), principal.User.ID, principal.User.Username, device, session)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "limit") {
			writeError(w, http.StatusTooManyRequests, "active LuCI access grant limit reached")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create LuCI access grant")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"url": accessURL, "expires_at": expiresAt})
}

func (a *App) createDeviceAccessGrant(ctx context.Context, userID, username string, device model.Device, session model.RemoteSession) (string, time.Time, error) {
	rawToken, err := randomToken(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().UTC().Add(deviceAccessGrantTTL)
	if err := a.store.CreateDeviceAccessGrant(ctx, store.TokenHash(rawToken), userID, device.ID, session.ID, expiresAt); err != nil {
		return "", time.Time{}, err
	}
	domainName := device.DNSLabel + "." + a.deviceDomain
	accessURL := a.publicScheme + "://" + domainName + "/_rmm/access?token=" + rawToken
	_, _ = a.store.AddAuditEvent(ctx, username, "luci.access_grant", device.ID, "", mustJSON(map[string]any{
		"remote_session_id": session.ID,
		"expires_at":        expiresAt,
		"request_id":        requestID(ctx),
	}))
	return accessURL, expiresAt, nil
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
		a.writeLuCIError(w, r, http.StatusUnauthorized, "LuCI access grant is required")
		return
	}
	route, found, err := a.store.AuthorizeDeviceAccessSession(r.Context(), store.TokenHash(cookie.Value), dnsLabel)
	if err != nil {
		a.writeLuCIError(w, r, http.StatusInternalServerError, "failed to authorize LuCI access")
		return
	}
	if !found {
		a.writeLuCIError(w, r, http.StatusUnauthorized, "LuCI access session has expired")
		return
	}
	if unsafeMethod(r.Method) && !sameOrigin(r) {
		a.writeLuCIError(w, r, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	a.proxyLuCIWithPrefix(w, r, route.DeviceID, route.RemoteSessionID, r.URL.Path, "")
}

func (a *App) consumeDeviceAccess(w http.ResponseWriter, r *http.Request, dnsLabel string) {
	if r.Method != http.MethodGet {
		a.writeLuCIError(w, r, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rawGrant := strings.TrimSpace(r.URL.Query().Get("token"))
	if rawGrant == "" {
		a.writeLuCIError(w, r, http.StatusBadRequest, "access token is required")
		return
	}
	rawSession, err := randomToken(32)
	if err != nil {
		a.writeLuCIError(w, r, http.StatusInternalServerError, "failed to create LuCI access session")
		return
	}
	route, found, err := a.store.ConsumeDeviceAccessGrant(r.Context(), store.TokenHash(rawGrant), store.TokenHash(rawSession), dnsLabel, time.Now().UTC().Add(deviceAccessSessionTTL))
	if err != nil {
		a.writeLuCIError(w, r, http.StatusInternalServerError, "failed to consume LuCI access grant")
		return
	}
	if !found || route.DNSLabel != dnsLabel {
		a.writeLuCIError(w, r, http.StatusUnauthorized, "invalid or expired LuCI access grant")
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

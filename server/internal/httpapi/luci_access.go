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
  <meta name="theme-color" content="#090b0d">
  <meta name="robots" content="noindex,nofollow">
  <title>{{.Title}} — OpenWrt RMM</title>

  <style>
    :root {
      color-scheme: dark;

      --color-bg: #090b0d;
      --color-surface-1: #0e1113;
      --color-surface-2: #131719;
      --color-surface-3: #191e21;

      --color-border: #283034;
      --color-border-strong: #394348;

      --color-text: #e4e8ea;
      --color-text-secondary: #a3aaae;
      --color-text-muted: #747d82;

      --color-accent: #6f8999;
      --color-accent-hover: #819bab;

      --color-warning: #b19a6b;

      --font-ui:
        Inter,
        "Segoe UI",
        Roboto,
        "Helvetica Neue",
        Arial,
        sans-serif;

      --font-mono:
        "IBM Plex Mono",
        "SFMono-Regular",
        Consolas,
        "Liberation Mono",
        monospace;
    }

    *,
    *::before,
    *::after {
      box-sizing: border-box;
    }

    html {
      min-width: 320px;
      min-height: 100%;
      background: var(--color-bg);
      text-size-adjust: 100%;
    }

    body {
      min-width: 320px;
      min-height: 100dvh;
      margin: 0;
      display: grid;
      place-items: center;
      padding: 24px;

      color: var(--color-text);
      background: var(--color-bg);

      font-family: var(--font-ui);
      font-size: 14px;
      line-height: 1.45;

      -webkit-font-smoothing: antialiased;
    }

    a {
      color: inherit;
      -webkit-tap-highlight-color: transparent;
    }

    a:focus-visible {
      outline: 2px solid var(--color-accent-hover);
      outline-offset: 2px;
    }

    .system-state {
      width: min(680px, 100%);
      overflow: hidden;

      border: 1px solid var(--color-border);
      border-radius: 6px;

      background: var(--color-surface-1);
    }

    .system-state-header {
      min-height: 54px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;

      padding: 0 20px;

      border-bottom: 1px solid var(--color-border);
      background: var(--color-surface-2);
    }

    .system-state-brand {
      color: var(--color-text);
      font-size: 13px;
      font-weight: 700;
      letter-spacing: .01em;
      text-decoration: none;
    }

    .system-state-code {
      color: var(--color-warning);

      font-family: var(--font-mono);
      font-size: 10px;
      font-weight: 600;
      letter-spacing: .08em;
    }

    .system-state-body {
      padding: 32px;
    }

    .system-state-symbol {
      display: grid;
      width: 42px;
      height: 42px;
      place-items: center;

      margin-bottom: 24px;

      border: 1px solid var(--color-border-strong);
      border-radius: 4px;

      color: var(--color-warning);
      background: var(--color-surface-2);
    }

    .system-state-symbol svg {
      width: 22px;
      height: 22px;

      fill: none;
      stroke: currentColor;
      stroke-width: 1.8;
      stroke-linecap: round;
      stroke-linejoin: round;
    }

    h1 {
      margin: 0 0 12px;

      color: var(--color-text);

      font-size: clamp(26px, 5vw, 36px);
      font-weight: 700;
      line-height: 1.08;
      letter-spacing: -.035em;
    }

    .system-state-description {
      max-width: 600px;
      margin: 0;

      color: var(--color-text-secondary);

      font-size: 14px;
      line-height: 1.65;
    }

    .system-state-context {
      display: grid;
      margin: 28px 0 0;

      border-top: 1px solid var(--color-border);
      border-bottom: 1px solid var(--color-border);
    }

    .system-state-context > div {
      display: grid;
      grid-template-columns: 130px minmax(0, 1fr);
      gap: 16px;

      padding: 11px 0;

      border-bottom: 1px solid var(--color-border);
    }

    .system-state-context > div:last-child {
      border-bottom: 0;
    }

    .system-state-context dt {
      color: var(--color-text-muted);

      font-family: var(--font-mono);
      font-size: 10px;
      font-weight: 600;
      letter-spacing: .04em;
      text-transform: uppercase;
    }

    .system-state-context dd {
      min-width: 0;
      margin: 0;

      color: var(--color-text-secondary);
      font-size: 12px;
    }

    .system-state-notice {
      margin-top: 24px;
      padding: 14px 16px;

      border: 1px solid var(--color-border);
      border-radius: 4px;

      color: var(--color-text-muted);
      background: var(--color-surface-2);

      font-size: 12px;
      line-height: 1.55;
    }

    .system-state-actions {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;

      margin-top: 24px;
    }

    .button {
      min-height: 36px;
      display: inline-flex;
      align-items: center;
      justify-content: center;

      padding: 0 12px;

      border: 1px solid var(--color-border);
      border-radius: 4px;

      color: var(--color-text-secondary);
      background: var(--color-surface-2);

      font-size: 13px;
      font-weight: 600;
      text-decoration: none;

      transition:
        border-color 120ms ease,
        background-color 120ms ease,
        color 120ms ease;
    }

    .button:hover {
      border-color: var(--color-border-strong);
      color: var(--color-text);
      background: var(--color-surface-3);
    }

    .button.primary {
      border-color: var(--color-accent);
      color: #080b0d;
      background: var(--color-accent);
    }

    .button.primary:hover {
      border-color: var(--color-accent-hover);
      color: #07090a;
      background: var(--color-accent-hover);
    }

    .system-state-request {
      margin-top: 20px;

      color: var(--color-text-muted);

      font-family: var(--font-mono);
      font-size: 10px;
      overflow-wrap: anywhere;
    }

    @media (max-width: 560px) {
      body {
        place-items: stretch;
        padding: 0;
      }

      .system-state {
        width: 100%;
        min-height: 100dvh;

        border: 0;
        border-radius: 0;
      }

      .system-state-header {
        padding: 0 18px;
      }

      .system-state-code {
        font-size: 9px;
      }

      .system-state-body {
        padding: 28px 18px;
      }

      .system-state-context > div {
        grid-template-columns: 1fr;
        gap: 4px;
      }

      .system-state-actions {
        display: grid;
      }

      .button {
        width: 100%;
        min-height: 42px;
      }
    }
  </style>
</head>

<body>
  <main class="system-state">
    <header class="system-state-header">
      <a class="system-state-brand" href="{{.ControlURL}}">
        OpenWrt RMM
      </a>

      <span class="system-state-code">
        {{.Code}}
      </span>
    </header>

    <div class="system-state-body">
      <div class="system-state-symbol" aria-hidden="true">
        <svg viewBox="0 0 24 24">
          <path d="M12 9v4"></path>
          <path d="M12 17h.01"></path>
          <path d="M10.3 3.6 2.7 17a2 2 0 0 0 1.7 3h15.2a2 2 0 0 0 1.7-3L13.7 3.6a2 2 0 0 0-3.4 0z"></path>
        </svg>
      </div>

      <h1>{{.Title}}</h1>

      <p class="system-state-description">
        {{.Description}}
      </p>

      <dl class="system-state-context">
        <div>
          <dt>Сервис</dt>
          <dd>Удалённый доступ LuCI</dd>
        </div>

        <div>
          <dt>Маршрут</dt>
          <dd>RMM → защищённый туннель → LuCI</dd>
        </div>
      </dl>

      <div class="system-state-notice">
        Настройки роутера не изменялись. Можно безопасно вернуться в RMM,
        проверить состояние устройства и при необходимости создать новый доступ.
      </div>

      <footer class="system-state-actions">
        <a class="button primary" href="{{.ActionURL}}">
          {{.ActionLabel}}
        </a>

        {{if ne .ActionURL .ControlURL}}
        <a class="button" href="{{.ControlURL}}">
          Вернуться в RMM
        </a>
        {{end}}
      </footer>

      {{if .RequestID}}
      <div class="system-state-request">
        ID запроса: {{.RequestID}}
      </div>
      {{end}}
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
	if unsafeMethod(r.Method) && !sameOrigin(r) && !sameOriginOpaqueLuCIRequest(r) {
		a.writeLuCIError(w, r, http.StatusForbidden, "cross-origin request rejected")
		return
	}
	a.proxyLuCIWithPrefix(w, r, route.DeviceID, route.RemoteSessionID, r.URL.Path, "")
}

func sameOriginOpaqueLuCIRequest(r *http.Request) bool {
	// Firefox can submit the LuCI form with an opaque Origin after the one-time access redirect.
	// The device access cookie was verified before this check; Fetch Metadata keeps the exception
	// limited to a same-origin browser request.
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("Origin")), "null") &&
		strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "same-origin")
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

# RMM OpenWrt Server

The server and bundled web application are licensed under `AGPL-3.0-only`. A deployed
modified version must keep a working offer of its corresponding source available to remote
users. The default legal page links to the project repository; downstream deployments must
update that link if their corresponding source is hosted elsewhere.

MVP backend for OpenWrt RMM.

## Run

```sh
RMM_INSECURE_DEV_MODE=true go run ./server/cmd/rmm-server
```

Environment variables:

- `RMM_ADDR` - listen address, default `:8080`
- `RMM_DB_PATH` - SQLite database path, default `rmm.db`
- `RMM_OPERATOR_PASSWORD` - required password used only when the bootstrap administrator is first created; rotate it later in the account UI
- `RMM_OPERATOR_TOKEN` - optional emergency/API bearer token
- `RMM_OPERATOR_USERNAME` - web UI username, default `admin`
- `RMM_PUBLIC_URL` - trusted external base URL used in password recovery links
- `RMM_SMTP_HOST`, `RMM_SMTP_PORT`, `RMM_SMTP_USERNAME`, `RMM_SMTP_PASSWORD`,
  `RMM_SMTP_FROM` - optional SMTP delivery for password recovery and alert notifications
- `RMM_SMTP_TLS_MODE` - `starttls` (default), `tls`, or `none` in insecure local mode only
- `RMM_SMTP_SERVER_NAME` - optional TLS server name override
- `RMM_TELEGRAM_BOT_TOKEN` - optional Telegram bot used for per-user alert delivery
- `RMM_NOTIFICATION_MAX_ATTEMPTS` - delivery attempt limit, default `5`
- `RMM_NOTIFICATION_RETENTION_DAYS` - terminal delivery history retention, default `90`
- `RMM_COOKIE_SECURE` - defaults to `true` outside explicit development mode
- `RMM_DEVICE_DOMAIN` - wildcard device domain, for example `routers.example.com`
- `RMM_ALLOW_LEGACY_ENROLLMENT` - opt-in shared enrollment compatibility mode
- `RMM_METRIC_RETENTION_DAYS` - metric history retention, default `30`
- `RMM_WEB_DIR` - static web UI directory, default `web`

## Docker Compose

The repository includes a Compose stack with the RMM server and a reverse SSH tunnel sidecar:

```sh
docker compose up -d --build
```

See `docs/docker-compose.md` for router key installation and remote access setup.

## MVP API

Agent API:

- `POST /api/agent/enroll`
- `POST /api/agent/heartbeat`
- `POST /api/agent/commands/next`
- `POST /api/agent/commands/{id}/result`

Operator API:

- `POST /api/auth/login`
- `POST /api/auth/password-reset/request`
- `POST /api/auth/password-reset/confirm`
- `POST /api/auth/logout`
- `GET /api/auth/me`
- `PATCH /api/auth/profile`
- `POST /api/auth/change-password`
- `POST /api/auth/logout-all`
- `GET|PUT /api/notifications/settings`
- `GET /api/notifications`
- `POST /api/notifications/test`
- `GET|POST /api/users` (administrator only)
- `PATCH /api/users/{id}` (administrator only)
- `POST /api/enrollment-grants`

- `GET /api/devices`
- `GET /api/events` (authenticated SSE stream; the client reloads user-scoped data on change)
- `GET /api/devices/{id}`
- `POST /api/devices/{id}/transfer`
- `POST /api/devices/{id}/commands`
- `GET /api/devices/{id}/commands`
- `GET /api/devices/{id}/commands/{command_id}`
- `POST /api/devices/{id}/commands/{command_id}/cancel`
- `GET /api/audit-events`

The web UI uses revocable, server-side sessions. Each normal user only sees and controls
devices enrolled with their own one-time grants. The optional bootstrap bearer token is
administrator-scoped:

Changing a password keeps the current browser session and revokes the user's other sessions.
The bootstrap environment password does not overwrite a password stored in SQLite after a
container restart.

```http
Authorization: Bearer <RMM_OPERATOR_TOKEN>
```

See [KeenDNS-like cloud mode](../docs/keendns.md) for wildcard DNS, wildcard TLS, and the
isolated LuCI access flow. Browser requests to an unavailable LuCI route receive a safe HTML
status page while API clients continue to receive JSON. The reverse proxy waits up to 15 seconds
for LuCI response headers before returning `504 Gateway Timeout`.

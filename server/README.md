# RMM OpenWrt Server

MVP backend for OpenWrt RMM.

## Run

```sh
RMM_INSECURE_DEV_MODE=true go run ./server/cmd/rmm-server
```

Environment variables:

- `RMM_ADDR` - listen address, default `:8080`
- `RMM_DB_PATH` - SQLite database path, default `rmm.db`
- `RMM_OPERATOR_PASSWORD` - required bootstrap administrator password
- `RMM_OPERATOR_TOKEN` - optional emergency/API bearer token
- `RMM_OPERATOR_USERNAME` - web UI username, default `admin`
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
- `GET /api/auth/me`
- `GET|POST /api/users` (administrator only)
- `POST /api/enrollment-grants`

- `GET /api/devices`
- `GET /api/devices/{id}`
- `POST /api/devices/{id}/commands`
- `GET /api/devices/{id}/commands`
- `GET /api/devices/{id}/commands/{command_id}`
- `POST /api/devices/{id}/commands/{command_id}/cancel`
- `GET /api/audit-events`

The web UI uses revocable, server-side sessions. Each normal user only sees and controls
devices enrolled with their own one-time grants. The optional bootstrap bearer token is
administrator-scoped:

```http
Authorization: Bearer <RMM_OPERATOR_TOKEN>
```

See [KeenDNS-like cloud mode](../docs/keendns.md) for wildcard DNS, wildcard TLS, and the
isolated LuCI access flow. Browser requests to an unavailable LuCI route receive a safe HTML
status page while API clients continue to receive JSON. The reverse proxy waits up to 15 seconds
for LuCI response headers before returning `504 Gateway Timeout`.

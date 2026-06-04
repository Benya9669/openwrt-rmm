# RMM OpenWrt Server

MVP backend for OpenWrt RMM.

## Run

```sh
go run ./server/cmd/rmm-server
```

Environment variables:

- `RMM_ADDR` - listen address, default `:8080`
- `RMM_DB_PATH` - SQLite database path, default `rmm.db`
- `RMM_ENROLLMENT_TOKEN` - shared enrollment token for MVP, default `dev-enroll-token`
- `RMM_OPERATOR_TOKEN` - operator API bearer token, default `dev-operator-token`
- `RMM_OPERATOR_USERNAME` - web UI username, default `admin`
- `RMM_OPERATOR_PASSWORD` - web UI password
- `RMM_SESSION_SECRET` - secret used to sign browser sessions
- `RMM_COOKIE_SECURE` - set `true` behind an HTTPS reverse proxy
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

- `GET /api/devices`
- `GET /api/devices/{id}`
- `POST /api/devices/{id}/commands`
- `GET /api/devices/{id}/commands`
- `GET /api/devices/{id}/commands/{command_id}`
- `POST /api/devices/{id}/commands/{command_id}/cancel`
- `GET /api/audit-events`

Operator API requests require:

```http
Authorization: Bearer dev-operator-token
```

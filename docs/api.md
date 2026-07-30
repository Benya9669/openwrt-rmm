# API

Base URL examples use `http://127.0.0.1:8080`.

## Operator Authentication

Browser users sign in with username/password. The server returns a random, revocable
`HttpOnly`, `SameSite=Strict` session cookie. Normal users are scoped to their own devices;
the bootstrap account has the `admin` role.

```http
POST /api/auth/login
Content-Type: application/json
```

```json
{
  "username": "admin",
  "password": "operator-password"
}
```

Check the current browser session:

```http
GET /api/auth/me
```

Sign out:

```http
POST /api/auth/logout
```

Operator API automation may continue using:

```http
Authorization: Bearer <operator-api-token>
```

The bearer token is optional and administrator-scoped.

## Users and enrollment grants

Administrators can list, create, disable, re-enable and reset passwords for accounts:

```http
GET /api/users
POST /api/users
PATCH /api/users/{user_id}
```

Every authenticated user can create a 60-second to 24-hour one-time enrollment grant
(15 minutes by default):

```http
POST /api/enrollment-grants
Content-Type: application/json

{"dns_label":"office-1","expires_seconds":900}
```

The response returns `enrollment_token` once. The agent exchanges it for its device ID and
token; a second exchange is rejected.

## Health

```http
GET /healthz
```

Response:

```json
{"status":"ok"}
```

## Release metadata

Authenticated clients obtain the server build and stable agent channel from the server
instead of embedding a version in JavaScript:

```http
GET /api/meta
Authorization: Bearer <operator-api-token>
```

```json
{
  "server_version": "0.9.0",
  "server_revision": "0123456789abcdef",
  "stable_agent_version": "0.6.8",
  "update_manifest_url": "https://benya9669.github.io/openwrt-rmm/update-manifest.json"
}
```

The dashboard applies Semantic Versioning: an older installed version receives an update
notice, an equal version is current, and a newer version is never offered a downgrade.
The referenced manifest, detached package-key signature, and Sigstore bundle are
published by the agent release workflow. The server keeps its configured fallback
version unless the detached signature verifies successfully.

## Agent Enrollment

```http
POST /api/agent/enroll
Content-Type: application/json
```

Request:

```json
{
  "enrollment_token": "one-time-enrollment-grant",
  "hostname": "openwrt-router-1",
  "openwrt_version": "OpenWrt 23.05.3"
}
```

Response:

```json
{
  "device_id": "dev_...",
  "device_token": "tok_..."
}
```

## Agent Heartbeat

```http
POST /api/agent/heartbeat
Authorization: Bearer tok_...
Content-Type: application/json
```

Request:

```json
{
  "device_id": "dev_...",
  "inventory": {
    "hostname": "openwrt-router-1",
    "openwrt_version": "OpenWrt 23.05.3",
    "wan_ip": "203.0.113.10/24",
    "dhcp_leases": [],
    "wifi_clients": []
  },
  "metrics": {
    "loadavg": "0.00 0.01 0.00 1/80 1234",
    "memory": {"total_kb": 262144, "used_kb": 131072},
    "disk": {"total_kb": 65536, "used_kb": 20000, "used_percent": "31%"},
    "interface_counters": [],
    "connectivity_checks": [
      {
        "target": "1.1.1.1",
        "reachable": true,
        "packet_loss_percent": 0,
        "latency_ms": 12.4
      }
    ]
  }
}
```

Response:

```json
{
  "commands": [
    {
      "id": "cmd_...",
      "device_id": "dev_...",
      "type": "ping",
      "args": {"target": "1.1.1.1"},
      "status": "queued",
      "created_at": "2026-06-01T12:00:00Z"
    }
  ]
}
```

## Agent Next Command

This endpoint is optimized for the MVP shell agent and returns at most one queued command as plain text.

```http
POST /api/agent/commands/next
Authorization: Bearer tok_...
Content-Type: application/json
```

Request:

```json
{
  "device_id": "dev_..."
}
```

Response:

```text
cmd_...	ping	{"target":"1.1.1.1"}
```

If there are no queued commands, the server returns `204 No Content`.

## Command Result

```http
POST /api/agent/commands/{command_id}/result
Authorization: Bearer tok_...
Content-Type: application/json
```

Request:

```json
{
  "device_id": "dev_...",
  "status": "completed",
  "exit_code": 0,
  "output": "PING 1.1.1.1 ...",
  "result": {}
}
```

## List Devices

```http
GET /api/devices
Authorization: Bearer <operator-api-token>
```

Response:

```json
{
  "devices": []
}
```

## Get Device

```http
GET /api/devices/{device_id}
Authorization: Bearer <operator-api-token>
```

## Update Device Fleet Metadata

```http
PATCH /api/devices/{device_id}/fleet
Authorization: Bearer <operator-api-token>
Content-Type: application/json
```

Request:

```json
{
  "group": "home",
  "tags": ["edge", "vpn"]
}
```

## Metrics History

```http
GET /api/devices/{device_id}/metrics-history?limit=100
Authorization: Bearer <operator-api-token>
```

Response:

```json
{
  "samples": [
    {
      "id": "met_...",
      "device_id": "dev_...",
      "inventory": {},
      "metrics": {},
      "created_at": "2026-06-01T12:00:00Z"
    }
  ]
}
```

## Device Alerts

```http
GET /api/devices/{device_id}/alerts
Authorization: Bearer <operator-api-token>
```

Alerts are computed from current device state, recent metric samples, and recent commands. The MVP alert types are:

- `offline`
- `memory_high`
- `disk_high`
- `wan_ip_changed`
- `command_attention`
- `wan_down`
- `packet_loss_high`
- `latency_high`

Alert statuses:

- `active`
- `acknowledged`
- `resolved`

Response:

```json
{
  "alerts": [
    {
      "id": "disk_high:dev_...",
      "device_id": "dev_...",
      "type": "disk_high",
      "severity": "warning",
      "status": "active",
      "message": "Disk usage is high",
      "details": {"used_percent": 88},
      "first_seen_at": "2026-06-01T12:00:00Z",
      "last_seen_at": "2026-06-01T12:03:00Z",
      "created_at": "2026-06-01T12:00:00Z"
    }
  ]
}
```

## Acknowledge Alert

```http
POST /api/devices/{device_id}/alerts/{alert_id}/acknowledge
Authorization: Bearer <operator-api-token>
```

Acknowledged alerts remain open while the condition still exists. When the condition disappears, the server marks the alert as `resolved`.

## Notification Settings and History

Notification settings belong to the authenticated user and apply to devices owned by that
user. Administrators do not implicitly receive notifications for devices owned by other
accounts.

```http
GET /api/notifications/settings
PUT /api/notifications/settings
GET /api/notifications?limit=50
POST /api/notifications/test
```

The update request accepts:

```json
{
  "email_enabled": true,
  "telegram_enabled": false,
  "telegram_chat_id": "123456789",
  "notify_warning": true,
  "notify_critical": true,
  "notify_resolved": true,
  "memory_threshold_percent": 85,
  "disk_threshold_percent": 85,
  "packet_loss_percent": 20,
  "latency_threshold_ms": 200,
  "repeat_minutes": 60
}
```

E-mail requires SMTP on the server and an e-mail address in the user profile. Telegram
requires `RMM_TELEGRAM_BOT_TOKEN` on the server and a numeric Chat ID in the profile.
Deliveries are persisted with `queued`, `sending`, `retry`, `sent`, or `dead_letter`
status. History also includes `attempt_count`, `max_attempts`, `last_attempt_at`, and
`next_attempt_at`. Raw destinations and server-side credentials are not returned by the
API. Active and resolved events use lifecycle deduplication; an optional repeat interval
can remind the user about an open problem.

The worker claims a delivery with a time-limited lease before contacting a provider. A
temporary failure uses exponential backoff starting at 30 seconds and capped at 30
minutes. After `RMM_NOTIFICATION_MAX_ATTEMPTS`, the delivery becomes `dead_letter`.
Expired worker leases are recovered after restart, so delivery is at-least-once and a
provider may rarely receive a duplicate if the server stopped after the provider accepted
the message but before SQLite recorded success.

## Create Command

```http
POST /api/devices/{device_id}/commands
Authorization: Bearer <operator-api-token>
Content-Type: application/json
```

Request:

```json
{
  "type": "ping",
  "args": {"target": "1.1.1.1"}
}
```

Allowed command types:

- `ping`
- `traceroute`
- `route_show`
- `interfaces_show`
- `reboot`
- `service_restart`
- `pkg_list_installed`
- `pkg_update`
- `pkg_list_upgradable`
- `pkg_install`
- `pkg_remove`
- `uci_show`
- `uci_backup`
- `uci_preview`
- `uci_set`
- `uci_commit`
- `uci_commit_confirmed`
- `uci_revert`
- `uci_restore`
- `remote_ssh_reverse`
- `remote_ssh_close`

Package commands are package-manager aware on the agent. OpenWrt 25.12+ uses `apk`; older releases usually use `opkg`. Legacy `opkg_*` command names are still accepted for compatibility, but the UI uses `pkg_*`.

## UCI Commands

UCI commands are queued through the same command endpoint:

```http
POST /api/devices/{device_id}/commands
Authorization: Bearer <operator-api-token>
Content-Type: application/json
```

Show a config:

```json
{
  "type": "uci_show",
  "args": {"config": "network"}
}
```

Create a backup:

```json
{
  "type": "uci_backup",
  "args": {"config": "network"}
}
```

Preview a value without leaving staged changes:

```json
{
  "type": "uci_preview",
  "args": {
    "config": "network",
    "section": "lan",
    "option": "ipaddr",
    "value": "192.168.1.1"
  }
}
```

Stage a value:

```json
{
  "type": "uci_set",
  "args": {
    "config": "network",
    "section": "lan",
    "option": "ipaddr",
    "value": "192.168.1.1",
    "commit": "false"
  }
}
```

Commit or revert staged changes:

```json
{"type": "uci_commit", "args": {"config": "network"}}
```

```json
{"type": "uci_revert", "args": {"config": "network"}}
```

Commit with connectivity confirmation:

```json
{
  "type": "uci_commit_confirmed",
  "args": {"config": "network", "confirm_seconds": "15"}
}
```

The agent commits, waits, checks server reachability, and restores the latest local backup if the server is unreachable.

`uci_preview` includes a `DIFF` section when the router has the `diff` command available. It still returns `CHANGE`, `BACKUP`, `BEFORE`, and `AFTER` sections for routers without `diff`.

Restore the latest local backup captured by `uci_backup`, `uci_preview`, or `uci_set`:

```json
{"type": "uci_restore", "args": {"config": "network"}}
```

The MVP agent only allows these UCI config packages:

- `network`
- `wireless`
- `dhcp`
- `firewall`
- `system`

The MVP UI exposes presets for:

- LAN IP: `network.lan.ipaddr`
- hostname: `system.@system[0].hostname`
- Wi-Fi SSID: `wireless.@wifi-iface[0].ssid`
- Wi-Fi password: `wireless.@wifi-iface[0].key`
- LAN DHCP toggle: `dhcp.lan.ignore`

Sensitive command output keys are redacted before storage and display. Current redaction targets include:

- `private_key`
- `password`
- `passwd`
- `secret`
- `psk`
- `token`

## Remote Sessions

Remote sessions create a short-lived SSH reverse tunnel command for the agent.

```http
POST /api/devices/{device_id}/remote-sessions
Authorization: Bearer <operator-api-token>
Content-Type: application/json
```

Request:

```json
{
  "target": "ssh",
  "server_host": "10.10.10.2",
  "server_port": 22,
  "remote_port": 22022,
  "local_port": 22,
  "luci_scheme": "https",
  "duration_seconds": 900
}
```

Response:

```json
{
  "id": "ras_...",
  "device_id": "dev_...",
  "target": "ssh",
  "status": "queued",
  "server_host": "10.10.10.2",
  "server_port": 22,
  "remote_port": 22022,
  "luci_port": 22122,
  "luci_scheme": "https",
  "local_host": "127.0.0.1",
  "local_port": 22,
  "command_id": "cmd_...",
  "created_at": "2026-06-01T12:00:00Z",
  "expires_at": "2026-06-01T12:15:00Z"
}
```

List sessions:

```http
GET /api/devices/{device_id}/remote-sessions?limit=25
Authorization: Bearer <operator-api-token>
```

Close a session and queue tunnel termination on the agent:

```http
POST /api/devices/{device_id}/remote-sessions/{session_id}/close
Authorization: Bearer <operator-api-token>
```

Open LuCI for an active session through the authenticated RMM proxy:

```http
GET /luci/{device_id}/{session_id}/  # legacy lab-only compatibility route
```

## Create Bulk Command

```http
POST /api/devices/bulk-commands
Authorization: Bearer <operator-api-token>
Content-Type: application/json
```

Request:

```json
{
  "device_ids": ["dev_..."],
  "type": "ping",
  "args": {"target": "1.1.1.1"}
}
```

The server creates one command per unique device id.

## List Device Commands

```http
GET /api/devices/{device_id}/commands?limit=50
Authorization: Bearer <operator-api-token>
```

Response:

```json
{
  "commands": [
    {
      "id": "cmd_...",
      "device_id": "dev_...",
      "type": "ping",
      "args": {"target": "1.1.1.1"},
      "status": "completed",
      "result": {},
      "output": "PING ...",
      "exit_code": 0,
      "attempt_count": 1,
      "max_attempts": 3,
      "created_at": "2026-06-01T12:00:00Z",
      "expires_at": "2026-06-02T12:00:00Z",
      "claimed_at": "2026-06-01T12:00:01Z",
      "completed_at": "2026-06-01T12:00:03Z",
      "expired_at": null
    }
  ]
}
```

Command lifecycle defaults:

- new commands expire after 24 hours;
- claimed commands are returned to `queued` after the server-side claim timeout while attempts remain;
- claimed commands transition to `expired` after `max_attempts` is reached;
- expired and cancelled commands no longer accept agent results.

## Get Command

```http
GET /api/devices/{device_id}/commands/{command_id}
Authorization: Bearer <operator-api-token>
```

## Cancel Command

```http
POST /api/devices/{device_id}/commands/{command_id}/cancel
Authorization: Bearer <operator-api-token>
```

Only `queued` and `claimed` commands can transition to `cancelled`.

## List Audit Events

```http
GET /api/audit-events?device_id=dev_...&limit=50
Authorization: Bearer <operator-api-token>
```

Response:

```json
{
  "audit_events": [
    {
      "id": "aud_...",
      "actor": "operator",
      "action": "command.create",
      "device_id": "dev_...",
      "command_id": "cmd_...",
      "details": {"type": "ping"},
      "created_at": "2026-06-01T12:00:00Z"
    }
  ]
}
```
## Account API

Authenticated browser sessions can manage their own account without administrator access:

- `GET /api/auth/me` returns the current user, including `display_name` and `email`.
- `PATCH /api/auth/profile` accepts `display_name` and `email`.
- `POST /api/auth/change-password` accepts `current_password` and `new_password`; other sessions are revoked.
- `POST /api/auth/logout-all` revokes every session for the current user.

## Notification center and delivery policy

- `GET /api/notification-center?limit=50` returns inbox entries and the unread count.
- `POST /api/notification-center/{id}/read` marks one entry read.
- `POST /api/notification-center/read-all` marks the current user's inbox read.
- `POST /api/notifications/verify/email/request|confirm` verifies the profile e-mail with a six-digit code.
- `POST /api/notifications/verify/telegram/request|confirm` proves ownership of a Telegram chat before it can be enabled.
- `GET|PUT /api/notifications/settings` includes timezone, quiet hours, maintenance pause and signed webhook settings.
- `GET|PATCH /api/devices/{id}/notification-settings` controls per-router severity and pause overrides.

Webhook requests use `Content-Type: application/json`, `X-RMM-Timestamp` and
`X-RMM-Signature: sha256=<hex HMAC-SHA256>`. The signed bytes are
`timestamp + "." + raw_request_body`. Only public HTTPS endpoints are accepted.

## LAN client presence

`GET /api/devices/{id}/clients` returns persisted clients with `first_seen_at`,
`last_seen_at`, `last_checked_at` and one of:

- `online`: confirmed by Wi-Fi association, an active neighbour state or a successful safe ICMP probe;
- `recent`: confirmed within the recent-presence window;
- `unconfirmed`: known from DHCP or stale neighbour data but not actively confirmed.

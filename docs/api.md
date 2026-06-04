# API

Base URL examples use `http://127.0.0.1:8080`.

## Operator Authentication

Browser operators sign in with username/password. The server returns a signed `HttpOnly` session cookie.

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
Authorization: Bearer operator-token
```

## Health

```http
GET /healthz
```

Response:

```json
{"status":"ok"}
```

## Agent Enrollment

```http
POST /api/agent/enroll
Content-Type: application/json
```

Request:

```json
{
  "enrollment_token": "dev-enroll-token",
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
Authorization: Bearer dev-operator-token
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
Authorization: Bearer dev-operator-token
```

## Update Device Fleet Metadata

```http
PATCH /api/devices/{device_id}/fleet
Authorization: Bearer dev-operator-token
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
Authorization: Bearer dev-operator-token
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
Authorization: Bearer dev-operator-token
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
Authorization: Bearer dev-operator-token
```

Acknowledged alerts remain open while the condition still exists. When the condition disappears, the server marks the alert as `resolved`.

## Create Command

```http
POST /api/devices/{device_id}/commands
Authorization: Bearer dev-operator-token
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

Package commands are package-manager aware on the agent. OpenWrt 25.12+ uses `apk`; older releases usually use `opkg`. Legacy `opkg_*` command names are still accepted for compatibility, but the UI uses `pkg_*`.

## UCI Commands

UCI commands are queued through the same command endpoint:

```http
POST /api/devices/{device_id}/commands
Authorization: Bearer dev-operator-token
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
Authorization: Bearer dev-operator-token
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
Authorization: Bearer dev-operator-token
```

Close a session record:

```http
POST /api/devices/{device_id}/remote-sessions/{session_id}/close
Authorization: Bearer dev-operator-token
```

The MVP close action updates server state and audit history. Active SSH processes stop by duration timeout on the agent side.

## Create Bulk Command

```http
POST /api/devices/bulk-commands
Authorization: Bearer dev-operator-token
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
Authorization: Bearer dev-operator-token
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
Authorization: Bearer dev-operator-token
```

## Cancel Command

```http
POST /api/devices/{device_id}/commands/{command_id}/cancel
Authorization: Bearer dev-operator-token
```

Only `queued` and `claimed` commands can transition to `cancelled`.

## List Audit Events

```http
GET /api/audit-events?device_id=dev_...&limit=50
Authorization: Bearer dev-operator-token
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

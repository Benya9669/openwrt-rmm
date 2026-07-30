# Architecture

## MVP Direction

The first implementation uses:

- Go server;
- SQLite persistence;
- Go OpenWrt agent, distributed as APK/IPK packages;
- outbound HTTP polling;
- REST API;
- per-user, one-time enrollment grants;
- per-device bearer token after enrollment.

This gives a small vertical slice:

```text
agent enrolls -> server creates device -> agent sends heartbeat -> server queues command -> agent executes command -> server stores result
```

## Server

The server owns users and roles, device ownership and DNS names, revocable sessions,
device identity, current state, command queues, temporary tunnels and command results.

Core tables:

- `devices`
- `commands`
- `users`
- `operator_sessions`
- `enrollment_grants`
- `device_access_grants`
- `device_access_sessions`
- `audit_events`
- `alerts`
- `metric_samples`
- `remote_sessions`
- `notification_settings`
- `notification_deliveries`
- `device_notification_settings`
- `inbox_notifications`
- `contact_verifications`
- `lan_clients`

Notification delivery uses a persistent SQLite queue. Workers atomically claim ready rows
with a lease, increment the attempt counter, and finish them as `sent`, `retry`, or
`dead_letter`. Expired leases are claimable after a process restart. Terminal history has
an independent retention period; queued, retrying, and in-flight rows are preserved.

## Agent

The production agent is a small Go binary with a native HTTPS client. It uses tools
normally available on OpenWrt only for platform operations:

- `ubus`
- `ip`
- `opkg`
- `/etc/init.d/*`
- `ping`, `uci`, and the active package manager

The agent does not accept inbound connections. It polls the server and executes only allowlisted command types.

## Security Model

Current security:

- enrollment requires a short-lived one-time grant owned by a user;
- enrolled devices receive a random bearer token;
- reusable credentials are stored as hashes;
- agent API requests require the device bearer token;
- users authenticate through revocable server-side sessions and are restricted to their
  own devices; admin functions require the admin role;
- LuCI is isolated on wildcard device subdomains and uses one-time access grants;
- server only queues allowlisted command types;
- agent also checks its own command allowlist.

Required before production:

- mTLS or signed device tokens;
- token rotation;
- command signatures;
- replay protection;
- per-device SSH tunnel credentials;
- organization-level tenancy and MFA.

## Transport

The agent uses outbound HTTPS polling:

- easier to run on constrained OpenWrt images;
- works behind NAT and CG-NAT;
- does not require stable long-lived connections;
- simple to debug with curl.

Server-Sent Events update the browser dashboard; agent transport remains HTTPS polling.
WebSocket or MQTT can be added later if faster agent command delivery is required.

The server exposes authenticated release metadata to the dashboard. A background
resolver downloads the stable agent manifest, verifies its detached ECDSA signature with
the package repository public key, and retains the last trusted fallback when refresh
fails.

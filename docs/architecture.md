# Architecture

## MVP Direction

The first implementation uses:

- Go server;
- SQLite persistence;
- POSIX shell OpenWrt agent;
- outbound HTTP polling;
- REST API;
- shared enrollment token for first registration;
- per-device bearer token after enrollment.

This gives a small vertical slice:

```text
agent enrolls -> server creates device -> agent sends heartbeat -> server queues command -> agent executes command -> server stores result
```

## Server

The server owns device identity, current device state, command queue, and command results.

MVP tables:

- `devices`
- `commands`

Later tables:

- `organizations`
- `users`
- `audit_events`
- `device_groups`
- `alerts`
- `metrics`

## Agent

The MVP agent is intentionally simple and uses tools normally available on OpenWrt:

- `ubus`
- `ip`
- `opkg`
- `/etc/init.d/*`
- `curl` or `wget`

The agent does not accept inbound connections. It polls the server and executes only allowlisted command types.

## Security Model

MVP security:

- enrollment requires a shared token;
- enrolled devices receive a random bearer token;
- agent API requests require the device bearer token;
- operator API requests require a bearer token;
- server only queues allowlisted command types;
- agent also checks its own command allowlist.

Required before production:

- mTLS or signed device tokens;
- token rotation;
- operator authentication and RBAC;
- command signatures;
- audit log for every operator action;
- replay protection;
- rate limits.

## Transport

MVP uses polling HTTP:

- easier to run on constrained OpenWrt images;
- works behind NAT and CG-NAT;
- does not require stable long-lived connections;
- simple to debug with curl.

WebSocket or MQTT can be added later for faster command delivery.

# Changelog

This file contains user-facing release notes. Every section must match its Git tag;
the release workflow fails when notes for a new tag have not been prepared.

## agent-v0.6.8

Stable agent release with more reliable LAN client presence detection.

### Added

- The agent safely probes private IPv4 addresses found in DHCP leases and reports the
  results with its heartbeat.
- Probing is capped at 32 addresses, six concurrent requests, and a short timeout to
  avoid noticeable load on the router or LAN.
- The server stores `first_seen`, `last_seen`, and the most recent probe time for each
  client.
- The LuCI application keeps English as its default language and provides Russian through
  the separate `luci-i18n-rmm-agent-ru` package.

### Fixed

- A static DHCP reservation is no longer treated as proof that a client is currently
  connected.
- Clients are separated into Online, Recently online, and Unconfirmed states.
- DHCP, Wi-Fi, neighbor-table, and active-probe data are merged without duplicate
  clients.

### Compatibility

- The API protocol remains `v1`; the server and agent can still be upgraded
  independently.
- Existing agent configuration remains compatible without changes.
- The primary workflow builds OpenWrt 24.10 and 25.12 packages. OpenWrt 21.02, 22.03,
  and 23.05 packages are added by the separate legacy workflow.

### Installation

Download the `.ipk` or `.apk` matching the OpenWrt release and target architecture.
Install `luci-app-rmm-agent` as well to configure the agent through LuCI. Install
`luci-i18n-rmm-agent-ru` for the Russian interface.

## server-v0.8.1

Corrective server release for cloud SSH and LuCI tunnels.

### Fixed

- The server validates SSH and LuCI ports issued to agents and rejects invalid
  endpoints.
- Reverse-tunnel parameters are validated before they are exposed to an operator.
- Boundary-value and malformed tunnel-service response tests were added.

### Compatibility

- No database migration is required.
- The agent protocol remains `v1`.

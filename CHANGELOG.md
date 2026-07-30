# Changelog

This file contains user-facing release notes. Every section must match its Git tag;
the release workflow fails when notes for a new tag have not been prepared.

## server-v0.9.0

Feature release for reliable notifications, LAN client presence, and release metadata.

### Added

- A built-in notification center with unread state, incident grouping, router links,
  and live updates through the existing authenticated event stream.
- E-mail and Telegram ownership verification before a destination can receive alerts.
- Signed webhook delivery, quiet hours with a user timezone, maintenance pauses, and
  per-router notification overrides.
- Persistent LAN client history with Online, Recently online, and Unconfirmed states.
- Authenticated `/api/meta` release metadata for the server version, source revision,
  stable agent version, and signed update-manifest location.
- A stable agent update manifest containing package-feed compatibility entries. Agent
  release workflows sign it with the package repository ECDSA key and Sigstore, then
  publish it with the package repository.

### Fixed

- The dashboard now compares agent versions using Semantic Versioning instead of treating
  every unequal version as outdated.
- Agents newer than the configured stable version are shown as newer, not offered a
  downgrade.
- Missing release metadata no longer produces a false update warning.
- The server periodically refreshes the stable manifest and accepts a new version only
  after verifying its detached signature; network or signature failures retain the last
  trusted fallback.
- HTML is served with `no-store`, scripts and styles are revalidated, and asset revisions
  were advanced so an old cached dashboard cannot keep reporting obsolete versions.
- Static DHCP reservations and stale neighbor entries no longer prove that a LAN client
  is currently online.

### Security

- Webhook destinations require public HTTPS endpoints, reject local/private targets, and
  receive an HMAC-SHA256 signature over the timestamp and raw body.
- Notification destinations stay masked in browser responses.
- Contact verification, per-user ownership checks, and per-device notification access
  remain enforced server-side.

### Compatibility and upgrade

- The agent protocol remains `v1`; existing `0.6.x` agents continue to work.
- SQLite startup migration adds the notification-center, verification, per-device
  notification, and LAN-client persistence structures without deleting existing data.
- Back up the production SQLite database and test the migration on a copy before
  deploying this release.
- Set `RMM_RELEASE_VERSION=0.9.0`. The default stable agent is `0.6.8` and can be
  overridden with `RMM_STABLE_AGENT_VERSION`.

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

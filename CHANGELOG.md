# Changelog

This file contains user-facing release notes. Every section must match its Git tag;
the release workflow fails when notes for a new tag have not been prepared.

## Unreleased

- Added admin-only, per-device agent rollback using ECDSA-verified historical stable manifests,
  trusted manifest URL boundaries, inventory compatibility checks, and immutable version-pinned packages.

## server-v0.9.2

First managed single-router production Go agent update stage.

### Added

- Authenticated per-device `agent-update` requests now derive a compatible immutable feed
  from the last reported production Go agent inventory and queue a dedicated update command.
- The server retains the complete signature-verified stable manifest, including its feed base
  and package compatibility entries, rather than retaining only the agent version.

### Security and compatibility

- Updates are limited to the `rmm-agent-go-production` package and require an exact reported
  OpenWrt release, target, and package manager match. Unsupported devices are not queued.
- Admins can create stable or signed-candidate canary rollouts for explicit device IDs, with
  persisted batch/device state, automatic failure pauses, and pause/resume/cancel controls.
- Rollback is intentionally not implemented: prior immutable feeds are not retained and the
  agent update protocol has no downgrade path.

## agent-v0.6.9

Managed update support for the production Go agent.

### Added

- Inventory now reports the production package identity, package manager, OpenWrt release,
  and target needed for the server to select an immutable compatible feed.
- `agent_update` validates its fixed package, package manager, and HTTPS feed arguments,
  requires at least 8192 KiB free on the root filesystem, and returns structured outcomes.

## server-v0.9.1

Stabilization release for notification operations, account navigation, and LAN client
inventory quality.

### Added

- Delivery metrics for queued, sent, retrying, and dead-letter notifications.
- Oldest queue age plus the latest successful delivery and latest safe error for each
  e-mail, Telegram, and webhook channel.
- Server-side notification history filters for router, severity, event, channel, and
  delivery status.
- Human-readable channel diagnostics that distinguish unavailable server configuration,
  missing destinations, unverified contacts, disabled channels, and ready channels.
- Separate Profile, Security, Notifications, and administrator-only Users tabs in the
  account dialog.
- Responsive metric cards, channel health summaries, and delivery-history filters in the
  Notifications tab.

### Fixed

- Failed and stale kernel neighbour entries no longer create standalone LAN clients.
- Historical unconfirmed neighbour noise is removed after the next heartbeat.
- DHCP-only records remain explicitly unconfirmed, while active neighbour, Wi-Fi, and
  probe evidence controls online presence.
- IPv4 is preferred over a link-local IPv6 address when both belong to the same MAC.
- Webhook deliveries are labelled correctly in notification history.

### Security and privacy

- Notification filters remain scoped to the authenticated user.
- Raw destinations, provider exceptions, secrets, and server credentials are not exposed
  through metrics or diagnostics.
- The Users tab is rendered only for administrators and existing server-side role checks
  remain authoritative.

### Compatibility and upgrade

- The agent protocol remains `v1`; no agent upgrade is required.
- No new SQLite schema migration is required. Metrics are calculated from the existing
  durable delivery queue.
- Back up the production database, set `RMM_RELEASE_VERSION=0.9.1`, and run the
  notification and LAN-client smoke checks before completing the deployment.

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

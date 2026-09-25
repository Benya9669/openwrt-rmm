# Changelog

This file contains user-facing release notes. Every section must match its Git tag;
the release workflow fails when notes for a new tag have not been prepared.

## Unreleased

No unreleased changes.

## server-v0.12.3

SQLite request-cancellation resilience hotfix.

### Fixed

- Cancelling a browser request no longer interrupts the shared SQLite connection and causes unrelated RMM API, tunnel, agent, or background operations to fail.
- Docker Compose defaults and deployment examples now target the `0.12.3` server and tunnel images.

## server-v0.12.2

Cloud LuCI login compatibility hotfix.

### Fixed

- Firefox can submit the LuCI login form opened through a cloud access link when it sends an opaque `Origin: null`; the exception is limited to an already-authorized device session with `Sec-Fetch-Site: same-origin`.
- Docker Compose defaults and deployment examples now target the `0.12.2` server and tunnel images.

## server-v0.12.1

Web interface polish and agent-update status hotfix.

### Changed

- Router backups now use a compact, responsive list with consistent actions and a concise safety notice that explains identity preservation and automatic recovery.
- Completed agent updates and rollbacks are reported through one-time toast notifications instead of remaining as a persistent status banner above the device tabs.
- Docker Compose defaults and deployment documentation now target the `0.12.1` server and tunnel images.

### Fixed

- Backup recovery guidance no longer collapses into a narrow column or forces horizontal overflow on laptop and mobile viewports.
- Historical completed agent operations no longer generate stale notifications when a router is opened, while newly completed operations still notify the operator once.

## server-v0.12.0

Security and encrypted recovery release.

### Added

- Commands are signed with a persistent Ed25519 server key and include device binding, an expiry time, and a random nonce for agent-side replay protection.
- Device credentials support a two-phase, interruption-safe rotation and an administrator-only emergency revoke; the tunnel sidecar terminates already-established reverse listeners after revoke, close, or expiry.
- Webhook secrets, Telegram identifiers, verification destinations, pending device credentials, and sensitive notification delivery payloads are encrypted at rest with context-bound AES-256-GCM.
- Managed router backups use `sysupgrade -b`, encrypted SQLite storage, SHA-256 verification, target compatibility checks, retention, archive manifests, and a guarded restore workflow with a local emergency backup.
- Administrators can download a consistent SQLite snapshot created with `VACUUM INTO` from the maintenance interface.

### Changed

- Docker Compose defaults and deployment documentation now target the `0.12.0` server/tunnel pair and advertise agent `0.8.0`.
- Existing plaintext notification secrets are encrypted automatically after the persistent data-encryption key is initialized.
- The database pins the command-signing and data-encryption key identifiers and refuses startup with unrelated recovery keys.
- Consistent SQLite snapshots stream from a temporary `VACUUM INTO` file instead of loading the complete database into server memory.

### Fixed

- Heartbeat command claiming no longer returns the same queued operation repeatedly before its result is received.
- The Playwright server wrapper uses a graceful test teardown signal before the bounded Windows child-process fallback.

## agent-v0.8.0

Signed commands and managed recovery.

### Added

- The agent pins the server command-signing public key and verifies every command signature, device ID, nonce, creation time, and expiry before execution.
- Persistent result and pending markers prevent a command from running twice after a crash or lost response.
- Two-phase device-token rotation is stored atomically without interrupting normal heartbeat delivery.
- Managed encrypted cloud backup upload and guarded restore verify size, target, and SHA-256, preserve the current agent identity, and automatically roll back if a later heartbeat does not confirm cloud connectivity.

### Security

- A changed server command-signing key is rejected until explicit re-enrollment.
- Explicit re-enrollment clears the old command-key pin and creates a new per-device tunnel identity.
- Interrupted state-changing commands fail closed instead of being replayed automatically.

## server-v0.11.3

Release and local-development stabilization.

### Fixed

- The Playwright server wrapper now has a bounded child-process shutdown path on Windows instead of leaving the local test run open after all scenarios have completed.

## server-v0.11.2

Interface consistency and login layout hotfix.

### Changed

* LuCI remote-access error pages now match the current OpenWrt RMM design system while remaining self-contained in the Go HTTP template.
* LuCI access errors use the same compact infrastructure-console styling, typography, spacing, controls, and semantic status presentation as the redesigned web interface.

### Fixed

* Login-page authentication capability labels no longer overlap their descriptions when using longer monospace labels such as `SESSION`.
* LuCI error pages no longer render duplicate “Вернуться в RMM” actions when the primary action already points back to the control panel.

## server-v0.11.1

Application icon refresh.

### Changed

- Updated favicon, PWA, and Apple touch icon assets to match the redesigned OpenWrt RMM interface.
- Removed the unintended opaque background around the application icon.

## server-v0.11.0

Complete UI/UX redesign and responsive interface overhaul for the OpenWrt RMM web console.

### Added

- A unified design system now defines the application's colors, typography, spacing, control sizes,
  borders, status presentation, responsive behavior, technical output, dialogs, and accessibility
  conventions.
- Reusable application-native confirmation dialogs replace browser-native confirmation flows for
  destructive and security-sensitive actions.
- Shared technical-output and configuration-diff patterns provide consistent presentation for
  command results, diagnostics, package operations, UCI previews, and backend error details.
- Dedicated system-state presentation now covers unavailable services, access errors, empty data,
  stale telemetry, offline devices, reconnecting states, and other exceptional conditions.
- Responsive mobile representations were added for dense operational data instead of relying on
  compressed desktop tables.
- A unified Tabler Icons-based icon system provides consistent outline icons across navigation,
  dialogs, statuses, actions, and dynamically rendered interface elements.

### Changed

- The authenticated application was redesigned around a compact dark infrastructure-console
  aesthetic with rectangular controls, reduced corner radii, muted semantic colors, and higher
  information density.
- Fleet, device overview, clients, network interfaces, problems, operations, diagnostics, remote
  access, Expert mode, UCI configuration, packages, audit, and maintenance now share the same
  visual and interaction system.
- Profile, account security, notification settings, notification history, user management, router
  enrollment, login, landing, legal, and public system states were brought into the same design
  language as the main RMM console.
- Desktop navigation, mobile navigation, tabs, dialogs, forms, tables, filters, statuses, and
  action hierarchies were standardized across the application.
- Device and fleet views prioritize operational status, stale/offline state, WAN health, problems,
  telemetry, and primary actions without oversized dashboard cards.
- Client and interface tables use structured compact records on narrow screens so IPv4, IPv6,
  MAC addresses, signal data, traffic counters, and long hostnames remain readable.
- Expert and UCI workflows now visually separate read-only, primary, recovery, and destructive
  operations while preserving the existing backend behavior.
- Notification settings are grouped by channels, event types, thresholds, quiet hours, device
  overrides, delivery state, and history instead of presenting one flat configuration form.
- Landing and login pages were restyled to match the infrastructure-console interface instead of
  using a separate bright SaaS-oriented visual language.
- Technical identifiers, addresses, command output, package names, UCI values, and similar data
  now use a consistent monospace presentation while normal interface copy remains sans-serif.

### Fixed

- Responsive layouts no longer depend on overlapping mobile overrides for the same components and
  now use a more consistent breakpoint strategy.
- Dense tables and technical values no longer introduce page-level horizontal scrolling on narrow
  viewports.
- Long IPv6 addresses, hostnames, tags, command output, audit context, webhook URLs, and backend
  messages are constrained without breaking their surrounding layouts.
- Mobile dialogs and forms remain within the dynamic viewport and keep their actions accessible
  on small screens.
- Offline, stale, loading, empty, filtered-empty, and error states are now visually distinguished
  instead of falling back to inconsistent component-specific presentation.
- Destructive operations no longer rely on generic browser `confirm`, `alert`, or `prompt`
  interactions where an application-native workflow is available.
- Legacy Unicode and emoji glyphs used as interface icons were replaced with consistent SVG
  iconography.
- Dialog close controls, navigation actions, notification controls, and other icon-only buttons
  now use consistent sizing and interaction states.

### Accessibility

- Icon-only controls now expose accessible names while decorative SVG icons are excluded from the
  accessibility tree.
- Status presentation combines text, iconography, and semantic color instead of relying on color
  alone.
- Dialog focus handling, keyboard interaction, focus-visible states, form labels, validation
  feedback, and destructive-action confirmations were standardized across the redesigned UI.
- Mobile controls and navigation use consistent touch targets and respect viewport and safe-area
  constraints.

### Documentation

- `DESIGN.md` documents the permanent OpenWrt RMM design system and acts as the source of truth for
  future frontend changes.
- The design guide defines visual principles, design tokens, responsive conventions, technical
  typography, status patterns, dialogs, forms, tables, destructive actions, and iconography.
- Tabler Icons are documented as the project's single supported production icon set, including
  sizing, stroke, color inheritance, accessibility, and vendoring conventions.

### Validation

- Frontend and browser coverage exercises the redesigned navigation, Fleet, device views, clients,
  network interfaces, operations, Expert workflows, dialogs, profile, notifications, user
  management, login, and public states.
- Responsive checks cover phone, tablet, desktop, and wide-desktop layouts, including narrow
  320–430 px viewports and dense technical content.
- Regression coverage verifies application-native confirmations, dynamic icon rendering, dialog
  behavior, filtering, navigation, command output, and representative error and offline states.

## server-v0.10.2

Secure tunnel authorization compatibility hotfix.

### Fixed

- The SSH sidecar now materializes its internal authorization token and endpoint in protected
  runtime files because OpenSSH intentionally sanitizes the `AuthorizedKeysCommand` environment.
- Per-device tunnel authentication no longer fails before contacting the server with
  `Permission denied (publickey)` after secure mode is enabled.

### Security

- Runtime authorization files remain root-owned, are group-readable only by the unprivileged
  command user, and use mode `0440` inside a `0750` directory.
- The token and authorization URL are removed from the long-running `sshd` process environment.
- The legacy shared-key file remains empty whenever secure per-device authorization is enabled.

### Validation

- A regression test invokes the authorization helper with a sanitized environment and verifies
  the exact bearer-token and key-fingerprint request without making a network call.

## server-v0.10.1

Single-file GitOps deployment and visible server version.

### Added

- The authenticated dashboard displays the running server build version next to API health.
- `compose.dev.yaml` preserves explicit source builds while production GitOps uses published images.

### Changed

- The base `compose.yaml` now pulls matching versioned server and tunnel images without requiring
  a release overlay.
- Database and tunnel volumes have configurable explicit names so a GitOps project rename can
  reattach existing state instead of silently creating empty project-scoped volumes.

### Documentation

- The deployment guide includes a non-destructive Arcane migration procedure with database and
  tunnel-key backups, exact-volume discovery, recreation and verification steps.

### Validation

- Compose configuration is validated in production and development-image modes.
- Browser coverage verifies that release metadata is rendered in the authenticated sidebar.

## server-v0.10.0

Secure cloud tunnels and responsive LAN inventory.

### Added

- The control plane stores a unique Ed25519 public-key fingerprint and rotation epoch for
  every router while the private key remains on the device.
- The SSH sidecar resolves authorized keys through a token-protected internal endpoint and
  limits each credential to the ports of its active, non-expired remote session.
- Remote session creation reserves ports transactionally and enforces per-device concurrent
  session and creation-rate limits.
- Device transfers revoke the previous tunnel credential and advance its key epoch so the
  router rotates its identity on the next heartbeat.

### Changed

- Secure tunnel commands include the persistent SSH host public key and require strict host-key
  verification from compatible agents.
- The stable agent advertised by server images is now `0.7.0`.
- The LAN client table uses flexible columns at 1366×768, keeps status markers aligned, and
  truncates long values without introducing horizontal scrolling.

### Fixed

- WAN neighbours are no longer presented as LAN clients.
- Duplicate tunnel-port reservations are rejected instead of allowing ambiguous forwarding.

### Deployment

- Deploy server and agent `0.7.0` first with `RMM_TUNNEL_AUTH_TOKEN` empty, wait for router
  heartbeats to register per-device keys, then configure the shared internal auth token and
  persistent `RMM_TUNNEL_HOST_PUBLIC_KEY` during a maintenance window.

### Validation

- Go tests cover credential registration, epoch rotation, transfer revocation, authenticated
  key lookup, port collisions and session limits.
- Browser tests cover the LAN client table at Full HD and 1366×768 without page or list overflow.

## agent-v0.7.0

Per-device tunnel identity and strict server authentication.

### Added

- Routers generate a unique Ed25519 tunnel identity and register only the public key with
  the control plane.
- The heartbeat reports the public credential and key epoch needed for server-side authorization.

### Security

- Secure tunnel commands pin the persistent server host key, enable strict host-key checking,
  use only the device identity and bind reverse forwards explicitly.
- Secure mode requires OpenSSH and fails closed when its host key or per-device identity is
  missing; the legacy client remains available only during the staged migration.
- A server epoch change stops existing tunnel processes and rotates the router identity before
  the next session is accepted.

### Packaging

- Production UCI synchronization preserves device identity state, migrates the previous default
  key path and exposes the tunnel credential epoch without storing private material in UCI.

### Validation

- Go tests cover key generation, epoch mismatch handling, host-key pinning, strict SSH arguments
  and explicit reverse-forward binds.

## server-v0.9.6

Agent update result reconciliation hotfix.

### Fixed

- A heartbeat that reports the exact requested agent version now reconciles an interrupted
  update result as successful, because the running version is the authoritative health check.
- Recovered rollout devices no longer remain failed or keep a rollout paused after the target
  version has reconnected successfully.

### Validation

- Store coverage reproduces an interrupted package-manager result followed by a heartbeat from
  the requested agent version.

## agent-v0.6.14

Self-update completion reporting hotfix.

### Fixed

- Managed APK and IPK upgrades mark their package-manager process as a self-update so package
  hooks do not restart the agent before it reports the successful command result.
- Manual package upgrades retain the existing automatic service restart behavior.

### Validation

- Go tests cover command-result reconciliation, while the package matrix validates both package
  formats and their installation hooks.

## server-v0.9.5

Canonical package repository endpoint hotfix.

### Fixed

- The built-in stable manifest and signature URLs now use the configured GitHub Pages
  custom domain directly instead of an address that responds with HTTP 301.
- Signed-manifest refresh continues to reject cross-origin redirects while succeeding
  against the canonical package origin.
- The offline fallback now matches the published agent `0.6.12`, and successful startup
  verification records the trusted manifest version in server logs.

## agent-v0.6.13

OpenWrt APK repository compatibility hotfix.

### Fixed

- Signed manifests now provide the direct `packages.adb` URL required by OpenWrt 25.12
  instead of a directory that makes `apk` request Alpine-style `APKINDEX.tar.gz` paths.
- All package, key, manifest, and feed metadata uses the canonical
  `packages.daemonlord.ru` origin and therefore avoids GitHub Pages redirects.

### Validation

- Repository tests cover both the directory-style IPK feed and direct APK database URL.

## server-v0.9.4

Notification center layout hotfix.

### Fixed

- Notification entries retain their intrinsic height inside the scrollable dialog instead
  of shrinking and allowing their descriptions to overlap adjacent entries.
- The notification center now stays within the dynamic viewport, prevents horizontal
  overflow, and uses a compact two-column entry layout on phones.

### Validation

- Browser coverage now renders fifty long unread notifications at the reported 528×760
  viewport and verifies separation, scrolling, and absence of horizontal overflow.

## agent-v0.6.12

UCI runtime-configuration synchronization hotfix.

### Fixed

- Synchronization no longer exits under `set -e` when an already enrolled router has no
  `enrollment_token` option to delete.
- Changes to multi-target connectivity checks now reach `/etc/rmm-agent.conf` after
  enrollment instead of leaving the previous value in place.

### Validation

- A mocked UCI regression test covers the missing enrollment token and verifies that all
  configured check targets and the existing device identity are preserved.

## agent-v0.6.11

Release workflow correction for the verified package-update agent.

### Fixed

- Agent version extraction in both current and legacy package workflows now supports the
  grouped Go `const` declaration used by the agent source.
- The branch quality job now verifies that the source and production package versions are
  non-empty and identical, so this failure is detected before a release tag is created.

### Compatibility

- Runtime behavior is unchanged from `0.6.10`; this version supersedes the signed
  `agent-v0.6.10` tag whose package matrix stopped before producing release artifacts.

## server-v0.9.3

Stabilization release for managed agent updates and rollback operations.

### Added

- Update and rollback history now exposes installation, reconnect verification, and final
  health state on each router.
- Rollouts wait for a heartbeat reporting the exact target version before advancing to the
  next batch.
- A reconciliation worker pauses rollouts when a queued operation or reconnect verification
  exceeds its safety deadline.
- Admin-only, per-device rollback uses ECDSA-verified historical stable manifests, trusted
  manifest URL boundaries, inventory compatibility checks, and immutable version-pinned packages.

### Fixed

- The built-in stable-agent fallback now matches the published `0.6.9` release.
- The release documentation now reflects `server-v0.9.1`, `server-v0.9.2`, and
  `agent-v0.6.9` instead of describing already published releases as pending.
- `server-v0.9.2` included the rollback endpoint while its release notes still described it
  as unavailable; this release establishes the implemented behavior as supported.

### Security and operations

- Server commands include signed-manifest coordinates for agents capable of independent
  verification while preserving the one-time compatibility path from agent `0.6.9`.
- `RMM_AGENT_RECONNECT_TIMEOUT_SECONDS` controls the rollout safety deadline and defaults
  to 300 seconds.

## agent-v0.6.10

Managed package verification and post-install health reporting.

### Added

- The agent independently downloads and verifies the ECDSA-signed update manifest and
  requires an exact release, target, format, feed, and package-version match.
- The production package installs the trusted ECDSA/APK and usign repository public keys.
- Successful package-manager execution is followed by an installed-version and executable
  binary health check before the agent restarts.
- Update results explicitly enter `waiting_reconnect`; the server confirms `healthy` only
  after the new agent reports the requested version.

### Compatibility

- Agent `0.6.9` remains able to perform its first managed upgrade without the new manifest
  fields. Agent `0.6.10` and later require the signed manifest coordinates.

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

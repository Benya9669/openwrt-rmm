# Recent Progress

Updated: 2026-08-24.

## Implemented in `main`

- Go agent `0.8.0` source with stable lock cleanup, tunnel endpoint validation, active LAN
  client probes and resilient UCI-to-runtime configuration synchronization.
- OpenWrt IPK/APK packaging for the current matrix plus a manual legacy tier.
- LuCI application with English as the default language and optional
  `luci-i18n-rmm-agent-ru`.
- Signed OpenWrt package feeds, GitHub Pages publishing, package provenance and
  server/tunnel image SBOM/Cosign support.
- Signed stable update metadata with package compatibility, server-side signature
  verification and Semantic Version comparison in the dashboard.
- Notification delivery through e-mail, Telegram and signed webhooks.
- Contact verification, quiet hours/timezone, maintenance pause and per-device overrides.
- Durable delivery queue with lease recovery, retries, dead-letter and retention.
- Built-in notification center with unread state, incident grouping and SSE refresh.
- Persistent LAN client presence with online, recent and unconfirmed states.
- Notification delivery metrics, channel diagnostics and filtered delivery history.
- Separate Profile, Security, Notifications and administrator-only Users tabs.
- LAN inventory filtering for failed/stale neighbour noise and IPv4 preference per MAC.
- Cloud-only wildcard router addressing; legacy DirectDNS routes are removed.
- Managed single-device updates, canary rollout, signed historical rollback, reconnect
  verification and package health reporting.
- Opt-in secure tunnel mode with per-device Ed25519 keys, pinned SSH host keys,
  port-scoped dynamic authorization, key epochs, transactional session limits and
  active listener termination after revoke, close or expiry.
- A complete infrastructure-console UI redesign with unified dialogs, status patterns,
  responsive records, Tabler icons and public/error surfaces.
- Device-bound Ed25519 command signatures, expiry/nonces, durable replay protection,
  two-phase agent credential rotation and emergency credential revocation.
- Context-bound AES-256-GCM encryption for notification secrets, pending credentials and
  payloads; key IDs are pinned to SQLite. Managed sysupgrade backups add encrypted storage,
  compatibility guards, heartbeat-confirmed automatic rollback and consistent streamed
  SQLite snapshots.

## Release state

- `server-v0.9.0`, `server-v0.9.1`, and `server-v0.9.2` are published.
- `agent-v0.6.9` is published with managed update support and immutable retained feeds.
- `server-v0.9.3` is published with verified update/rollback operations and
  reconnect-aware rollout safety.
- The signed `agent-v0.6.10` tag produced no package release because the matrix used an
  obsolete source-version parser. The corrected `agent-v0.6.11` matrix was cancelled;
  its published tag remains immutable and is superseded by `0.6.12`.
- Agent `0.6.12` fixes post-enrollment UCI synchronization when the one-time enrollment
  token has already been removed. Server `0.9.4` fixes compressed notification entries.
- The production `0.6.9 → 0.6.12` upgrade completed, exposing a redirected manifest origin
  and an Alpine-style APK index lookup; `0.9.5`/`0.6.13` use the canonical origin and
  direct OpenWrt `packages.adb` URL.
- Agent `0.6.14` and server `0.9.6` are published. The agent defers package-hook restart
  until after result delivery, while the server reconciles the outcome from the reported
  running version. The production update completed successfully.
- Server `0.10.0` and agent `0.7.0` are prepared as the secure-tunnel release pair. They
  add per-device SSH identities, port-scoped authorization, host-key pinning, credential
  rotation, session limits, WAN neighbour filtering and a compact 1366×768 client table.
- Server `0.10.1` is prepared as a deployment patch: the base Compose file pulls published
  images directly, preserves existing data through explicit volume names, and shows the
  running server version in the authenticated UI.
- Server `0.10.2` fixes secure tunnel authorization under OpenSSH's sanitized
  `AuthorizedKeysCommand` environment by using protected runtime credential files.
- Server `0.11.0` is published with the complete UI redesign. Server `0.11.1` refreshes
  application icons and `0.11.2` aligns Login and cloud LuCI error states.
- Local pre-release checks pass for the `0.9.3` server image and for unsigned OpenWrt
  25.12.4 ramips/mt7621 APK artifacts (agent, LuCI, Russian i18n, and repository index).
- The browser suite covers notification overflow in addition to the existing
  login/profile/LuCI/update/rollback/responsive scenarios.

## Next

The authoritative development order is maintained in `ROADMAP.md`. Immediate work is:

1. run migration and package smoke for `server-v0.12.0` / `agent-v0.8.0`;
2. verify command-key pinning, token rotation and emergency revoke on a test router;
3. perform a router backup/restore drill and preserve all three server recovery files;
4. complete production notification, LAN-client and tunnel smoke tests.

# Recent Progress

Updated: 2026-08-10.

## Implemented in `main`

- Go agent `0.6.11` source with stable lock cleanup, tunnel endpoint validation, active LAN
  client probes.
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

## Release state

- `server-v0.9.0`, `server-v0.9.1`, and `server-v0.9.2` are published.
- `agent-v0.6.9` is published with managed update support and immutable retained feeds.
- `server-v0.9.3` is published with verified update/rollback operations and
  reconnect-aware rollout safety.
- The signed `agent-v0.6.10` tag produced no package release because the matrix used an
  obsolete source-version parser. Agent `0.6.11` supersedes it with pre-tag validation.
- Local pre-release checks pass for the `0.9.3` server image and for unsigned OpenWrt
  25.12.4 ramips/mt7621 APK artifacts (agent, LuCI, Russian i18n, and repository index).
- The browser suite covers 12 login/profile/LuCI/update/rollback/responsive scenarios.

## Next

The authoritative development order is maintained in `ROADMAP.md`. Immediate work is:

1. publish the signed `agent-v0.6.11` 24.10/25.12 matrix and test `0.6.9 → 0.6.11` on a real router;
2. test reconnect timeout, rollout pause/resume and signed historical rollback;
3. deploy `server-v0.9.3` and agent `0.6.11` after a production database backup;
4. complete production notification, LAN-client and tunnel smoke tests;
5. move per-device tunnel credentials and signed/replay-protected commands ahead of
   backup/restore and remote update work.

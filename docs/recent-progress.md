# Recent Progress

Updated: 2026-07-31.

## Implemented in `main`

- Go agent `0.6.8` with stable lock cleanup, tunnel endpoint validation and active LAN
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

## Release state

- `server-v0.9.0` is published with the notification center, verified channels and LAN
  client persistence.
- `agent-v0.6.8` is a signed tag; its complete current OpenWrt matrix and Pages deployment
  finished successfully.
- `server-v0.9.1` is being prepared as a stabilization release for notification
  operations, account navigation and LAN inventory cleanup.

## Next

The authoritative development order is maintained in `ROADMAP.md`. Immediate work is:

1. publish the signed agent update manifest and verify package installation from the feed;
2. publish and deploy `server-v0.9.1` after a production database backup;
3. complete production notification, LAN-client and tunnel smoke tests;
4. move per-device tunnel credentials and signed/replay-protected commands ahead of
   backup/restore and remote update work.

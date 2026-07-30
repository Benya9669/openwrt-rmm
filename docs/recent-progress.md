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
- Cloud-only wildcard router addressing; legacy DirectDNS routes are removed.

## Release state

- `server-v0.8.1` is published but predates the latest notification center and LAN client
  changes.
- `agent-v0.6.8` is a signed tag; its complete current OpenWrt matrix and Pages deployment
  finished successfully.
- `server-v0.9.0` is being prepared to include the current `main`, use a signed immutable tag and
  be validated against a copy of the production SQLite database.

## Next

The authoritative development order is maintained in `ROADMAP.md`. Immediate work is:

1. finish and verify the `agent-v0.6.8` package release;
2. publish and deploy a signed server release containing the current `main`;
3. complete production notification, LAN-client and tunnel smoke tests;
4. add notification operational metrics;
5. move per-device tunnel credentials and signed/replay-protected commands ahead of
   backup/restore and remote update work.

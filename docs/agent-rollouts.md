# Managed Agent Rollouts

Admins create an agent rollout for explicit device IDs, a `stable` or `candidate` channel,
batch size, and failure threshold (default `1`). The server snapshots only supported production
Go agents for which the selected signed manifest has an exact immutable feed compatible with the
reported OpenWrt release, target, and package manager.

Only the initial batch is queued. A successful package-manager exit moves the device to
`waiting_reconnect`; the next batch is queued only after a heartbeat reports the exact target
agent version. Failed commands and reconnect timeouts count toward the threshold and automatically
pause the rollout. Operators may pause, resume, or cancel; paused and cancelled rollouts never
queue new commands. Cancellation also cancels commands which have not yet been claimed.

The server records `rollout_id`, `channel`, `target_version`, immutable feed, package version,
manifest URL, and signature URL in every generated `agent_update` command. Rollout API access is
restricted to administrators and actions are audited. The reconnect deadline is controlled by
`RMM_AGENT_RECONNECT_TIMEOUT_SECONDS` and defaults to five minutes.

Candidate is offered only when `RMM_CANDIDATE_UPDATE_MANIFEST_URL` and
`RMM_CANDIDATE_UPDATE_MANIFEST_SIGNATURE_URL` configure a manifest that verifies with
`RMM_UPDATE_MANIFEST_PUBLIC_KEY`. If it cannot be verified during startup, candidate rollout
creation is rejected and the UI marks candidate unavailable.

## Per-device rollback

Admins can queue an explicit rollback with `POST /api/devices/{id}/agent-rollback` and JSON
`manifest_url` plus `signature_url`. Both URLs must be HTTPS and remain under the configured
update manifest's origin and directory. The server verifies the supplied stable-channel historical
manifest using the configured ECDSA key, selects only the device-compatible immutable package entry,
and rejects targets that are not lower than the device's reported `agent_version`.

The queued command is version-pinned (`feed_url`, package name, and package version). Agent `0.6.10`
and later independently verify the ECDSA signature and require the signed manifest to contain the
local OpenWrt release, target, package format, feed, and package version. With agent
`0.10.1` and a matching server update, stable APK patch releases within `25.12`
share compatibility (for example a `25.12.4` SDK package on `25.12.5`). Other
OpenWrt lines still require an exact release. The agent advertises this capability
as `agent_update_release_line_25_12`; older agents on a differing patch need a
one-time native APK upgrade before managed updates, schedules or rollout waves
can use that feed. Targets, immutable package coordinates, ECDSA manifest and native
APK signatures remain required. Snapshot and release-candidate builds do not
receive the stable release-line fallback. SDK-pinned feed directory names remain
unchanged; the theme is unaffected. Verify a heartbeat advertises the capability
and queue an update on a test router before expanding the rollout. The Go agent then
uses `apk add --allow-downgrade` or a version-pinned `opkg install`, verifies the installed package
version and executable, persists `waiting_reconnect`, and restarts itself. The server changes the
operation to `healthy` only after a subsequent heartbeat reports the requested version.

The compatibility upgrade from `0.6.9` does not include manifest coordinates because that agent
predates their allowlist. Package-manager feed signature verification still applies. Every command
issued to `0.6.10` or later requires independent manifest verification.

For explicit canaries, growing waves, observation periods and delivery guards, see [management development](management-development.md#обновления-волнами). The guarded mode requires agents that independently verify signed manifests; the existing fixed-batch workflow remains available.

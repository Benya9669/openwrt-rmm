# Managed Agent Rollouts

Admins create an agent rollout for explicit device IDs, a `stable` or `candidate` channel,
batch size, and failure threshold (default `1`). The server snapshots only supported production
Go agents for which the selected signed manifest has an exact immutable feed compatible with the
reported OpenWrt release, target, and package manager.

Only the initial batch is queued. Each completed successful `agent_update` queues the next batch
after the prior batch has finished. Failed commands count toward the threshold and automatically
pause the rollout. Operators may pause, resume, or cancel; paused and cancelled rollouts never
queue new commands. Cancellation also cancels commands which have not yet been claimed.

The server records `rollout_id`, `channel`, `target_version`, and `feed_url` in every generated
`agent_update` command. Rollout API access is restricted to administrators and actions are audited.

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

The queued command is version-pinned (`feed_url`, package name, and package version). The Go agent
accepts only package-operation metadata, uses `apk add --allow-downgrade` or a version-pinned
`opkg install`, persists the result, then restarts itself only after success.

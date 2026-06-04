# Remote Access

Remote access is now implemented as an MVP reverse SSH session flow. The server stores short-lived session records, queues a `remote_ssh_reverse` command for the agent, and records create/close events in the audit log.

## Current Capabilities

- Temporary SSH access through an outbound reverse tunnel.
- LuCI access through an authenticated RMM HTTP proxy over the same tunnel.
- Session list and close action in the web UI.
- Session audit with device, command id, endpoint, and expiration.
- Automatic session expiration in the server store.
- Docker/Linux-friendly contract: the RMM host or container must provide the SSH tunnel endpoint.

## Flow

1. Operator opens a remote SSH session for a device.
2. Server creates a `remote_sessions` record with expiration.
3. Server queues `remote_ssh_reverse` for the router.
4. Agent runs `ssh` or `dbclient` and requests reverse forwards for router SSH and LuCI.
5. Operator connects to SSH or opens LuCI through the authenticated RMM proxy.
6. Session expires automatically on the router or is closed by an operator command.

## Docker/Linux Requirement

The reverse tunnel needs an SSH server reachable by the router. The default command uses:

- user: `rmm-tunnel`
- server port: `22`
- remote router port: `22`
- duration: `15 minutes`

The included Compose stack exposes the SSH endpoint on port `2222` and operator SSH ports `22000-22099`. Internal LuCI ports `22100-22199` stay inside the Compose network. See [docker-compose.md](docker-compose.md) for deployment and router key installation.

Select HTTP or HTTPS when opening the session to match the router's LuCI listener. HTTPS supports router-local self-signed certificates inside the tunnel.

## Safety Rules

- No permanent inbound router ports.
- Sessions are time-bound.
- All session opens and closes are audited.
- The agent only accepts safe host/user/port characters.
- Future hardening should add per-session credentials and stronger approval policy.

## Next Hardening

- Per-session SSH keys.
- Dedicated tunnel sidecar container.
- Server-side active tunnel health checks.
- Browser terminal proxy after the tunnel endpoint is reliable.

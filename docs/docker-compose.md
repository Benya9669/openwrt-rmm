# Docker Compose Deployment

The Compose stack contains two services:

- `rmm-server`: Go API, web UI, and persistent SQLite database.
- `tunnel-ssh`: SSH endpoint used by routers for reverse SSH tunnels.

Published ports:

- `18080/tcp`: RMM API and web UI.
- `2222/tcp`: router-to-server SSH tunnel connection.
- `22000-22099/tcp`: temporary operator endpoints, bound to loopback by default.
- `22100-22199/tcp`: internal LuCI proxy forwards; these are not published by Compose.

## 1. Configure Secrets

Create the local environment file:

```powershell
Copy-Item .env.example .env
notepad .env
```

Set a long `RMM_OPERATOR_PASSWORD`. Shared enrollment is disabled: users add routers with
15-minute one-time grants created in the web UI. Leave `RMM_OPERATOR_TOKEN` empty unless
an emergency/API bearer token is required.

To enable password recovery and e-mail alert notifications, set `RMM_PUBLIC_URL` to the
external HTTPS address and configure SMTP in `.env`:

```dotenv
RMM_PUBLIC_URL=https://rmm.example.com
RMM_SMTP_HOST=smtp.example.com
RMM_SMTP_PORT=587
RMM_SMTP_USERNAME=rmm@example.com
RMM_SMTP_PASSWORD=replace-with-the-smtp-password
RMM_SMTP_FROM=OpenWrt RMM <rmm@example.com>
RMM_SMTP_TLS_MODE=starttls
```

Use `RMM_SMTP_TLS_MODE=tls` for implicit TLS on port 465. Plain SMTP is rejected in
production mode. Reset links are one-time, expire after 30 minutes, and revoke all
existing web sessions after the password is changed.

Telegram notifications use one server-side bot token and a per-user numeric Chat ID:

```dotenv
RMM_TELEGRAM_BOT_TOKEN=replace-with-the-token-from-botfather
RMM_NOTIFICATION_MAX_ATTEMPTS=5
RMM_NOTIFICATION_RETENTION_DAYS=90
```

Do not commit this token. After deployment, each user enables Telegram and enters their
Chat ID in **Profile → Notifications and thresholds**, then uses **Send test**.
Failed deliveries are retried with exponential backoff. Terminal `sent` and `dead_letter`
history is removed after `RMM_NOTIFICATION_RETENTION_DAYS`; pending work is never removed
by retention maintenance.

## 2. Start The Stack

For a production deployment, pin the server release in `.env` and use the release
overlay. This pulls the server and tunnel images published by the same `server-v*` tag:

```dotenv
RMM_RELEASE_VERSION=0.8.0
```

```powershell
docker compose -f compose.yaml -f compose.release.yaml pull
docker compose -f compose.yaml -f compose.release.yaml up -d
docker compose -f compose.yaml -f compose.release.yaml ps
```

Keep the exact version instead of `latest` so an upgrade is deliberate and reversible.
The base Compose file remains buildable from source for development and emergency
recovery:

```powershell
docker compose up -d --build
docker compose ps
docker compose logs --tail 100
```

For a local HTTP-only lab, explicitly set `RMM_INSECURE_DEV_MODE=true` and
`RMM_COOKIE_SECURE=false`. Production should use the HTTPS overlays. Open:

```text
http://127.0.0.1:18080
```

The SQLite database and SSH keys are stored in named Docker volumes.

For HTTPS and domain-based access, use [npmplus.md](npmplus.md) or the optional Caddy overlay described in [reverse-proxy.md](reverse-proxy.md). The wildcard device-domain setup is documented in [keendns.md](keendns.md).

## 3. Generate And Install The Tunnel Key

Generate the router tunnel key before starting Compose:

```sh
mkdir -p secrets
ssh-keygen -t ed25519 -N '' -C rmm-router-tunnel -f secrets/router_tunnel_key
chmod 600 secrets/router_tunnel_key
```

Compose mounts only `secrets/router_tunnel_key.pub` into the SSH sidecar. The private key remains on the deployment host and must be installed on approved routers.

OpenWrt 25 uses `apk`. Install the OpenSSH client:

```sh
apk add openssh-client
```

Install the key without `scp` or SFTP:

```powershell
Get-Content -Raw .\secrets\router_tunnel_key | ssh root@10.10.10.1 "umask 077; mkdir -p /etc/rmm-agent; cat > /etc/rmm-agent/tunnel_key; chmod 600 /etc/rmm-agent/tunnel_key"
```

Update `/etc/rmm-agent.conf`:

```sh
TUNNEL_IDENTITY_FILE="/etc/rmm-agent/tunnel_key"
```

Deploy the latest agent and restart it:

```powershell
Get-Content -Raw .\agent\openwrt\rmm-agent.sh | ssh root@10.10.10.1 "cat > /usr/bin/rmm-agent; chmod +x /usr/bin/rmm-agent; /etc/init.d/rmm-agent restart"
```

## 4. Open A Remote Session

In the device Remote access panel use:

- Tunnel server: the Docker host IP reachable by the router, for example `10.10.10.2`.
- Server SSH port: `2222`.
- Remote port: leave empty for automatic selection.
- Router SSH port: `22`.
- Duration: `15 min`.

After the agent reports success, connect using the command shown in the UI:

```sh
ssh -p 22022 root@10.10.10.2
```

The actual port is selected per session.

## Operations

Administrators can create users, change their roles, disable accounts, and issue a
temporary password from the profile dialog. Every user receives routers through their
own one-time enrollment grants. A router can be transferred from its Expert tab after
the current user confirms their password; active LuCI access is closed during transfer.

The public landing page is served at `/`, sign-in and password recovery at `/login`, and
the authenticated cabinet at `/app`. Live updates use authenticated Server-Sent Events
at `/api/events`. The server disables proxy buffering and sends keep-alives every 20
seconds; if a proxy interrupts the stream, the browser falls back to a 30-second poll.

View logs:

```powershell
docker compose logs -f rmm-server
docker compose logs -f tunnel-ssh
```

Restart:

```powershell
docker compose restart
```

Stop without deleting data:

```powershell
docker compose down
```

Back up the SQLite database:

```powershell
docker compose cp rmm-server:/data/rmm.db .\tmp\rmm-backup.db
```

## Security Notes

- Restrict ports `18080`, `2222`, and `22000-22999` with the host firewall.
- Port `2222` must be reachable by managed routers.
- Ports `22000-22099` should only be reachable by trusted operators or VPN clients.
- Use long random enrollment/operator tokens.
- SSH shell, PTY and SFTP sessions are disabled on the tunnel account; only remote forwarding is allowed.
- The current stack still uses one persistent router tunnel key. Per-device SSH certificates are the next hardening step.

# Docker Compose Deployment

The Compose stack contains two services:

- `rmm-server`: Go API, web UI, and persistent SQLite database.
- `tunnel-ssh`: SSH endpoint used by routers for reverse SSH tunnels.

The base `compose.yaml` pulls versioned GHCR images and is self-contained for GitOps
controllers that accept only one Compose file. `compose.release.yaml` remains compatible
with older deployments but is no longer required. Source builds use `compose.dev.yaml`.

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
RMM_BACKUP_RETENTION_DAYS=90
RMM_STABLE_AGENT_VERSION=0.8.0
RMM_AGENT_RECONNECT_TIMEOUT_SECONDS=300
RMM_UPDATE_MANIFEST_URL=https://packages.daemonlord.ru/update-manifest.json
RMM_UPDATE_MANIFEST_SIGNATURE_URL=https://packages.daemonlord.ru/update-manifest.sig
RMM_UPDATE_MANIFEST_PUBLIC_KEY=/app/keys/rmm-openwrt.pem
RMM_COMMAND_SIGNING_KEY_PATH=/data/command-signing-ed25519.pem
RMM_DATA_ENCRYPTION_KEY_PATH=/data/data-encryption.key
```

Do not commit this token. After deployment, each user enables Telegram and enters their
Chat ID in **Profile → Notifications and thresholds**, then uses **Send test**.
Failed deliveries are retried with exponential backoff. Terminal `sent` and `dead_letter`
history is removed after `RMM_NOTIFICATION_RETENTION_DAYS`; pending work is never removed
by retention maintenance.

## 2. Start The Stack

For a production deployment, pin the server release in `.env`. The base Compose file pulls
the server and tunnel images published by the same `server-v*` tag:

```dotenv
RMM_RELEASE_VERSION=0.12.3
```

```powershell
docker compose pull
docker compose up -d
docker compose ps
```

Keep the exact version instead of `latest` so an upgrade is deliberate and reversible.
The base Compose file remains buildable from source for development and emergency
recovery:

```powershell
docker compose -f compose.yaml -f compose.dev.yaml up -d --build
docker compose ps
docker compose logs --tail 100
```

### Upgrade order for server 0.12 and agent 0.8

Deploy the server and tunnel images first. On its first successful start, server 0.12
creates `/data/command-signing-ed25519.pem` and `/data/data-encryption.key`, pins both key
identifiers to the SQLite database, and encrypts existing sensitive notification fields.
Only then upgrade routers to agent 0.8, which pins the command-signing public key received
from the server and rejects unsigned, expired, replayed, or incorrectly bound commands.
Agent 0.8 must not be deployed against an older server because the older heartbeat response
does not contain a command-signing key.

Treat the SQLite snapshot and both key files as one recovery set. A restored database with
different keys is rejected at startup instead of silently making encrypted records or
enrolled agents unusable. Copy the key files to encrypted offline storage after the first
0.12 start; never commit or attach them to a release.

This is field-level protection, not full SQLite/SQLCipher encryption. Command handoff
tokens, notification destinations and payloads, verification destinations, webhook
secrets, and router backup archives are encrypted with context-bound AES-256-GCM; password
material and access tokens are stored as one-way hashes. Inventory, metrics, usernames,
e-mail addresses, and audit metadata remain readable to the database process. Use encrypted
host storage as well when offline theft of the complete RMM volume is in scope.

An image-only rollback to 0.11.x is not safe after the field migration because older
servers do not decrypt the new records and agent 0.8 expects signed-command metadata.
Rollback must restore the pre-upgrade RMM data volume (or its consistent SQLite snapshot)
together with the 0.11.x image and compatible agents. Keep that recovery set until the
0.12 production smoke and router restore drill are complete.

### Recreate a GitOps stack without losing data

The SQLite database and persistent SSH host keys live in two named volumes. Before moving
the stack to Arcane or changing its project name, pause automatic reconciliation and find
the exact existing volume names:

```sh
docker inspect "$(docker ps -q --filter label=com.docker.compose.service=rmm-server | head -n1)" \
  --format '{{range .Mounts}}{{println .Name .Destination}}{{end}}'
docker inspect "$(docker ps -q --filter label=com.docker.compose.service=tunnel-ssh | head -n1)" \
  --format '{{range .Mounts}}{{println .Name .Destination}}{{end}}'
```

Back up both volumes while the two services are stopped. Replace the example names below
with the names reported by `docker inspect`:

```sh
mkdir -p backups
docker stop openwrt-rmm-rmm-server-1 openwrt-rmm-tunnel-ssh-1
docker run --rm -v openwrt-rmm_rmm-data:/source:ro -v "$PWD/backups:/backup" \
  alpine:3.22 sh -c 'tar czf /backup/rmm-data.tgz -C /source .'
docker run --rm -v openwrt-rmm_tunnel-data:/source:ro -v "$PWD/backups:/backup" \
  alpine:3.22 sh -c 'tar czf /backup/tunnel-data.tgz -C /source .'
tar tzf backups/rmm-data.tgz | head
tar tzf backups/tunnel-data.tgz | head
```

Configure Arcane with the existing volume names and the exact image release:

```dotenv
RMM_RELEASE_VERSION=0.12.3
RMM_DATA_VOLUME=openwrt-rmm_rmm-data
RMM_TUNNEL_DATA_VOLUME=openwrt-rmm_tunnel-data
```

Arcane can now delete and recreate the containers and network. Do not select an option that
deletes volumes, and never run `docker compose down -v`. After deployment, verify the mounts,
health endpoint, router list, and the server version displayed below the OpenWrt RMM logo.

For a local HTTP-only lab, explicitly set `RMM_INSECURE_DEV_MODE=true` and
`RMM_COOKIE_SECURE=false`. Production should use the HTTPS overlays. Open:

```text
http://127.0.0.1:18080
```

The SQLite database and SSH keys are stored in named Docker volumes.

For HTTPS and domain-based access, use [npmplus.md](npmplus.md) or the optional Caddy overlay described in [reverse-proxy.md](reverse-proxy.md). The wildcard device-domain setup is documented in [keendns.md](keendns.md).

## 3. Enable Per-device Tunnel Keys

The secure mode does not distribute one shared private key. Agent `0.7.0` creates
`/etc/rmm-agent/tunnel_device_key` locally with mode `0600` and reports only its public
key during heartbeat.

Use a staged migration so existing routers do not lose remote access:

1. Deploy the new server, tunnel image and agent while `RMM_TUNNEL_AUTH_TOKEN` is empty.
2. Wait until every online router has sent at least one heartbeat with its per-device key.
3. Read the persistent SSH host public key:

```sh
docker compose exec tunnel-ssh cat /data/ssh_host_ed25519_key.pub
```

4. Generate an independent internal authorization token:

```sh
openssl rand -hex 32
```

5. Put both values in `.env`; quote the host-key line because it contains spaces:

```dotenv
RMM_TUNNEL_AUTH_TOKEN=<64 hexadecimal characters>
RMM_TUNNEL_HOST_PUBLIC_KEY="ssh-ed25519 AAAA..."
```

6. Recreate only the two affected services during a maintenance window. This terminates
   currently open tunnel sessions but does not touch the database volume:

```sh
docker compose up -d --force-recreate rmm-server tunnel-ssh
```

When the authorization token is set, the SSH sidecar empties the legacy static
`authorized_keys` file. New authentications are resolved by key fingerprint through the
internal server API and are restricted to the ports of a non-expired session.

### Legacy rollback

If migration must be rolled back, remove `RMM_TUNNEL_AUTH_TOKEN` from both services and
recreate them. The following shared-key procedure is retained only for that rollback.

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

Legacy agents use this setting in `/etc/rmm-agent.conf`:

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

Create a consistent SQLite backup from **Expert → Maintenance → Download database snapshot**.
The server uses SQLite `VACUUM INTO`, so the downloaded file is a self-contained snapshot
rather than a copy of the live WAL-backed database. Router sysupgrade backups are managed
from the device **Operations** tab, encrypted in the database, and removed after
`RMM_BACKUP_RETENTION_DAYS`.

The `/data/command-signing-ed25519.pem` and `/data/data-encryption.key` files are generated
on first start. Keep encrypted offline copies together with the database snapshot. Losing
the data-encryption key makes encrypted notification fields and router backups unrecoverable;
replacing the command-signing key requires explicit agent re-enrollment because agents pin it.

## Security Notes

- Restrict ports `18080`, `2222`, and `22000-22199` with the host firewall.
- Port `2222` must be reachable by managed routers.
- Ports `22000-22099` should only be reachable by trusted operators or VPN clients.
- Use long random enrollment/operator tokens.
- SSH shell, PTY and SFTP sessions are disabled on the tunnel account; only remote forwarding is allowed.
- In secure mode the tunnel sidecar polls the authenticated active-port endpoint every five seconds and terminates listeners whose session was revoked, closed or expired. If the server cannot be reached, it retries without dropping otherwise valid maintenance access.
- Keep `RMM_TUNNEL_AUTH_TOKEN` independent from user, device, enrollment and session tokens.
- Never commit the authorization token, tunnel host private key or router device keys.
- Never commit the command-signing or data-encryption keys; protect backups of both keys separately from the database.
- Roll out secure mode in two stages; enabling it before agent `0.7.0` heartbeats will intentionally reject legacy shared-key authentication.

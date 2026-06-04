# Docker Compose Deployment

The Compose stack contains two services:

- `rmm-server`: Go API, web UI, and persistent SQLite database.
- `tunnel-ssh`: SSH endpoint used by routers for reverse SSH tunnels.

Published ports:

- `18080/tcp`: RMM API and web UI.
- `2222/tcp`: router-to-server SSH tunnel connection.
- `22000-22099/tcp`: temporary operator endpoints created by remote sessions.

## 1. Configure Secrets

Create the local environment file:

```powershell
Copy-Item .env.example .env
notepad .env
```

Replace `RMM_ENROLLMENT_TOKEN` and `RMM_OPERATOR_TOKEN` before exposing the server outside a trusted network.

## 2. Start The Stack

```powershell
docker compose up -d --build
docker compose ps
docker compose logs --tail 100
```

Open:

```text
http://127.0.0.1:18080
```

The SQLite database and SSH keys are stored in named Docker volumes.

For HTTPS and domain-based access, use [npmplus.md](npmplus.md) or the optional Caddy overlay described in [reverse-proxy.md](reverse-proxy.md).

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
- Current MVP uses one persistent router tunnel key. Per-device or per-session keys are the next hardening step.

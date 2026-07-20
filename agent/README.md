# OpenWrt RMM Agent

Current stable Go agent: `0.5.2`. It reports runtime health, pending command results,
and the last heartbeat transport error after connectivity is restored.

Production Go agent for OpenWrt, with the shell implementation retained as a fallback runtime.

The agent uses outbound HTTP polling:

1. Enrolls with the server using an enrollment token.
2. Sends heartbeat with inventory and metrics.
3. Receives queued commands from the heartbeat response.
4. Executes only allowlisted commands.
5. Sends command results back to the server.

## Config

Default config path:

```sh
/etc/rmm-agent.conf
```

Example:

```sh
SERVER_URL="https://rmm.example.com"
ENROLLMENT_TOKEN="paste-a-one-time-grant-from-your-account"
INTERVAL_SECONDS="30"
TUNNEL_IDENTITY_FILE="/etc/rmm-agent/tunnel_key"
```

After enrollment the agent writes:

```sh
DEVICE_ID="..."
DEVICE_TOKEN="..."
```

## Run

```sh
sh ./agent/openwrt/rmm-agent.sh
```

## Go Agent

The Go agent lives at:

```text
agent/go/cmd/rmm-agent
```

It is protocol-compatible with the server for enrollment, heartbeat, inventory, metrics, command polling, command result reporting, lock handling, backoff, graceful shutdown, and result spooling. Its inventory payload includes system metadata, interfaces, routes, WAN IP, DHCP leases, kernel neighbor state, Wi-Fi clients, memory, disk, interface counters, package manager metadata, and connectivity checks. The UI uses live Wi-Fi and neighbor state to distinguish connected clients from static or infinite DHCP leases.

The Go agent is available as the production OpenWrt package and supports the migrated command allowlist:

- `ping`
- `traceroute`
- `route_show`
- `interfaces_show`
- `reboot`
- `service_restart`
- `pkg_list_installed` / `opkg_list_installed`
- `pkg_update` / `opkg_update`
- `pkg_list_upgradable` / `opkg_list_upgradable`
- `pkg_install` / `opkg_install`
- `pkg_remove` / `opkg_remove`
- `uci_show`
- `uci_backup`
- `uci_preview`
- `uci_set`
- `uci_commit`
- `uci_commit_confirmed`
- `uci_revert`
- `uci_restore`
- `remote_ssh_reverse`
- `remote_ssh_close`

Other queued commands are reported as failed with a clear message.

Build locally:

```sh
go build -o ./tmp/rmm-agent-go ./agent/go/cmd/rmm-agent
```

Run once against an existing config:

```sh
./tmp/rmm-agent-go -config /etc/rmm-agent.conf -once
```

For side-by-side testing, use a separate config and hostname suffix so the Go agent enrolls as a separate device:

```sh
cp /etc/rmm-agent.conf /etc/rmm-agent-go.conf
sed -i '/^DEVICE_ID=/d;/^DEVICE_TOKEN=/d' /etc/rmm-agent-go.conf
cat >>/etc/rmm-agent-go.conf <<'EOF'
HOSTNAME_SUFFIX="-go"
LOCK_FILE="/tmp/rmm-agent-go.lock"
SPOOL_DIR="/tmp/rmm-agent-go-results"
BACKUP_DIR="/tmp/rmm-agent-go-backups"
TUNNEL_STATE_DIR="/tmp/rmm-agent-go-tunnels"
EOF
./tmp/rmm-agent-go -config /etc/rmm-agent-go.conf -once
```

Side-by-side procd service:

```sh
cp ./tmp/rmm-agent-go /usr/bin/rmm-agent-go
chmod +x /usr/bin/rmm-agent-go
cp ./agent/openwrt/rmm-agent-go.init /etc/init.d/rmm-agent-go
chmod +x /etc/init.d/rmm-agent-go
cp ./agent/openwrt/rmm-agent-go.conf /etc/rmm-agent-go.conf
vi /etc/rmm-agent-go.conf
/etc/init.d/rmm-agent-go enable
/etc/init.d/rmm-agent-go start
```

The side-by-side service uses `/etc/rmm-agent-go.conf`, `/tmp/rmm-agent-go.lock`, `/tmp/rmm-agent-go-results`, `/tmp/rmm-agent-go-backups`, and `/tmp/rmm-agent-go-tunnels` so it does not collide with the shell agent.

Production switch from the shell agent to the Go agent:

```sh
/etc/init.d/rmm-agent stop
/etc/init.d/rmm-agent disable
/etc/init.d/rmm-agent-go stop 2>/dev/null || true
cp /usr/bin/rmm-agent-go /usr/bin/rmm-agent
chmod +x /usr/bin/rmm-agent
cp ./agent/openwrt/rmm-agent-go-production.init /etc/init.d/rmm-agent
chmod +x /etc/init.d/rmm-agent
sed -i '/^HOSTNAME_SUFFIX=/d;/^HOSTNAME_OVERRIDE=/d;/^LOCK_FILE=/d;/^SPOOL_DIR=/d;/^BACKUP_DIR=/d;/^TUNNEL_STATE_DIR=/d' /etc/rmm-agent.conf
rmdir /tmp/rmm-agent.lock 2>/dev/null || true
/etc/init.d/rmm-agent enable
/etc/init.d/rmm-agent start
```

This keeps the existing `/etc/rmm-agent.conf`, including `DEVICE_ID` and `DEVICE_TOKEN`, so the router continues as the same RMM object instead of enrolling as a duplicate `-go` device. Keep the shell script backup until the Go service has been stable for at least one maintenance window.

The procd service removes the configured `LOCK_FILE` only after the agent process has
stopped. To verify the lifecycle during a maintenance window:

```sh
/etc/init.d/rmm-agent stop
test ! -e /tmp/rmm-agent.lock && echo "lock removed"
/etc/init.d/rmm-agent start
```

If `LOCK_FILE` is overridden in `/etc/rmm-agent.conf`, check that path instead.

## OpenWrt Package

Package skeleton:

```text
agent/package/rmm-agent
agent/package/rmm-agent-go
agent/package/rmm-agent-go-production
```

The shell package installs `/usr/bin/rmm-agent`, `/etc/init.d/rmm-agent`, and `/etc/rmm-agent.conf`.
The Go package installs `/usr/bin/rmm-agent-go`, `/etc/init.d/rmm-agent-go`, and `/etc/rmm-agent-go.conf`.
The production Go package installs `/usr/bin/rmm-agent`, `/etc/init.d/rmm-agent`, and `/etc/rmm-agent.conf` using the Go runtime instead of the shell script.

For Docker Compose reverse SSH access, install the generated tunnel private key at `/etc/rmm-agent/tunnel_key` with mode `600`. The agent automatically uses it for `remote_ssh_reverse`.

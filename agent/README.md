# OpenWrt RMM Agent

Production MVP shell agent for OpenWrt, plus an experimental Go agent preview.

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
SERVER_URL="http://server:8080"
ENROLLMENT_TOKEN="dev-enroll-token"
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

## Go Agent Preview

The Go agent lives at:

```text
agent/go/cmd/rmm-agent
```

It is protocol-compatible with the server for enrollment, heartbeat, inventory, metrics, command polling, command result reporting, lock handling, backoff, graceful shutdown, and result spooling. Its inventory payload includes system metadata, interfaces, routes, WAN IP, DHCP leases, Wi-Fi clients, memory, disk, interface counters, package manager metadata, and connectivity checks.

The Go agent is not packaged as the production OpenWrt agent yet, but it supports the migrated command allowlist:

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

For side-by-side testing, use a separate config and hostname suffix so the Go preview enrolls as a separate device:

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

## OpenWrt Package

Package skeleton:

```text
agent/package/rmm-agent
```

It installs the agent as `/usr/bin/rmm-agent`, the procd service as `/etc/init.d/rmm-agent`, and the default config as `/etc/rmm-agent.conf`.

For Docker Compose reverse SSH access, install the generated tunnel private key at `/etc/rmm-agent/tunnel_key` with mode `600`. The agent automatically uses it for `remote_ssh_reverse`.

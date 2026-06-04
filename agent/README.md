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

It is protocol-compatible with the server for enrollment, heartbeat, inventory, metrics, command polling, command result reporting, lock handling, backoff, graceful shutdown, and result spooling.

The Go agent is not the production command runner yet. It currently supports the first read-only command set:

- `ping`
- `traceroute`
- `route_show`
- `interfaces_show`
- `pkg_list_installed` / `opkg_list_installed`

Other queued commands are reported as failed with a clear message until the shell allowlist is migrated command by command.

Build locally:

```sh
go build -o ./tmp/rmm-agent-go ./agent/go/cmd/rmm-agent
```

Run once against an existing config:

```sh
./tmp/rmm-agent-go -config /etc/rmm-agent.conf -once
```

## OpenWrt Package

Package skeleton:

```text
agent/package/rmm-agent
```

It installs the agent as `/usr/bin/rmm-agent`, the procd service as `/etc/init.d/rmm-agent`, and the default config as `/etc/rmm-agent.conf`.

For Docker Compose reverse SSH access, install the generated tunnel private key at `/etc/rmm-agent/tunnel_key` with mode `600`. The agent automatically uses it for `remote_ssh_reverse`.

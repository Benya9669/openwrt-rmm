# OpenWrt RMM Agent

MVP shell agent for OpenWrt.

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

## OpenWrt Package

Package skeleton:

```text
agent/package/rmm-agent
```

It installs the agent as `/usr/bin/rmm-agent`, the procd service as `/etc/init.d/rmm-agent`, and the default config as `/etc/rmm-agent.conf`.

For Docker Compose reverse SSH access, install the generated tunnel private key at `/etc/rmm-agent/tunnel_key` with mode `600`. The agent automatically uses it for `remote_ssh_reverse`.

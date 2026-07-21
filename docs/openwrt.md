# OpenWrt Notes

## MVP Agent Install

Manual install example:

```sh
cp agent/openwrt/rmm-agent.sh /usr/bin/rmm-agent
chmod +x /usr/bin/rmm-agent
cp agent/openwrt/rmm-agent.init /etc/init.d/rmm-agent
chmod +x /etc/init.d/rmm-agent
```

Create config:

```sh
cat >/etc/rmm-agent.conf <<'EOF'
SERVER_URL="https://rmm.example.com"
ENROLLMENT_TOKEN="paste-a-one-time-grant-from-your-account"
INTERVAL_SECONDS="30"
CHECK_TARGETS="1.1.1.1 8.8.8.8"
TUNNEL_IDENTITY_FILE="/etc/rmm-agent/tunnel_key"
DIRECT_DNS_ENABLED="false"
DNS_UPDATE_INTERVAL_SECONDS="300"
PUBLIC_IPV4_URL="https://api.ipify.org"
PUBLIC_IPV6_URL="https://api6.ipify.org"
EOF
chmod 600 /etc/rmm-agent.conf
```

Enable service:

```sh
/etc/init.d/rmm-agent enable
/etc/init.d/rmm-agent start
```

## OpenWrt Package Build

The repository includes an MVP package skeleton at:

```text
agent/package/rmm-agent
```

Copy or symlink it into an OpenWrt buildroot:

```sh
cp -R agent/package/rmm-agent /path/to/openwrt/package/rmm-agent
cd /path/to/openwrt
make menuconfig
make package/rmm-agent/compile V=s
```

The package installs:

- `/usr/bin/rmm-agent`
- `/etc/init.d/rmm-agent`
- `/etc/rmm-agent.conf`

After installing the `.ipk` on a router:

```sh
vi /etc/rmm-agent.conf
/etc/init.d/rmm-agent enable
/etc/init.d/rmm-agent start
```

The config file is marked as a package conffile, so `opkg` should preserve local credentials and server settings on upgrade.

## Agent Dependencies

Required:

- `/bin/sh`
- `sed`
- `awk`
- `ip`
- `curl` or `wget`

Recommended:

- `ubus`
- `opkg`
- `traceroute`
- `openssh-client` for Docker Compose reverse SSH access

## Current Shell Agent Notes

The MVP shell agent avoids requiring `jq`. It uses the plain-text `POST /api/agent/commands/next` endpoint for command polling and JSON endpoints for enrollment, heartbeat, and command results.

The agent supports a first safe UCI workflow through allowlisted commands:

- `uci_show`
- `uci_backup`
- `uci_preview`
- `uci_set`
- `uci_commit`
- `uci_commit_confirmed`
- `uci_revert`
- `uci_restore`

The MVP allowlist is limited to `network`, `wireless`, `dhcp`, `firewall`, and `system`.

`uci_preview` stages the requested change, captures before/after output, then reverts the config package. `uci_set` captures `uci export <config>` before staging a change, so command output includes a backup snapshot.

The shell agent stores the latest local backup per config in `/tmp/rmm-agent-backups`. `uci_restore` imports and commits that backup. `uci_commit_confirmed` commits, waits for server reachability, and restores the latest backup if the server cannot be reached.

The MVP agent also uses:

- a lock directory at `/tmp/rmm-agent.lock`;
- a result spool at `/tmp/rmm-agent-results`;
- heartbeat backoff up to 300 seconds on repeated failures.

`CHECK_TARGETS` controls heartbeat connectivity probes. The agent pings each target and sends `reachable`, `packet_loss_percent`, and `latency_ms` in `metrics.connectivity_checks`.

For a router and server in the same LAN, verify the server from the router with the server LAN IP:

```sh
ping -c 3 10.10.10.2
curl -i http://10.10.10.2:18080/healthz
```

Do not use `127.0.0.1` from the router for the RMM server unless the server is running on the router itself.

The production Go agent can update the direct DNS backend. Enable it in LuCI under
**Services -> RMM agent -> Direct DNS**, or set `DIRECT_DNS_ENABLED="true"` manually.
The feature uses HTTPS-only address discovery and is disabled by default because enabling
it sends the router's public IP address to the configured discovery services and RMM.

The shell agent redacts sensitive command output keys before sending command results to the server. This is a defense-in-depth measure; the server also redacts before storing results.

The production agent should either:

- be rewritten in Go;
- include a tiny JSON parser strategy;
- keep a stable shell-oriented command polling endpoint.

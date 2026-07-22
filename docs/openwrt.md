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

The shell agent redacts sensitive command output keys before sending command results to the server. This is a defense-in-depth measure; the server also redacts before storing results.

The production agent should either:

- be rewritten in Go;
- include a tiny JSON parser strategy;
- keep a stable shell-oriented command polling endpoint.

## GitHub Actions package matrix

`.github/workflows/build.yml` runs Go, web and license checks on every push and pull
request. A manual workflow run or a tag matching `agent-v*` additionally builds installable
OpenWrt packages through the official SDKs.

The release matrix covers OpenWrt `21.02.7`, `22.03.7`, `23.05.5`, `24.10.7` and
`25.12.4`, and these common CPU/target families where the target exists:

- x86-64 (`x86/64`, `amd64`);
- MediaTek MT7621 (`ramips/mt7621`, `mipsle`);
- Atheros (`ath79/generic`, `mips`);
- Qualcomm IPQ40xx (`ipq40xx/generic`, ARMv7);
- Raspberry Pi 4 (`bcm27xx/bcm2711`, ARM64);
- MediaTek Filogic (`mediatek/filogic`, ARM64) on supported releases.

The workflow resolves each SDK filename and SHA256 from the official OpenWrt mirror,
falling back to the OpenWrt archive for end-of-life branches. Releases through `24.10`
produce `.ipk`; `25.12` produces `.apk`. Manual runs retain each target as a workflow
artifact for 30 days. An `agent-v*` tag also creates a GitHub Release containing all packages and
a combined `SHA256SUMS`, a Sigstore signature bundle and native signed package feeds.
See [package-repository.md](package-repository.md) for key provisioning and the commands
used to connect `opkg` or `apk` to the repository.

This matrix represents common CPU families, not every OpenWrt target. Add an explicit
matrix row for another target/subtarget and select the matching Go architecture rather
than assuming binaries from a different CPU family are compatible.

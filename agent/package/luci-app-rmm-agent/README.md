# luci-app-rmm-agent

Current JavaScript/UCI LuCI application for configuring the OpenWrt RMM agent.

It provides:

- enable/disable and service restart;
- HTTPS server URL and polling interval;
- one-time enrollment grant input;
- connectivity targets and tunnel identity path;
- explicit lab-only HTTP override;
- controlled device identity reset for re-enrollment.

The agent package owns `/etc/config/rmm-agent` and converts UCI values to the existing
`/etc/rmm-agent.conf` format with mode `0600`. Existing enrolled installations are migrated
without replacing their device ID/token. Once enrollment succeeds, the one-time grant is
removed from UCI and is not persisted for reuse.

Place `agent/package` in an OpenWrt package feed and build/install the LuCI app with either
the shell runtime (`rmm-agent`) or the production Go runtime (`rmm-agent-go-production`):

```sh
make package/rmm-agent/compile V=s
make package/luci-app-rmm-agent/compile V=s
opkg install rmm-agent_*.ipk luci-app-rmm-agent_*.ipk
```

On apk-based OpenWrt releases, install the generated `.apk` packages instead.

## Reproducible Docker build

The repository includes a containerized OpenWrt SDK build. By default it uses the
official OpenWrt 25.12.4 `ramips/mt7621` SDK for ASUS RT-AX53U and compiles the Go
agent as little-endian MIPS with software floating point. It builds the LuCI
application, the shell agent and the production Go agent together:

```sh
docker build \
  --file deploy/luci-builder/Dockerfile \
  --target artifacts \
  --output type=local,dest=dist/rmm-openwrt-25.12.4-ramips-mt7621 \
  .
```

The output directory contains the installable packages and `SHA256SUMS`.
`luci-app-rmm-agent` and the shell `rmm-agent` are architecture-independent. The Go
runtime is architecture-specific and is built as `linux/mipsle` with
`GOMIPS=softfloat` by default. Use an SDK from the same OpenWrt release and target as
the router, and set the matching Go architecture when targeting another device. For
example, an x86_64 router needs this additional build argument:

```sh
--build-arg RMM_GOARCH=amd64 --build-arg RMM_GOAMD64=v1
```

Override `OPENWRT_SDK_URL` and `OPENWRT_SDK_SHA256` with `--build-arg` when building for
another release or target. Install exactly one runtime package: `rmm-agent` or
`rmm-agent-go-production`. They intentionally conflict because both own the same service
and executable paths.

## Install on OpenWrt 25.12.4

For the ASUS RT-AX53U (`ramips/mt7621`), copy the production Go runtime and the LuCI
application to the router. Do not copy or install the shell runtime at the same time:

```sh
cd dist/rmm-openwrt-25.12.4-ramips-mt7621
sha256sum -c SHA256SUMS
scp \
  rmm-agent-go-production-0.5.2-r1.apk \
  luci-app-rmm-agent-0.1.0-r1.apk \
  root@ROUTER_IP:/tmp/
```

Then install the locally built, unsigned packages over SSH:

```sh
apk add --allow-untrusted \
  /tmp/rmm-agent-go-production-0.5.2-r1.apk \
  /tmp/luci-app-rmm-agent-0.1.0-r1.apk
/etc/init.d/rpcd restart
/etc/init.d/uhttpd restart
```

Open LuCI, go to **Services → RMM agent**, enter the HTTPS server URL and a fresh
one-time enrollment grant, enable the agent, save the settings, and start or restart the
service. Verify it from SSH with:

```sh
/etc/init.d/rmm-agent status
logread -e rmm-agent
```

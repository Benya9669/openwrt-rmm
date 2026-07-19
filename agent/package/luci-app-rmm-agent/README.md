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

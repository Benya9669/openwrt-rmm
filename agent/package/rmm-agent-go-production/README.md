# rmm-agent-go-production OpenWrt Package

OpenWrt package skeleton for installing the Go agent as the primary `rmm-agent` service.

## Buildroot Usage

Cross-build the Go binary for the router target first and place it into the package-local files directory:

```sh
mkdir -p agent/package/rmm-agent-go-production/files/usr/bin
GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -trimpath -ldflags="-s -w" -o agent/package/rmm-agent-go-production/files/usr/bin/rmm-agent ./agent/go/cmd/rmm-agent
```

Copy or symlink this directory into an OpenWrt buildroot package path:

```sh
cp -R agent/package/rmm-agent-go-production /path/to/openwrt/package/rmm-agent-go-production
cd /path/to/openwrt
make package/rmm-agent-go-production/compile V=s
```

Then install the generated `.ipk` on a router that already has `/etc/rmm-agent.conf` with the correct `DEVICE_ID` and `DEVICE_TOKEN`:

```sh
opkg install /tmp/rmm-agent-go-production_*.ipk
/etc/init.d/rmm-agent restart
```

## Installed Files

- `/usr/bin/rmm-agent`
- `/etc/init.d/rmm-agent`
- `/etc/rmm-agent.conf`

This package is intended for the final shell-to-Go migration when the router should keep the same RMM object identity.

Version `0.6.2` uses the cloud tunnel exclusively and no longer discovers or publishes the
router's public WAN addresses.

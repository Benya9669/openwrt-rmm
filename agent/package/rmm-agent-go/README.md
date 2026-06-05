# rmm-agent-go OpenWrt Package

OpenWrt package skeleton for the Go agent.

## Buildroot Usage

Cross-build the Go binary for the router target first and place it into the package-local files directory:

```sh
mkdir -p agent/package/rmm-agent-go/files/usr/bin
GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -trimpath -ldflags="-s -w" -o agent/package/rmm-agent-go/files/usr/bin/rmm-agent-go ./agent/go/cmd/rmm-agent
```

Copy or symlink this directory into an OpenWrt buildroot package path:

```sh
cp -R agent/package/rmm-agent-go /path/to/openwrt/package/rmm-agent-go
cd /path/to/openwrt
make package/rmm-agent-go/compile V=s
```

Then install the generated `.ipk` on a router:

```sh
opkg install /tmp/rmm-agent-go_*.ipk
vi /etc/rmm-agent-go.conf
/etc/init.d/rmm-agent-go enable
/etc/init.d/rmm-agent-go start
```

## Installed Files

- `/usr/bin/rmm-agent-go`
- `/etc/init.d/rmm-agent-go`
- `/etc/rmm-agent-go.conf`

The package intentionally keeps a separate config and service name so it can run side by side with the shell agent during migration.

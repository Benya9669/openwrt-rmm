# rmm-agent OpenWrt Package

OpenWrt package skeleton for the MVP shell agent.

## Buildroot Usage

Copy or symlink this directory into an OpenWrt buildroot package path:

```sh
cp -R agent/package/rmm-agent /path/to/openwrt/package/rmm-agent
cd /path/to/openwrt
./scripts/feeds update -a
./scripts/feeds install -a
make menuconfig
make package/rmm-agent/compile V=s
```

Then install the generated `.ipk` on a router:

```sh
opkg install /tmp/rmm-agent_*.ipk
vi /etc/rmm-agent.conf
/etc/init.d/rmm-agent enable
/etc/init.d/rmm-agent start
```

## Installed Files

- `/usr/bin/rmm-agent`
- `/etc/init.d/rmm-agent`
- `/etc/rmm-agent.conf`

## Maintainer Note

The package currently vendors a copy of `agent/openwrt/rmm-agent.sh` into `files/usr/bin/rmm-agent` because OpenWrt package builds expect package-local files. Keep both files in sync until a small sync script is added.

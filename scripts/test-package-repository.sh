#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT

mkdir -p \
  "$fixture/input/openwrt-24.10.7-ramips-mt7621" \
  "$fixture/input/openwrt-25.12.4-ramips-mt7621" \
  "$fixture/control"
printf 'Package: rmm-agent-go-production\nVersion: 0.6.8-1\n' > "$fixture/control/control"
tar -C "$fixture/control" -czf "$fixture/control.tar.gz" control
ar r "$fixture/input/openwrt-24.10.7-ramips-mt7621/rmm-agent-go-production_0.6.8-1_mipsel_24kc.ipk" \
  "$fixture/control.tar.gz"
touch "$fixture/input/openwrt-25.12.4-ramips-mt7621/rmm-agent-go-production-0.6.8-r1.apk"
touch "$fixture/input/openwrt-25.12.4-ramips-mt7621/packages.adb"

bash "$repository_root/scripts/prepare-package-repository.sh" \
  "$fixture/input" \
  "$fixture/output" \
  "0.6.8"

node -e '
  const fs = require("node:fs");
  const manifest = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
  const ipk = manifest.packages.find((entry) => entry.format === "ipk");
  const apk = manifest.packages.find((entry) => entry.format === "apk");
  if (manifest.agent.version !== "0.6.8" || manifest.packages.length !== 2) process.exit(1);
  if (!ipk || ipk.feed_url !== "https://packages.daemonlord.ru/feeds/0.6.8/openwrt/24.10.7/ramips-mt7621" || ipk.package_version !== "0.6.8-1") process.exit(1);
  if (!apk || apk.feed_url !== "https://packages.daemonlord.ru/feeds/0.6.8/openwrt/25.12.4/ramips-mt7621/packages.adb" || apk.package_version !== "0.6.8-r1") process.exit(1);
' "$fixture/output/update-manifest.json"

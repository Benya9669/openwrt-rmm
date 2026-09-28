#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: $0 <lock-file> <openwrt-release> <output-dir>" >&2
  exit 2
fi

lock_file="$1"
openwrt_release="$2"
output_dir="$3"

test -f "$lock_file"
mapfile -t records < <(sed '/^[[:space:]]*#/d; /^[[:space:]]*$/d' "$lock_file")
if [ "${#records[@]}" -ne 1 ]; then
  echo "LuCI release lock must contain exactly one record" >&2
  exit 1
fi
record="${records[0]}"
if [ "$record" = none ]; then
  mkdir -p "$output_dir"
  exit 0
fi

read -r repository tag manifest_sha extra <<<"$record"
if [ -n "${extra:-}" ] || [ "$repository" != 'Benya9669/luci-theme-rmm' ] ||
   [[ ! "$tag" =~ ^luci-v[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
   [[ ! "$manifest_sha" =~ ^[0-9a-f]{64}$ ]] ||
   [[ ! "$openwrt_release" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "invalid LuCI release lock or OpenWrt version" >&2
  exit 1
fi

command -v gh >/dev/null 2>&1 || { echo "gh is required" >&2; exit 1; }
command -v sha256sum >/dev/null 2>&1 || { echo "sha256sum is required" >&2; exit 1; }

download_dir="$(mktemp -d)"
trap 'rm -rf "$download_dir"' EXIT
gh release download "$tag" --repo "$repository" --dir "$download_dir"
printf '%s  %s\n' "$manifest_sha" "$download_dir/SHA256SUMS" | sha256sum --check --strict
if [ "$(wc -l < "$download_dir/SHA256SUMS")" -ne 4 ] ||
   ! awk '$2 !~ /^\.\/openwrt-(24\.10\.7|25\.12\.4)-luci-(theme-rmm|app-rmm-dashboard).*\.(ipk|apk)$/ { bad = 1 } END { exit bad }' "$download_dir/SHA256SUMS"; then
  echo "LuCI release manifest contains unexpected assets" >&2
  exit 1
fi
(cd "$download_dir" && sha256sum --check --strict SHA256SUMS)

package_format=ipk
if [[ "$openwrt_release" == 25.* ]]; then
  package_format=apk
fi

mkdir -p "$output_dir"
for package_name in luci-theme-rmm luci-app-rmm-dashboard; do
  matches=("$download_dir/openwrt-$openwrt_release-$package_name"*."$package_format")
  if [ "${#matches[@]}" -ne 1 ] || [ ! -f "${matches[0]}" ]; then
    echo "expected one $package_name $package_format for OpenWrt $openwrt_release" >&2
    exit 1
  fi
  filename="${matches[0]##*/}"
  if [ "$(awk -v name="./$filename" '$2 == name { count++ } END { print count + 0 }' "$download_dir/SHA256SUMS")" -ne 1 ]; then
    echo "release manifest does not uniquely cover $filename" >&2
    exit 1
  fi
  cp "${matches[0]}" "$output_dir/${filename#openwrt-$openwrt_release-}"
done

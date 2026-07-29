#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: $0 <release-assets-dir> <sdk-lock-file> <output-dir>" >&2
  exit 2
fi

source_dir="$1"
lock_file="$2"
output_dir="$3"

if [ ! -d "$source_dir" ]; then
  echo "release assets directory does not exist: $source_dir" >&2
  exit 1
fi
if [ ! -f "$lock_file" ]; then
  echo "SDK lock file does not exist: $lock_file" >&2
  exit 1
fi
if [ -e "$output_dir" ] && [ -n "$(find "$output_dir" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]; then
  echo "output directory must be empty: $output_dir" >&2
  exit 1
fi
mkdir -p "$output_dir"

artifact_count=0
while IFS=$'\t' read -r release target subtarget _sha256 _url; do
  [ -n "$release" ] || continue
  case "$release" in
    \#*) continue ;;
  esac

  case "$target/$subtarget" in
    x86/64) label="x86-64" ;;
    *) label="${target}-${subtarget}" ;;
  esac

  artifact_name="openwrt-${release}-${label}"
  destination="$output_dir/$artifact_name"
  matched=0
  for asset in "$source_dir/$artifact_name"-*; do
    [ -f "$asset" ] || continue
    mkdir -p "$destination"
    basename="${asset##*/}"
    cp "$asset" "$destination/${basename#"$artifact_name"-}"
    matched=1
  done
  if [ "$matched" -eq 1 ]; then
    artifact_count=$((artifact_count + 1))
  fi
done < "$lock_file"

if [ "$artifact_count" -eq 0 ]; then
  echo "no prefixed OpenWrt package assets found in $source_dir" >&2
  exit 1
fi

echo "reconstructed ${artifact_count} OpenWrt package artifact directories"

#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: $0 <downloaded-artifacts-dir> <output-dir> <agent-version>" >&2
  exit 2
fi

source_dir="$1"
output_dir="$2"
agent_version="$3"

if [ ! -d "$source_dir" ]; then
  echo "artifact directory does not exist: $source_dir" >&2
  exit 1
fi

case "$agent_version" in
  ''|*[!0-9A-Za-z._-]*)
    echo "invalid agent version: $agent_version" >&2
    exit 1
    ;;
esac

if [ -e "$output_dir" ] && [ -n "$(find "$output_dir" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]; then
  echo "output directory must be empty: $output_dir" >&2
  exit 1
fi
mkdir -p "$output_dir/feeds/$agent_version/openwrt" "$output_dir/keys/usign" "$output_dir/keys/apk"
manifest_entries="$output_dir/.update-manifest-entries"
: > "$manifest_entries"

artifact_count=0
for artifact_dir in "$source_dir"/openwrt-*; do
  [ -d "$artifact_dir" ] || continue
  artifact_name="$(basename "$artifact_dir")"
  remainder="${artifact_name#openwrt-}"
  openwrt_release="${remainder%%-*}"
  target_label="${remainder#"$openwrt_release"-}"

  if [ -z "$openwrt_release" ] || [ -z "$target_label" ] || [ "$target_label" = "$remainder" ]; then
    echo "cannot parse artifact name: $artifact_name" >&2
    exit 1
  fi
  case "$openwrt_release:$target_label" in
    *[!0-9A-Za-z._:-]*)
      echo "artifact name contains unsupported manifest characters: $artifact_name" >&2
      exit 1
      ;;
  esac

  destination="$output_dir/feeds/$agent_version/openwrt/$openwrt_release/$target_label"
  mkdir -p "$destination"
  cp "$artifact_dir"/* "$destination/"
  package_format="ipk"
  if find "$artifact_dir" -maxdepth 1 -type f -name '*.apk' -print -quit | grep -q .; then
    package_format="apk"
  fi
  if [ -s "$manifest_entries" ]; then
    printf ',\n' >> "$manifest_entries"
  fi
  printf '    {"openwrt_release":"%s","target":"%s","format":"%s","feed_url":"%s"}' \
    "$openwrt_release" \
    "$target_label" \
    "$package_format" \
    "https://benya9669.github.io/openwrt-rmm/feeds/stable/openwrt/$openwrt_release/$target_label" \
    >> "$manifest_entries"

  while IFS= read -r public_key; do
    cp "$public_key" "$output_dir/keys/usign/$(basename "$public_key")"
  done < <(find "$artifact_dir" -maxdepth 1 -type f -regextype posix-extended \
    -regex '.*/[0-9a-f]{16}' -print)

  if [ -f "$artifact_dir/rmm-openwrt.pem" ]; then
    cp "$artifact_dir/rmm-openwrt.pem" "$output_dir/keys/apk/rmm-openwrt.pem"
  fi
  artifact_count=$((artifact_count + 1))
done

if [ "$artifact_count" -eq 0 ]; then
  echo "no openwrt-* artifact directories found in $source_dir" >&2
  exit 1
fi

mkdir -p "$output_dir/feeds/stable"
cp -a "$output_dir/feeds/$agent_version/." "$output_dir/feeds/stable/"
touch "$output_dir/.nojekyll"

cat > "$output_dir/update-manifest.json" <<EOF
{
  "schema": 1,
  "channel": "stable",
  "agent": {
    "version": "${agent_version}",
    "release_url": "https://github.com/Benya9669/openwrt-rmm/releases/tag/agent-v${agent_version}",
    "feed_base_url": "https://benya9669.github.io/openwrt-rmm/feeds/stable/openwrt"
  },
  "packages": [
EOF
cat "$manifest_entries" >> "$output_dir/update-manifest.json"
printf '\n' >> "$output_dir/update-manifest.json"
cat >> "$output_dir/update-manifest.json" <<EOF
  ]
}
EOF
rm -f "$manifest_entries"

cat > "$output_dir/index.html" <<EOF
<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>OpenWrt RMM package repository</title>
</head>
<body>
  <main>
    <h1>OpenWrt RMM package repository</h1>
    <p>Текущая стабильная версия агента: <strong>${agent_version}</strong>.</p>
    <p>Инструкции подключения находятся в документации проекта.</p>
  </main>
</body>
</html>
EOF

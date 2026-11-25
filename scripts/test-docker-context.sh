#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture=$(mktemp -d)
trap 'rm -rf -- "$fixture"' EXIT
context="$fixture/context"
mkdir "$context"
printf 'FROM scratch\nCOPY . /\n' > "$context/Dockerfile"
key='keys/openwrt/apk/rmm-openwrt.pem'
usign='keys/openwrt/usign/7fb0908fb6bc82c8'
app='agent/package/luci-app-rmm-agent'
prod='agent/package/rmm-agent-go-production'
feed='site/feeds/stable/openwrt/25.12.4/ramips-mt7621'
common=(go.mod go.sum internal/fleetschema/fleet.sql "$key")
server=(server/cmd/rmm-server/main.go web/assets/icons/tabler-sprite.svg deploy/server/entrypoint.sh)
agent=(agent/go/cmd/rmm-agent/main.go "$app/root/usr/libexec/rpcd/rmm.agent"
       "$prod/files/etc/rmm-agent/update-manifest.pem" deploy/luci-external/example.apk)
indexes=("$feed/test.apk" "$feed/packages.adb" "$feed/SHA256SUMS")
blocked=(.env rmm.db-shm rmm.db-wal release-keys/signing.key .tmp/backup.tar .tmp-docker-config/config.json
         .gopath/pkg/mod/sample/main.go test-results/trace.zip secrets/database.key keys/openwrt/apk/private.pem
         "$prod/files/etc/rmm-agent/private.pem" "$feed/private.pem")
for path in "${common[@]}" "${server[@]}" "${agent[@]}" "${indexes[@]}" "$usign" "${blocked[@]}"; do
  mkdir -p "$context/$(dirname "$path")"
  printf 'fixture\n' > "$context/$path"
done

check() {
  local title="$1" rules="$2" out="$fixture/$1" path
  shift 2
  cp "$root/$rules" "$context/.dockerignore"
  docker buildx build --output "type=local,dest=$out" "$context"
  printf '%s\n' "$@" | LC_ALL=C sort > "$fixture/expected"
  (cd "$out" && find . -type f | sed 's|^./||' | LC_ALL=C sort) > "$fixture/actual"
  diff -u "$fixture/expected" "$fixture/actual"
  printf 'Verified %s context\n' "$title"
}
check server .dockerignore "${common[@]}" "${server[@]}"
check builder deploy/luci-builder/Dockerfile.dockerignore "${common[@]}" "${agent[@]}" "$usign"
check indexer deploy/luci-feed-indexer/Dockerfile.dockerignore "$key" "$usign" "${indexes[@]}"

#!/usr/bin/env bash
set -euo pipefail

fixture="$(mktemp -d)"
mkdir -p "$fixture/bin" "$fixture/release" "$fixture/output"

for release in 24.10.7 25.12.4; do
  format=ipk
  if [[ "$release" == 25.* ]]; then format=apk; fi
  for name in luci-theme-rmm luci-app-rmm-dashboard; do
    printf '%s\n' "$release/$name" > "$fixture/release/openwrt-$release-$name-0.1.0-r1.$format"
  done
done
(cd "$fixture/release" && sha256sum --text ./openwrt-* > SHA256SUMS)
manifest_sha="$(sha256sum "$fixture/release/SHA256SUMS" | cut -d ' ' -f 1)"
printf 'Benya9669/luci-theme-rmm luci-v0.1.0 %s\n' "$manifest_sha" > "$fixture/lock"

cat > "$fixture/bin/gh" <<'EOF'
#!/bin/sh
set -eu
test "$1" = release
test "$2" = download
shift 3
while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo) shift 2 ;;
    --dir) destination="$2"; shift 2 ;;
    *) exit 1 ;;
  esac
done
cp "$FIXTURE_RELEASE"/* "$destination"/
EOF
chmod +x "$fixture/bin/gh"
export FIXTURE_RELEASE="$fixture/release"
export PATH="$fixture/bin:$PATH"

bash scripts/fetch-luci-release.sh "$fixture/lock" 24.10.7 "$fixture/output"
test -f "$fixture/output/luci-theme-rmm-0.1.0-r1.ipk"
test -f "$fixture/output/luci-app-rmm-dashboard-0.1.0-r1.ipk"
test "$(find "$fixture/output" -type f | wc -l)" -eq 2

printf 'Benya9669/luci-theme-rmm luci-v0.1.0 %064d\n' 0 > "$fixture/bad-lock"
if bash scripts/fetch-luci-release.sh "$fixture/bad-lock" 25.12.4 "$fixture/bad-output" >/dev/null 2>&1; then
  echo 'import accepted an invalid manifest digest' >&2
  exit 1
fi
test ! -e "$fixture/bad-output"

# Six assets include the translation for both SDKs; old four-asset releases
# above remain supported.
for release in 24.10.7 25.12.4; do
  format=ipk
  if [[ "$release" == 25.* ]]; then format=apk; fi
  printf 'translation\n' > "$fixture/release/openwrt-$release-luci-i18n-rmm-dashboard-zh-cn-0.5.0-r1.$format"
done
(cd "$fixture/release" && sha256sum --text ./openwrt-* > SHA256SUMS)
manifest_sha="$(sha256sum "$fixture/release/SHA256SUMS" | cut -d ' ' -f 1)"
printf 'Benya9669/luci-theme-rmm luci-v0.5.0 %s\n' "$manifest_sha" > "$fixture/translation-lock"
for release in 24.10.7 25.12.4; do
  bash scripts/fetch-luci-release.sh "$fixture/translation-lock" "$release" "$fixture/output-$release"
  test "$(find "$fixture/output-$release" -type f | wc -l)" -eq 3
done

for release in 24.10.7 25.12.4; do
  format=ipk
  if [[ "$release" == 25.* ]]; then format=apk; fi
  printf 'russian translation\n' > "$fixture/release/openwrt-$release-luci-i18n-rmm-dashboard-ru-0.5.0-r1.$format"
done
(cd "$fixture/release" && sha256sum --text ./openwrt-* > SHA256SUMS)
manifest_sha="$(sha256sum "$fixture/release/SHA256SUMS" | cut -d ' ' -f 1)"
printf 'Benya9669/luci-theme-rmm luci-v0.5.0 %s\n' "$manifest_sha" > "$fixture/russian-lock"
for release in 24.10.7 25.12.4; do
  bash scripts/fetch-luci-release.sh "$fixture/russian-lock" "$release" "$fixture/russian-$release"
  test "$(find "$fixture/russian-$release" -type f | wc -l)" -eq 4
done

printf 'none\n' > "$fixture/none-lock"
bash scripts/fetch-luci-release.sh "$fixture/none-lock" 25.12.4 "$fixture/none-output"
test -d "$fixture/none-output"

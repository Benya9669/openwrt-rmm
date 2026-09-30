#!/usr/bin/env bash
set -euo pipefail

fixture="$(mktemp -d)"
# Fixtures are left in the temporary directory, following existing release tests.
for format in ipk apk; do
  source_dir="$fixture/source-$format"
  feed="$fixture/site/feeds/stable/openwrt/25.12.4/test-$format"
  archive="$fixture/site/feeds/0.9.0/openwrt/25.12.4/test-$format"
  mkdir -p "$source_dir" "$feed" "$archive"
  for name in luci-theme-rmm luci-app-rmm-dashboard; do
    printf 'new %s\n' "$name" > "$source_dir/${name}_0.3.0-r1_all.$format"
    printf 'old\n' > "$feed/${name}_0.1.0-r1_all.$format"
    printf 'old\n' > "$feed/${name}-0.2.0-r1.$format"
    printf 'archive\n' > "$archive/${name}_0.1.0-r1_all.$format"
  done
  for name in rmm-agent-go-production_0.9.0 luci-app-rmm-agent_0.2.3 luci-i18n-rmm-agent-ru_0.2.3 luci-theme-rmm-extra_1.0; do
    printf 'preserve\n' > "$feed/$name.$format"
  done
  (cd "$archive" && sha256sum ./*) > "$fixture/archive-$format.sha"
  (cd "$feed" && sha256sum rmm-agent* luci-app-rmm-agent* luci-i18n-* luci-theme-rmm-extra*) > "$fixture/unrelated-$format.sha"
  bash scripts/stage-luci-stable-packages.sh "$source_dir" "$feed"
  bash scripts/stage-luci-stable-packages.sh "$source_dir" "$feed"
  for name in luci-theme-rmm luci-app-rmm-dashboard; do
    cmp "$source_dir/${name}_0.3.0-r1_all.$format" "$feed/${name}_0.3.0-r1_all.$format"
    test ! -e "$feed/${name}_0.1.0-r1_all.$format"
    test ! -e "$feed/${name}-0.2.0-r1.$format"
    test -f "$archive/${name}_0.1.0-r1_all.$format"
  done
  test "$(find "$feed" -type f | wc -l)" -eq 6
  (cd "$archive" && sha256sum --check "$fixture/archive-$format.sha")
  (cd "$feed" && sha256sum --check "$fixture/unrelated-$format.sha")
  if bash scripts/stage-luci-stable-packages.sh "$source_dir" "$archive" >/dev/null 2>&1; then
    echo 'accepted an archive destination' >&2; exit 1
  fi
  # Incomplete input must not remove the current feed packages.
  mv "$source_dir/luci-theme-rmm_0.3.0-r1_all.$format" "$fixture/removed-$format"
  if bash scripts/stage-luci-stable-packages.sh "$source_dir" "$feed" >/dev/null 2>&1; then
    echo 'accepted incomplete input' >&2; exit 1
  fi
  test "$(find "$feed" -type f | wc -l)" -eq 6
  # A path beginning inside stable but resolving to an archive is also rejected.
  mv "$fixture/removed-$format" "$source_dir/luci-theme-rmm_0.3.0-r1_all.$format"
  if bash scripts/stage-luci-stable-packages.sh "$source_dir" "$feed/../../../../0.9.0/openwrt/25.12.4/test-$format" >/dev/null 2>&1; then
    echo 'accepted a path resolving to an archive destination' >&2; exit 1
  fi
  (cd "$archive" && sha256sum --check "$fixture/archive-$format.sha")

  # New releases replace only the dashboard's own translation.
  translation=luci-i18n-rmm-dashboard-zh-cn
  printf 'translated\n' > "$source_dir/${translation}_0.5.0-r1_all.$format"
  printf 'old translation\n' > "$feed/${translation}_0.4.0-r1_all.$format"
  bash scripts/stage-luci-stable-packages.sh "$source_dir" "$feed"
  bash scripts/stage-luci-stable-packages.sh "$source_dir" "$feed"
  test ! -e "$feed/${translation}_0.4.0-r1_all.$format"
  cmp "$source_dir/${translation}_0.5.0-r1_all.$format" "$feed/${translation}_0.5.0-r1_all.$format"
  test "$(find "$feed" -type f | wc -l)" -eq 7
  (cd "$feed" && sha256sum --check "$fixture/unrelated-$format.sha")
  # A legacy two-package import preserves the installed translation.
  mv "$source_dir/${translation}_0.5.0-r1_all.$format" "$fixture/translation-$format"
  bash scripts/stage-luci-stable-packages.sh "$source_dir" "$feed"
  cmp "$fixture/translation-$format" "$feed/${translation}_0.5.0-r1_all.$format"

  mv "$fixture/translation-$format" "$source_dir/${translation}_0.5.0-r1_all.$format"
  russian=luci-i18n-rmm-dashboard-ru
  printf 'russian\n' > "$source_dir/${russian}_0.5.0-r1_all.$format"
  printf 'old russian\n' > "$feed/${russian}_0.4.0-r1_all.$format"
  bash scripts/stage-luci-stable-packages.sh "$source_dir" "$feed"
  bash scripts/stage-luci-stable-packages.sh "$source_dir" "$feed"
  test ! -e "$feed/${russian}_0.4.0-r1_all.$format"
  cmp "$source_dir/${russian}_0.5.0-r1_all.$format" "$feed/${russian}_0.5.0-r1_all.$format"
  test "$(find "$feed" -type f | wc -l)" -eq 8
  (cd "$feed" && sha256sum --check "$fixture/unrelated-$format.sha")
done
echo 'Stable LuCI replacement checks passed'

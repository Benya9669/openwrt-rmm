#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: $0 <verified-packages-dir> <stable-target-dir>" >&2
  exit 2
fi

command -v realpath >/dev/null 2>&1 || { echo 'realpath is required' >&2; exit 1; }
source_dir="$(realpath -e -- "$1")"
feed_dir="$(realpath -e -- "$2")"
test -d "$source_dir"
test -d "$feed_dir"
# Cleanup is permitted only in a resolved stable target directory, never archives.
if [[ ! "$feed_dir" =~ /feeds/stable/openwrt/[0-9]+\.[0-9]+\.[0-9]+/[A-Za-z0-9._-]+$ ]] ||
   [ "$source_dir" = "$feed_dir" ]; then
  echo 'destination must be a stable OpenWrt target directory' >&2
  exit 1
fi

shopt -s nullglob dotglob
packages=("$source_dir"/*)
if [ "${#packages[@]}" -lt 2 ] || [ "${#packages[@]}" -gt 4 ]; then
  echo 'expected theme, dashboard and optional ru/zh-cn translations' >&2
  exit 1
fi

theme=0
dashboard=0
translation=0
translation_ru=0
format=''
for package in "${packages[@]}"; do
  filename="${package##*/}"
  if [ ! -f "$package" ] || [ -L "$package" ] ||
     [[ ! "$filename" =~ ^(luci-theme-rmm|luci-app-rmm-dashboard|luci-i18n-rmm-dashboard-zh-cn|luci-i18n-rmm-dashboard-ru)[_-][0-9][A-Za-z0-9._+~-]*\.(ipk|apk)$ ]]; then
    echo "unexpected LuCI package: $filename" >&2
    exit 1
  fi
  package_name="${BASH_REMATCH[1]}"
  package_format="${BASH_REMATCH[2]}"
  if [ -n "$format" ] && [ "$format" != "$package_format" ]; then
    echo 'LuCI packages must use the same format' >&2
    exit 1
  fi
  format="$package_format"
  case "$package_name" in
    luci-theme-rmm) theme=$((theme + 1)) ;;
    luci-app-rmm-dashboard) dashboard=$((dashboard + 1)) ;;
    luci-i18n-rmm-dashboard-zh-cn) translation=$((translation + 1)) ;;
    luci-i18n-rmm-dashboard-ru) translation_ru=$((translation_ru + 1)) ;;
  esac
  # Do not overwrite through a destination symlink.
  if [ -L "$feed_dir/$filename" ] || { [ -e "$feed_dir/$filename" ] && [ ! -f "$feed_dir/$filename" ]; }; then
    echo "unsafe destination entry: $filename" >&2
    exit 1
  fi
done
test "$theme" -eq 1
test "$dashboard" -eq 1
test "$translation" -le 1
test "$translation_ru" -le 1
test "$((translation + translation_ru))" -eq "$((${#packages[@]} - 2))"

# The caller verified the release manifest and old snapshot before this step.
# Copy all new packages successfully before removing superseded versions.
cp -- "${packages[@]}" "$feed_dir/"
for package in "$feed_dir"/*; do
  filename="${package##*/}"
  if [[ "$filename" =~ ^(luci-theme-rmm|luci-app-rmm-dashboard|luci-i18n-rmm-dashboard-zh-cn|luci-i18n-rmm-dashboard-ru)[_-][0-9][A-Za-z0-9._+~-]*\.(ipk|apk)$ ]] &&
     [ -f "$package" ] && [ ! -L "$package" ] &&
     [ ! -f "$source_dir/$filename" ] &&
     { [ "${BASH_REMATCH[1]}" != luci-i18n-rmm-dashboard-zh-cn ] || [ "$translation" -eq 1 ]; } &&
     { [ "${BASH_REMATCH[1]}" != luci-i18n-rmm-dashboard-ru ] || [ "$translation_ru" -eq 1 ]; }; then
    echo "Removing superseded stable LuCI package: $filename"
    rm -- "$package"
  fi
done

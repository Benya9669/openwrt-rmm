#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  echo "usage: $0 <destination-dir>" >&2
  exit 2
fi

destination="$1"
if [ -e "$destination" ]; then
  echo "destination already exists: $destination" >&2
  exit 1
fi

run_id="$(
  gh api "repos/${GITHUB_REPOSITORY}/actions/artifacts?name=package-repository-site&per_page=100" \
    --jq '[.artifacts[] | select(.expired == false)] | sort_by(.created_at) | last | .workflow_run.id // empty'
)"
if [ -z "$run_id" ]; then
  echo "no retained package-repository-site artifact; refusing to replace the deployed feed" >&2
  exit 1
fi

gh run download "$run_id" \
  --repo "$GITHUB_REPOSITORY" \
  --name package-repository-site \
  --dir "$destination"

test -s "$destination/update-manifest.json"
test -s "$destination/update-manifest.sig"
test -d "$destination/feeds/stable/openwrt"

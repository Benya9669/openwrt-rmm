#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 [--dry-run] <backup-directory>" >&2
  exit 2
}

dry_run=false
if [ "${1:-}" = "--dry-run" ]; then
  dry_run=true
  shift
fi
[ "$#" -eq 1 ] || usage

for program in docker sha256sum mktemp; do
  if ! command -v "$program" >/dev/null 2>&1; then
    echo "$program is required" >&2
    exit 1
  fi
done

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"
compose=(docker compose -f compose.postgres.yaml)
"${compose[@]}" config --quiet

backup_dir="$1"
if [ "$dry_run" = true ]; then
  echo "Would write a verified PostgreSQL custom-format dump under: $backup_dir"
  exit 0
fi

mkdir -p -- "$backup_dir"
backup_dir="$(cd "$backup_dir" && pwd)"
umask 077
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
output="$backup_dir/rmm-postgres-$stamp.dump"
if [ -e "$output" ]; then
  echo "backup already exists: $output" >&2
  exit 1
fi
temporary="$(mktemp "$backup_dir/.rmm-postgres-$stamp.XXXXXX")"
trap 'rm -f -- "$temporary"' EXIT

"${compose[@]}" exec -T rmm-server sh -c \
  'PGSSLMODE=verify-full PGSSLROOTCERT=/run/postgres-tls/ca.crt pg_dump -h postgres -U rmm -d rmm --format=custom' \
  > "$temporary"
"${compose[@]}" exec -T rmm-server pg_restore --list < "$temporary" > /dev/null
if [ ! -s "$temporary" ]; then
  echo "PostgreSQL dump is empty" >&2
  exit 1
fi
mv -n -- "$temporary" "$output"
if [ -e "$temporary" ]; then
  echo "backup filename already exists: $output" >&2
  exit 1
fi
trap - EXIT
sha256sum "$output" > "$output.sha256"
echo "Created $output and $output.sha256"

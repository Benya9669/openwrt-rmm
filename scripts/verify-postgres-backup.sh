#!/usr/bin/env bash
set -euo pipefail
umask 077

usage() {
  echo 'Usage: verify-postgres-backup.sh --dump FILE --sha256 HEX --encryption-key FILE --signing-key FILE --image SERVER_IMAGE [--postgres-image postgres:18] [--expected FILE] [--dry-run]'
}
dump='' expected_hash='' encryption='' signing='' image='' postgres_image='postgres:18' expected='' dry_run=0
while (($#)); do
  case "$1" in
    --dump|--sha256|--encryption-key|--signing-key|--image|--postgres-image|--expected)
      (($# >= 2)) || { usage >&2; exit 2; }
      case "$1" in
        --dump) dump=$2;; --sha256) expected_hash=$2;; --encryption-key) encryption=$2;;
        --signing-key) signing=$2;; --image) image=$2;; --postgres-image) postgres_image=$2;; --expected) expected=$2;;
      esac
      shift 2;;
    --dry-run) dry_run=1; shift;;
    --help) usage; exit 0;;
    *) usage >&2; exit 2;;
  esac
done
for command in docker sha256sum realpath mktemp od tr date sleep rm; do
  command -v "$command" >/dev/null || { echo "$command is required" >&2; exit 1; }
done
[[ -n "$image" && "$image" != -* && "$postgres_image" != -* && "$expected_hash" =~ ^[a-fA-F0-9]{64}$ ]] || { usage >&2; exit 2; }
for file in "$dump" "$encryption" "$signing"; do
  [[ -f "$file" && -r "$file" && -s "$file" ]] || { echo 'Dump and both existing keys must be readable, nonempty regular files' >&2; exit 1; }
done
dump=$(realpath "$dump"); encryption=$(realpath "$encryption"); signing=$(realpath "$signing")
actual_hash=$(sha256sum "$dump"); actual_hash=${actual_hash%% *}
[[ "$actual_hash" == "${expected_hash,,}" ]] || { echo 'Dump SHA-256 mismatch; restore was not started' >&2; exit 1; }
mounts=(--mount "type=bind,src=$encryption,dst=/recovery/encryption.key,readonly" --mount "type=bind,src=$signing,dst=/recovery/signing.pem,readonly")
verify_args=(--encryption-key /recovery/encryption.key --signing-key /recovery/signing.pem)
if [[ -n "$expected" ]]; then
  [[ -f "$expected" && -r "$expected" ]] || { echo 'Expected report is not readable' >&2; exit 1; }
  expected=$(realpath "$expected")
  mounts+=(--mount "type=bind,src=$expected,dst=/recovery/expected.json,readonly")
  verify_args+=(--expected /recovery/expected.json)
fi
if ((dry_run)); then
  echo 'Inputs and SHA-256 verified. Would create an isolated internal Docker network and disposable PostgreSQL, restore the dump, and run the read-only verifier.'
  exit 0
fi
network='' database='' verifier='' env_file=''
cleanup() {
  local result=$?
  trap - EXIT
  if [[ -n "$verifier" ]]; then docker rm -f -v "$verifier" >/dev/null || result=1; fi
  if [[ -n "$database" ]]; then docker rm -f -v "$database" >/dev/null || result=1; fi
  if [[ -n "$network" ]]; then docker network rm "$network" >/dev/null || result=1; fi
  if [[ -n "$env_file" ]]; then rm -f -- "$env_file" || result=1; fi
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
env_file=$(mktemp)
password=$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')
printf 'POSTGRES_PASSWORD=%s\nPOSTGRES_USER=rmm_recovery\nPOSTGRES_DB=rmm_recovery\nRMM_RECOVERY_DATABASE_URL=postgres://rmm_recovery:%s@recovery-db:5432/rmm_recovery?sslmode=disable\n' "$password" "$password" > "$env_file"
unset password
network=$(docker network create --internal "rmm-recovery-$(date +%s)-$$")
database=$(docker create --network "$network" --network-alias recovery-db --env-file "$env_file" "$postgres_image")
docker start "$database" >/dev/null
ready=0
for ((attempt=0; attempt<60; attempt++)); do
  if docker exec "$database" pg_isready -h 127.0.0.1 -U rmm_recovery -d rmm_recovery >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
((ready)) || { echo 'Disposable PostgreSQL did not become ready' >&2; exit 1; }
# No --clean, production connection, published port, or shared data volume.
docker exec -i "$database" pg_restore --exit-on-error --single-transaction --no-owner --no-acl -U rmm_recovery -d rmm_recovery < "$dump"
actual_hash=$(sha256sum "$dump"); actual_hash=${actual_hash%% *}
[[ "$actual_hash" == "${expected_hash,,}" ]] || { echo 'Dump changed during restore' >&2; exit 1; }
verifier=$(docker create --network "$network" --env-file "$env_file" "${mounts[@]}" --entrypoint /usr/local/bin/rmm-db-verify "$image" "${verify_args[@]}")
docker start -a "$verifier"
[[ $(docker inspect --format '{{.State.ExitCode}}' "$verifier") == 0 ]] || { echo 'Recovery verification failed' >&2; exit 1; }

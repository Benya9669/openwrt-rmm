#!/bin/sh
set -eu

user="${1:-}"
fingerprint="${2:-}"
token_file="${3:-/run/rmm-tunnel/auth-token}"
endpoint_file="${4:-/run/rmm-tunnel/auth-url}"
[ "$user" = "rmm-tunnel" ] || exit 1
case "$fingerprint" in
	SHA256:|SHA256:*[!A-Za-z0-9_+/=-]*) exit 1 ;;
	SHA256:*) ;;
	*) exit 1 ;;
esac

[ -r "$token_file" ] || exit 1
[ -r "$endpoint_file" ] || exit 1
token="$(tr -d '\r\n' < "$token_file")"
[ -n "$token" ] || exit 1
case "$token" in
	*[!A-Za-z0-9._~-]*) exit 1 ;;
esac

endpoint="$(tr -d '\r\n' < "$endpoint_file")"
case "$endpoint" in
	http://*|https://*) ;;
	*) exit 1 ;;
esac
printf 'header = "Authorization: Bearer %s"\nheader = "X-RMM-Key-Fingerprint: %s"\n' "$token" "$fingerprint" |
	exec curl --config - --fail --silent --show-error --max-time 3 -- "$endpoint"

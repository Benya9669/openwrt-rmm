#!/bin/sh
set -eu

user="${1:-}"
fingerprint="${2:-}"
[ "$user" = "rmm-tunnel" ] || exit 1
case "$fingerprint" in
	SHA256:|SHA256:*[!A-Za-z0-9_+/=-]*) exit 1 ;;
	SHA256:*) ;;
	*) exit 1 ;;
esac

token="${RMM_TUNNEL_AUTH_TOKEN:-}"
if [ -n "${RMM_TUNNEL_AUTH_TOKEN_FILE:-}" ]; then
	[ -r "$RMM_TUNNEL_AUTH_TOKEN_FILE" ] || exit 1
	token="$(tr -d '\r\n' < "$RMM_TUNNEL_AUTH_TOKEN_FILE")"
fi
[ -n "$token" ] || exit 1
case "$token" in
	*[!A-Za-z0-9._~-]*) exit 1 ;;
esac

endpoint="${RMM_TUNNEL_AUTH_URL:-http://rmm-server:8080/internal/tunnel/authorized-key}"
printf 'header = "Authorization: Bearer %s"\nheader = "X-RMM-Key-Fingerprint: %s"\n' "$token" "$fingerprint" |
	exec curl --config - --fail --silent --show-error --max-time 3 "$endpoint"

#!/bin/sh
set -eu

token_file="${1:-/run/rmm-tunnel/auth-token}"
endpoint_file="${2:-/run/rmm-tunnel/active-ports-url}"
poll_seconds="${3:-5}"
allowed_file="/run/rmm-tunnel/active-ports"
candidate_file="/run/rmm-tunnel/listening-ports"

case "$poll_seconds" in
	''|*[!0-9]*) exit 1 ;;
esac
[ "$poll_seconds" -ge 1 ] || exit 1

while :; do
	if [ -r "$token_file" ] && [ -r "$endpoint_file" ]; then
		token="$(tr -d '\r\n' < "$token_file")"
		endpoint="$(tr -d '\r\n' < "$endpoint_file")"
		temporary="${allowed_file}.tmp.$$"
		if printf 'header = "Authorization: Bearer %s"\n' "$token" |
			curl --config - --fail --silent --show-error --max-time 3 -- "$endpoint" > "$temporary"; then
			awk '/^[0-9]+$/ && $1 >= 22000 && $1 <= 22199 { print $1 }' "$temporary" |
				sort -n -u > "$allowed_file"
			netstat -lntp 2>/dev/null |
				awk '$6 == "LISTEN" { local = $4; owner = $7; sub(/^.*:/, "", local); split(owner, process, "/"); if (local ~ /^[0-9]+$/ && process[1] ~ /^[0-9]+$/) print local, process[1] }' > "$candidate_file"
			while read -r port pid; do
				case "$port" in
					22|''|*[!0-9]*) continue ;;
				esac
				if [ "$port" -ge 22000 ] && [ "$port" -le 22199 ] && ! grep -qx "$port" "$allowed_file"; then
					if [ -r "/proc/$pid/comm" ] && grep -qx 'sshd' "/proc/$pid/comm"; then
						echo "Closing revoked or expired tunnel listener on port $port (pid $pid)." >&2
						kill "$pid" 2>/dev/null || true
					fi
				fi
			done < "$candidate_file"
		fi
		rm -f "$temporary"
	fi
	sleep "$poll_seconds"
done

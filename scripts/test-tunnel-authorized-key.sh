#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT

mkdir -p "$fixture/bin"
cat > "$fixture/bin/curl" <<'EOF'
#!/bin/sh
set -eu
cat > "$OUTPUT_CONFIG"
: > "$OUTPUT_ARGS"
for argument in "$@"; do
	printf '%s\n' "$argument" >> "$OUTPUT_ARGS"
done
EOF
chmod 0755 "$fixture/bin/curl"

token='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
fingerprint='SHA256:MqSaF41dKrT6gkyJ8JH9tD1Gnb/kLLqthEnQ2zBgK7Y'
endpoint='http://rmm-server:8080/internal/tunnel/authorized-key'
printf '%s' "$token" > "$fixture/auth-token"
printf '%s' "$endpoint" > "$fixture/auth-url"
chmod 0400 "$fixture/auth-token" "$fixture/auth-url"

env -i \
	PATH="$fixture/bin:/usr/bin:/bin" \
	OUTPUT_CONFIG="$fixture/curl-config" \
	OUTPUT_ARGS="$fixture/curl-args" \
	"$repository_root/deploy/tunnel/authorized-key.sh" \
	rmm-tunnel "$fingerprint" "$fixture/auth-token" "$fixture/auth-url"

grep -Fxq "header = \"Authorization: Bearer $token\"" "$fixture/curl-config"
grep -Fxq "header = \"X-RMM-Key-Fingerprint: $fingerprint\"" "$fixture/curl-config"
grep -Fxq -- '--fail' "$fixture/curl-args"
grep -Fxq -- "$endpoint" "$fixture/curl-args"

chmod 0600 "$fixture/auth-token"
: > "$fixture/auth-token"
if env -i \
	PATH="$fixture/bin:/usr/bin:/bin" \
	OUTPUT_CONFIG="$fixture/rejected-config" \
	OUTPUT_ARGS="$fixture/rejected-args" \
	"$repository_root/deploy/tunnel/authorized-key.sh" \
	rmm-tunnel "$fingerprint" "$fixture/auth-token" "$fixture/auth-url"; then
	echo "empty tunnel authorization token was accepted" >&2
	exit 1
fi

echo "tunnel authorized-key tests passed"

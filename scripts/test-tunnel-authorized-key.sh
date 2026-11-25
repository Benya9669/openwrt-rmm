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
	sh "$repository_root/deploy/tunnel/authorized-key.sh" \
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
	sh "$repository_root/deploy/tunnel/authorized-key.sh" \
	rmm-tunnel "$fingerprint" "$fixture/auth-token" "$fixture/auth-url"; then
	echo "empty tunnel authorization token was accepted" >&2
	exit 1
fi

# Exercise the entrypoint against an isolated persisted volume and record the sshd options.
mkdir -p "$fixture/data"
touch "$fixture/data"/ssh_host_{ed25519,rsa}_key{,.pub}
printf '%s\n' 'existing shared public key fixture' > "$fixture/data/authorized_keys"
for command in passwd chown reaper; do
  printf '#!/bin/sh\nexit 0\n' > "$fixture/bin/$command"
done
printf '#!/bin/sh\necho 0\n' > "$fixture/bin/id"
printf '#!/bin/sh\nprintf "%%s\\n" "$@" > "$OUTPUT_ARGS"\n' > "$fixture/bin/sshd"
chmod +x "$fixture"/bin/{sshd,id,passwd,chown,reaper}
sed -e "s|/data|$fixture/data|g" -e "s|/bootstrap/|$fixture/bootstrap/|g" \
    -e "s|/run/rmm-tunnel|$fixture/runtime|g" -e "s|/usr/sbin/sshd|$fixture/bin/sshd|" \
    -e "s|/usr/local/bin/tunnel-session-reaper|$fixture/bin/reaper|" \
    "$repository_root/deploy/tunnel/entrypoint.sh" > "$fixture/entrypoint.sh"
run_mode() {
  env -i PATH="$fixture/bin:/usr/bin:/bin" OUTPUT_ARGS="$fixture/sshd-args" "$@" sh "$fixture/entrypoint.sh"
}
grep -Fxq 'AuthorizedKeysFile none' "$repository_root/deploy/tunnel/sshd_config"
run_mode
if grep -q 'AuthorizedKeysFile=' "$fixture/sshd-args"; then exit 1; fi
run_mode RMM_ALLOW_LEGACY_TUNNEL_KEY=true
grep -Fxq "AuthorizedKeysFile=$fixture/data/authorized_keys" "$fixture/sshd-args"
run_mode RMM_ALLOW_LEGACY_TUNNEL_KEY=true RMM_TUNNEL_AUTH_TOKEN="$token"
if grep -q 'AuthorizedKeysFile=' "$fixture/sshd-args"; then exit 1; fi
grep -Fxq 'existing shared public key fixture' "$fixture/data/authorized_keys"
run_mode
test ! -e "$fixture/runtime/auth-token"
if run_mode RMM_ALLOW_LEGACY_TUNNEL_KEY=true RMM_TUNNEL_AUTH_TOKEN_FILE="$fixture/auth-token"; then exit 1; fi

echo "tunnel authorization and key mode tests passed"

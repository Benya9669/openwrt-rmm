#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT

mkdir -p "$fixture/bin"
cat > "$fixture/bin/uci" <<'EOF'
#!/bin/sh
case "$*" in
	"-q get rmm-agent.main.migration_complete") printf '%s\n' 1 ;;
	"-q get rmm-agent.main.server_url") printf '%s\n' https://rmm.example.test ;;
	"-q get rmm-agent.main.allow_insecure_http") printf '%s\n' 0 ;;
	"-q get rmm-agent.main.reset_identity") printf '%s\n' "${RMM_TEST_RESET:-0}" ;;
	"-q get rmm-agent.main.enrollment_token") [ "${RMM_TEST_RESET:-0}" = "1" ] && printf '%s\n' new-grant || exit 1 ;;
	"-q get rmm-agent.main.interval_seconds") printf '%s\n' 60 ;;
	"-q get rmm-agent.main.check_targets") printf '%s\n' '1.1.1.1 8.8.8.8 9.9.9.9' ;;
	"-q get rmm-agent.main.tunnel_identity_file") printf '%s\n' /etc/rmm-agent/tunnel_key ;;
	"-q set rmm-agent.main.tunnel_identity_file=/etc/rmm-agent/tunnel_device_key") exit 0 ;;
	"-q set rmm-agent.main.reset_identity=0") exit 0 ;;
	"-q delete rmm-agent.main.enrollment_token") exit 1 ;;
	"-q commit rmm-agent") exit 0 ;;
	*) printf 'unexpected uci call: %s\n' "$*" >&2; exit 2 ;;
esac
EOF
chmod +x "$fixture/bin/uci"

for sync_script in \
	"agent/package/rmm-agent-go-production/files/usr/libexec/rmm-agent-uci-sync" \
	"agent/package/rmm-agent/files/usr/libexec/rmm-agent-uci-sync"; do
	runtime_config="$fixture/$(basename "$(dirname "$(dirname "$(dirname "$sync_script")")")").conf"
	cat > "$runtime_config" <<'EOF'
SERVER_URL="https://old.example.test"
ENROLLMENT_TOKEN=""
INTERVAL_SECONDS="60"
CHECK_TARGETS="1.1.1.1"
TUNNEL_IDENTITY_FILE="/etc/rmm-agent/tunnel_key"
DEVICE_ID="router-1"
DEVICE_TOKEN="device-secret"
EOF

	PATH="$fixture/bin:$PATH" \
		UCI_CONFIG="rmm-agent" \
		SHELL_CONFIG="$runtime_config" \
		sh "$repository_root/$sync_script"

	grep -Fxq 'CHECK_TARGETS="1.1.1.1 8.8.8.8 9.9.9.9"' "$runtime_config"
	grep -Fxq 'DEVICE_ID="router-1"' "$runtime_config"
	grep -Fxq 'DEVICE_TOKEN="device-secret"' "$runtime_config"
	case "$sync_script" in
		*go-production*)
			grep -Fxq 'TUNNEL_DEVICE_IDENTITY_FILE="/etc/rmm-agent/tunnel_device_key"' "$runtime_config"
			grep -Fxq 'TUNNEL_KEY_EPOCH="1"' "$runtime_config"
			;;
	esac
done

reset_config="$fixture/reset.conf"
cat > "$reset_config" <<'EOF'
SERVER_URL="https://old.example.test"
TUNNEL_KEY_EPOCH="4"
COMMAND_SIGNING_PUBLIC_KEY="ed25519:old"
COMMAND_SIGNING_KEY_ID="old-key"
DEVICE_ID="router-1"
DEVICE_TOKEN="device-secret"
DEVICE_TOKEN_EPOCH="4"
EOF

PATH="$fixture/bin:$PATH" \
	RMM_TEST_RESET=1 \
	UCI_CONFIG="rmm-agent" \
	SHELL_CONFIG="$reset_config" \
	sh "$repository_root/agent/package/rmm-agent-go-production/files/usr/libexec/rmm-agent-uci-sync"

grep -Fxq 'ENROLLMENT_TOKEN="new-grant"' "$reset_config"
grep -Fxq 'DEVICE_ID=""' "$reset_config"
grep -Fxq 'DEVICE_TOKEN=""' "$reset_config"
grep -Fxq 'DEVICE_TOKEN_EPOCH="1"' "$reset_config"
grep -Fxq 'TUNNEL_KEY_EPOCH="1"' "$reset_config"
grep -Fxq 'COMMAND_SIGNING_PUBLIC_KEY=""' "$reset_config"
grep -Fxq 'COMMAND_SIGNING_KEY_ID=""' "$reset_config"

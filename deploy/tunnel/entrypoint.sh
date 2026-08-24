#!/bin/sh
set -eu

mkdir -p /data

if ! id rmm-tunnel >/dev/null 2>&1; then
	adduser -D -h /home/rmm-tunnel -s /bin/sh rmm-tunnel
fi
passwd -d rmm-tunnel >/dev/null 2>&1 || true

auth_dir="/run/rmm-tunnel"
auth_token_file="$auth_dir/auth-token"
auth_url_file="$auth_dir/auth-url"
active_ports_url_file="$auth_dir/active-ports-url"
token="${RMM_TUNNEL_AUTH_TOKEN:-}"
if [ -n "${RMM_TUNNEL_AUTH_TOKEN_FILE:-}" ]; then
	[ -r "$RMM_TUNNEL_AUTH_TOKEN_FILE" ] || {
		echo "Tunnel authorization token file is not readable." >&2
		exit 1
	}
	token="$(tr -d '\r\n' < "$RMM_TUNNEL_AUTH_TOKEN_FILE")"
fi
endpoint="${RMM_TUNNEL_AUTH_URL:-http://rmm-server:8080/internal/tunnel/authorized-key}"
active_ports_endpoint="${RMM_TUNNEL_ACTIVE_PORTS_URL:-http://rmm-server:8080/internal/tunnel/active-ports}"

if [ -n "$token" ]; then
	if [ "${#token}" -lt 32 ] || [ "${#token}" -gt 256 ]; then
		echo "Tunnel authorization token length is invalid." >&2
		exit 1
	fi
	case "$token" in
		*[!A-Za-z0-9._~-]*)
			echo "Tunnel authorization token contains invalid characters." >&2
			exit 1
			;;
	esac
	case "$endpoint" in
		http://*|https://*) ;;
		*)
			echo "Tunnel authorization URL is invalid." >&2
			exit 1
			;;
	esac
	case "$endpoint" in
		*[[:space:]]*)
			echo "Tunnel authorization URL contains whitespace." >&2
			exit 1
			;;
	esac
	case "$active_ports_endpoint" in
		http://*|https://*) ;;
		*)
			echo "Tunnel active ports URL is invalid." >&2
			exit 1
			;;
	esac
	case "$active_ports_endpoint" in
		*[[:space:]]*)
			echo "Tunnel active ports URL contains whitespace." >&2
			exit 1
			;;
	esac

	mkdir -p "$auth_dir"
	printf '%s' "$token" > "$auth_token_file"
	printf '%s' "$endpoint" > "$auth_url_file"
	printf '%s' "$active_ports_endpoint" > "$active_ports_url_file"
	nobody_gid="$(id -g nobody)"
	chown "0:$nobody_gid" "$auth_dir" "$auth_token_file" "$auth_url_file" "$active_ports_url_file"
	chmod 0750 "$auth_dir"
	chmod 0440 "$auth_token_file" "$auth_url_file" "$active_ports_url_file"
else
	rm -f "$auth_token_file" "$auth_url_file" "$active_ports_url_file"
fi

# OpenSSH intentionally sanitizes the environment of AuthorizedKeysCommand.
# The command reads the root-created runtime files above instead, so the
# authorization token is not inherited by the long-running sshd process.
unset RMM_TUNNEL_AUTH_TOKEN RMM_TUNNEL_AUTH_TOKEN_FILE RMM_TUNNEL_AUTH_URL RMM_TUNNEL_ACTIVE_PORTS_URL

if [ ! -f /data/ssh_host_ed25519_key ]; then
	ssh-keygen -q -t ed25519 -N '' -f /data/ssh_host_ed25519_key
fi
if [ ! -f /data/ssh_host_rsa_key ]; then
	ssh-keygen -q -t rsa -b 3072 -N '' -f /data/ssh_host_rsa_key
fi

if [ -n "$token" ]; then
	# Per-device keys are resolved for each authentication attempt. Keep the static
	# file present but empty so a previously shared bootstrap key cannot bypass revocation.
	: > /data/authorized_keys
elif [ -f /bootstrap/router_tunnel_key.pub ]; then
	printf 'restrict,port-forwarding %s\n' "$(cat /bootstrap/router_tunnel_key.pub)" > /data/authorized_keys
else
	if [ ! -f /data/router_tunnel_key ]; then
		ssh-keygen -q -t ed25519 -N '' -C 'rmm-router-tunnel' -f /data/router_tunnel_key
	fi
	if [ ! -f /data/authorized_keys ]; then
		printf 'restrict,port-forwarding %s\n' "$(cat /data/router_tunnel_key.pub)" > /data/authorized_keys
	fi
fi

chmod 0600 /data/ssh_host_ed25519_key /data/ssh_host_rsa_key /data/authorized_keys
chmod 0644 /data/ssh_host_ed25519_key.pub /data/ssh_host_rsa_key.pub
if [ -f /data/router_tunnel_key ]; then
	chmod 0600 /data/router_tunnel_key
	chmod 0644 /data/router_tunnel_key.pub
fi
chown rmm-tunnel:rmm-tunnel /data/authorized_keys

if [ -n "$token" ]; then
	/usr/local/bin/tunnel-session-reaper "$auth_token_file" "$active_ports_url_file" 5 &
fi

echo "Tunnel SSH endpoint ready."
exec /usr/sbin/sshd -D -e -f /etc/ssh/sshd_config

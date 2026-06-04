#!/bin/sh
set -eu

mkdir -p /data

if ! id rmm-tunnel >/dev/null 2>&1; then
	adduser -D -h /home/rmm-tunnel -s /bin/sh rmm-tunnel
fi
passwd -d rmm-tunnel >/dev/null 2>&1 || true

if [ ! -f /data/ssh_host_ed25519_key ]; then
	ssh-keygen -q -t ed25519 -N '' -f /data/ssh_host_ed25519_key
fi
if [ ! -f /data/ssh_host_rsa_key ]; then
	ssh-keygen -q -t rsa -b 3072 -N '' -f /data/ssh_host_rsa_key
fi

if [ ! -f /data/router_tunnel_key ]; then
	ssh-keygen -q -t ed25519 -N '' -C 'rmm-router-tunnel' -f /data/router_tunnel_key
fi

if [ ! -f /data/authorized_keys ]; then
	printf 'restrict,port-forwarding %s\n' "$(cat /data/router_tunnel_key.pub)" > /data/authorized_keys
fi

chmod 0600 /data/ssh_host_ed25519_key /data/ssh_host_rsa_key /data/router_tunnel_key /data/authorized_keys
chmod 0644 /data/ssh_host_ed25519_key.pub /data/ssh_host_rsa_key.pub /data/router_tunnel_key.pub
chown rmm-tunnel:rmm-tunnel /data/authorized_keys

echo "Tunnel SSH endpoint ready. Install /data/router_tunnel_key on each approved router."
exec /usr/sbin/sshd -D -e -f /etc/ssh/sshd_config

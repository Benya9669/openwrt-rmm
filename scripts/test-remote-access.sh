#!/usr/bin/env bash
set -euo pipefail

for command in docker go ssh ssh-keygen; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "$command is required" >&2
    exit 1
  fi
done
if [ "$(uname -s)" != Linux ]; then
  echo "Real-agent remote access E2E requires Linux." >&2
  exit 1
fi
repo_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"
agent_binary="$(mktemp)"
tunnel_id=""
cleanup() {
  if [ -n "$tunnel_id" ]; then
    # This exact ID belongs to the isolated container created below; /data is tmpfs.
    docker rm --force "$tunnel_id" >/dev/null
  fi
  rm -f -- "$agent_binary"
}
trap cleanup EXIT
go build -o "$agent_binary" ./agent/go/cmd/rmm-agent
docker build -t rmm-tunnel-e2e deploy/tunnel
tunnel_id="$(docker run --detach --tmpfs /data --cap-add SYS_PTRACE --label rmm.test=tunnel-e2e \
  --add-host host.docker.internal:host-gateway \
  -p 127.0.0.1:18086:22 -p 127.0.0.1:22055:22055 -p 127.0.0.1:22155:22155 \
  -e RMM_TUNNEL_AUTH_TOKEN=test-only-tunnel-authorization-1234567890 \
  -e RMM_TUNNEL_AUTH_URL=http://host.docker.internal:18085/internal/tunnel/authorized-key \
  -e RMM_TUNNEL_ACTIVE_PORTS_URL=http://host.docker.internal:18085/internal/tunnel/active-ports \
  rmm-tunnel-e2e)"
for attempt in {1..30}; do
  if docker exec "$tunnel_id" test -s /data/ssh_host_ed25519_key.pub; then break; fi
  sleep 1
done
export RMM_TEST_AGENT_BINARY="$agent_binary"
export RMM_TEST_API_BIND=0.0.0.0:18085
export RMM_TEST_HOST=127.0.0.1
export RMM_TEST_TUNNEL_SSH_PORT=18086
export RMM_TEST_TUNNEL_HOST_KEY
RMM_TEST_TUNNEL_HOST_KEY="$(docker exec "$tunnel_id" cat /data/ssh_host_ed25519_key.pub)"
if ! go test -race -count=1 -v ./server/internal/httpapi -run '^TestReverseTunnelCloudLuCIE2E$' -timeout 120s; then
  docker logs "$tunnel_id"
  exit 1
fi

# NPMplus Reverse Proxy

NPMplus can terminate HTTPS and proxy the RMM web UI, browser authentication, HTTP API, and agent traffic.

The RMM application keeps its own operator login. Do not apply an NPMplus Access List to the entire RMM Proxy Host because managed routers must reach `/api/agent/*` without an interactive NPMplus login.

## NPMplus In Host Network Mode

When NPMplus uses `network_mode: host`, it reaches host-published services through `127.0.0.1`. A shared Docker network and Docker service DNS are not used.

Set the RMM HTTP port to listen only on host loopback:

```env
RMM_COOKIE_SECURE=true
RMM_HTTP_BIND_IP=127.0.0.1
RMM_HTTP_PORT=18080
```

Start RMM with the NPMplus host-mode settings overlay:

```sh
docker compose -f compose.yaml -f compose.npmplus.yaml up -d --build
```

## NPMplus Proxy Host

Create a Proxy Host with:

| Field | Value |
|---|---|
| Domain Names | `rmm.example.com` |
| Scheme | `http` |
| Forward Hostname / IP | `127.0.0.1` |
| Forward Port | `18080` |
| Certificate | your domain certificate |
| Force SSL | enabled |
| HTTP/2 / HTTP/3 | optional |
| Access List | public / none |

No WebSocket or custom Advanced configuration is currently required.

The RMM login page remains the authentication boundary for operators. Agents continue using their device bearer credentials on the same domain.

Because NPMplus shares the Linux host network namespace, `127.0.0.1:18080` points to the RMM port published on the host.

## SSH Reverse Tunnel Ports

HTTP Proxy Hosts do not handle SSH. The current tunnel service publishes:

- `2222/tcp`: routers connect to the reverse tunnel SSH service.
- `22000-22099/tcp`: operators connect to active router sessions.

Recommended policy:

- allow `2222/tcp` from managed router networks;
- allow `22000-22099/tcp` only from operator VPN/trusted networks;
- keep these ports blocked from unrelated internet clients.

NPMplus Streams can proxy individual TCP ports, but creating a Stream for every temporary port is inconvenient. Direct firewall-controlled ports or a VPN are better for `22000-22099`.

## Cloudflare Note

If the domain is proxied through Cloudflare, normal HTTP RMM traffic can work, but Cloudflare's standard HTTP proxy does not forward SSH ports such as `2222` or `22000-22099`. Those require direct TCP reachability or another tunnel/VPN solution.

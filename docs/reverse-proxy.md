# Reverse Proxy

The web UI and HTTP API work through a standard HTTP reverse proxy. The application uses same-origin requests and an `HttpOnly` signed session cookie.

For NPMplus, use the dedicated [npmplus.md](npmplus.md) guide. The included Caddy overlay is only an optional standalone alternative.

## What The Proxy Handles

- Web UI.
- Operator login/logout.
- Operator HTTP API.
- Agent enrollment, heartbeat, command polling, and command results.

## What The Proxy Does Not Handle

An ordinary HTTP reverse proxy does not proxy SSH:

- `2222/tcp` must remain reachable by managed routers, or be handled by a TCP proxy.
- `22000-22099/tcp` must remain reachable by trusted operators, or be handled by a TCP proxy/VPN.

## Included Caddy Configuration

Set a public DNS name in `.env`:

```env
RMM_DOMAIN=rmm.example.com
RMM_COOKIE_SECURE=true
RMM_HTTP_BIND_IP=127.0.0.1
```

Start the base stack with the proxy overlay:

```sh
docker compose -f compose.yaml -f compose.proxy.yaml up -d --build
```

Caddy automatically obtains and renews a TLS certificate when:

- the domain resolves to the server;
- ports `80` and `443` are reachable from the internet.

Open:

```text
https://rmm.example.com
```

## Existing Reverse Proxy

Proxy HTTP traffic to:

```text
http://rmm-server:8080
```

When the browser uses HTTPS, set:

```env
RMM_COOKIE_SECURE=true
```

Preserve the original `Host` header and forward client/protocol headers. Example Nginx location:

```nginx
location / {
    proxy_pass http://127.0.0.1:18080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

No WebSocket settings are currently required.

## Recommended Network Policy

- Public or VPN operator access: `443/tcp`.
- Router tunnel access: `2222/tcp`.
- Trusted operator/VPN access only: `22000-22099/tcp`.
- Do not expose the direct application port `18080` publicly when using a reverse proxy.

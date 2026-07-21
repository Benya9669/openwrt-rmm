# RMM device names (KeenDNS-like cloud mode)

The first implementation uses cloud mode: every router receives a unique name such as
`office.routers.example.com`, while DNS points all names to the RMM reverse proxy. The
router itself keeps an outbound reverse SSH tunnel, so no public WAN address or inbound
router firewall rule is required.

## Request flow

1. A user creates a 15-minute, one-time enrollment grant in the RMM UI.
2. The OpenWrt agent consumes the grant and becomes owned by that user.
3. RMM assigns the requested DNS label, or generates one from the hostname.
4. The user opens a temporary LuCI tunnel and asks RMM for an access link.
5. The link is valid for 60 seconds and exchanges itself for a host-only cookie on the
   router subdomain. The control-panel cookie is never sent to that subdomain.

## DNS

Create a wildcard record for the device domain:

```text
*.routers.example.com.  A     203.0.113.10
*.routers.example.com.  AAAA  2001:db8::10   # only when IPv6 reaches the proxy
```

The wildcard target must be the public address of the reverse proxy, not a router WAN
address. Keep the control panel on a separate host, for example `rmm.example.com`.

## Wildcard TLS

Public ACME wildcard certificates require DNS-01 validation. Obtain a certificate for
`*.routers.example.com` through your DNS provider or a Caddy build with that provider's
DNS module. Store the resulting certificate and key outside Git, set:

```text
RMM_DEVICE_DOMAIN=routers.example.com
RMM_TUNNEL_PUBLIC_HOST=rmm.example.com
RMM_TUNNEL_PUBLIC_PORT=2222
RMM_WILDCARD_CERT_PATH=./secrets/routers-wildcard.crt
RMM_WILDCARD_KEY_PATH=./secrets/routers-wildcard.key
```

Then validate and start the overlays:

```sh
docker compose -f compose.yaml -f compose.proxy.yaml -f compose.keendns.yaml config
docker compose -f compose.yaml -f compose.proxy.yaml -f compose.keendns.yaml up -d --build
```

`RMM_TUNNEL_PUBLIC_HOST` is the public address reached by the router's outbound SSH
connection. It defaults to the hostname from `RMM_PUBLIC_URL`; set it explicitly when
the tunnel uses another hostname. `RMM_TUNNEL_PUBLIC_PORT` must match the public port
published for `tunnel-ssh` (2222 by default).

The operator UI uses `/api/devices/{id}/cloud-access` as a single cloud-access flow. It
reuses an existing healthy tunnel, waits while a new tunnel starts, and distinguishes an
offline router from a tunnel that never became reachable. Host, ports, and LuCI scheme
remain available only under the expert settings.

Do not enable `RMM_ALLOW_LEGACY_LUCI_PROXY` in production. It exists only for migration
from the old same-origin `/luci/...` route.

## Cloud-only addressing

Device names always resolve to the RMM reverse proxy. The agent never discovers or
publishes the router's public WAN addresses, and users do not need inbound firewall rules.
Legacy direct-DNS database tables are retained only for non-destructive upgrades and are
not exposed through the API or populated for new devices.

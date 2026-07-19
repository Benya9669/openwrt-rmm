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
RMM_WILDCARD_CERT_PATH=./secrets/routers-wildcard.crt
RMM_WILDCARD_KEY_PATH=./secrets/routers-wildcard.key
```

Then validate and start the overlays:

```sh
docker compose -f compose.yaml -f compose.proxy.yaml -f compose.keendns.yaml config
docker compose -f compose.yaml -f compose.proxy.yaml -f compose.keendns.yaml up -d --build
```

Do not enable `RMM_ALLOW_LEGACY_LUCI_PROXY` in production. It exists only for migration
from the old same-origin `/luci/...` route.

## Future direct mode

A direct mode can later publish a router's verified public WAN address and update it when
the agent reports a change. That mode needs authoritative DNS-provider integration,
reachability checks, certificate issuance on the router, and explicit firewall policy;
it is intentionally not mixed into the cloud-mode security boundary.

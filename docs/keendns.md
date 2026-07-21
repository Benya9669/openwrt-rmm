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

## Direct DNS mode backend

The server now keeps a separate direct-DNS record for every enrolled router: unique DNS
label, public IPv4/IPv6 addresses, TTL, enabled state, last agent update, and the latest
100 address changes. Existing devices receive a record automatically during database
migration. A transfer to another account clears the addresses and history and disables
publishing, so the previous owner's WAN data is not exposed.

Only the router's own device token can update its addresses. Private, loopback,
link-local, documentation, benchmark, CGNAT, multicast, and otherwise non-public
addresses are rejected. Updates are limited to 30 authenticated attempts per router per
minute. Browser APIs retain the normal per-user device isolation and record all settings
changes in the audit log.

An authoritative DNS synchronizer can read enabled records through a separate bearer
credential. Generate a random secret of at least 32 characters and keep it outside Git:

```text
RMM_DNS_SYNC_TOKEN=<random-secret>
```

```http
GET /api/internal/dns/records
Authorization: Bearer <RMM_DNS_SYNC_TOKEN>
```

When the variable is unset, this endpoint responds with `404`. Its response has
`Cache-Control: no-store` and contains only enabled records that have at least one public
address.

Go agent `0.6.0` adds opt-in public-address discovery and periodic authenticated updates.
It can be configured from LuCI and uses separate HTTPS endpoints for IPv4 and IPv6. Direct
DNS is disabled by default and does not affect heartbeat health when a discovery service is
temporarily unavailable.

Publishing records to an authoritative DNS provider, reachability checks, router-side
certificate issuance, firewall policy, and the RMM web interface remain separate follow-up
stages. Cloud-mode LuCI names continue to work unchanged.

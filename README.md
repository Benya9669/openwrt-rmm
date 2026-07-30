<p align="center">
  <img src="web/web-app-192.png" width="112" alt="OpenWrt RMM logo">
</p>

# OpenWrt RMM

[Русская версия](README_RU.md) · [Releases](https://github.com/Benya9669/openwrt-rmm/releases) · [Changelog](CHANGELOG.md) · [Security](docs/security.md)

OpenWrt RMM is a self-hosted remote monitoring and management platform for OpenWrt
routers. It combines a Go cloud server, a lightweight outbound agent, a responsive web
dashboard, temporary SSH/LuCI access, notifications, and signed OpenWrt package feeds.

The agent does not require an inbound port on the router. It connects to the server over
HTTPS and opens a restricted reverse tunnel only when an authorized operator requests
remote access.

## Highlights

- Separate user accounts and router ownership.
- One-time router enrollment grants.
- Live inventory, health metrics, connectivity checks, and LAN client presence.
- Safe allowlisted diagnostics and package/UCI operations.
- Temporary SSH and LuCI access through an isolated cloud tunnel.
- Built-in notification center, e-mail, Telegram, and signed webhooks.
- Quiet hours, maintenance pauses, and per-router alert preferences.
- Responsive web UI and a LuCI application for agent configuration.
- Signed IPK/APK repositories for supported OpenWrt releases.

## Components

```text
Browser ──HTTPS──> RMM server ──SQLite
                       │
                       ├── notification workers
                       └── isolated SSH tunnel service
                                      ▲
OpenWrt router ──outbound HTTPS/SSH───┘
  ├── rmm-agent-go-production
  ├── luci-app-rmm-agent
  └── luci-i18n-rmm-agent-ru (optional)
```

| Component | Purpose | License |
| --- | --- | --- |
| `server/`, `web/` | API, persistence, dashboard, notifications | AGPL-3.0-only |
| `deploy/tunnel/` | Restricted reverse SSH tunnel service | AGPL-3.0-only |
| `agent/` | Go agent, LuCI app, and OpenWrt packages | MIT |

## Deployment

Requirements:

- Docker Engine with Compose v2;
- an HTTPS reverse proxy such as NPMplus;
- a wildcard DNS record and certificate for cloud LuCI access;
- an SSH key dedicated to router tunnels.

Copy the example configuration and follow the production deployment guide:

```sh
cp .env.example .env
docker compose -f compose.yaml -f compose.release.yaml pull
docker compose -f compose.yaml -f compose.release.yaml up -d
docker compose -f compose.yaml -f compose.release.yaml ps
```

Do not start production with the example secrets. Generate the tunnel key and configure
the required environment values first.

- [Docker Compose deployment](docs/docker-compose.md)
- [NPMplus configuration](docs/npmplus.md)
- [KeenDNS-like wildcard router access](docs/keendns.md)
- [Architecture](docs/architecture.md)

## Installing the OpenWrt agent

The recommended method is the signed package feed:

- OpenWrt 24.10 and older: IPK/opkg;
- OpenWrt 25.12 and newer: APK;
- OpenWrt 21.02, 22.03, and 23.05: legacy support tier.

Install the runtime and LuCI application:

```sh
opkg install rmm-agent-go-production luci-app-rmm-agent
# or
apk add rmm-agent-go-production luci-app-rmm-agent
```

English is the default LuCI language. Install the optional Russian translation:

```sh
opkg install luci-i18n-rmm-agent-ru
# or
apk add luci-i18n-rmm-agent-ru
```

See [Signed OpenWrt package repository](docs/package-repository.md) for feed URLs,
verification keys, and architecture-specific instructions.

## Development

```sh
go test ./...
go vet ./...
node --check web/app.js
docker compose config
```

Run the server in explicit local development mode:

```sh
RMM_INSECURE_DEV_MODE=true \
RMM_OPERATOR_PASSWORD='replace-with-a-long-development-password' \
go run ./server/cmd/rmm-server
```

Never enable insecure development mode on an internet-facing deployment.

## Releases

The server and agent use independent version lines:

- `server-v*` publishes the server and tunnel container images;
- `agent-v*` publishes installable IPK/APK packages;
- release descriptions are maintained in English in [CHANGELOG.md](CHANGELOG.md).

GitHub Releases contain only installable packages. Signed repository indexes and public
keys are published through GitHub Pages, while provenance is kept in GitHub attestations.
See [Release policy](RELEASES.md).

## Security and license

Review [the security model](docs/security.md) before exposing the service publicly.
Back up the SQLite volume before every server upgrade.

The cloud application is licensed under `AGPL-3.0-only`; the OpenWrt agent and LuCI
packages are licensed under MIT. See [LICENSE.md](LICENSE.md) and [NOTICE.md](NOTICE.md).

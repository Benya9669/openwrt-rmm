# Release policy

OpenWrt RMM versions the cloud application and router bundle independently with Semantic
Versioning.

## Server releases

Tags use `server-vMAJOR.MINOR.PATCH`, for example `server-v0.8.0`.

A server release contains the Go API, web interface, database migrations and deployment
files. The GitHub Actions server workflow tests the server and publishes:

```text
ghcr.io/benya9669/openwrt-rmm-server:0.8.0
ghcr.io/benya9669/openwrt-rmm-server:latest
```

Increment `MAJOR` for incompatible API or migration requirements, `MINOR` for compatible
features, and `PATCH` for compatible fixes. Database backups remain mandatory before a
production upgrade even when the release is marked compatible.

## Agent releases

Tags use `agent-vMAJOR.MINOR.PATCH`, for example `agent-v0.6.2`.

An agent release contains the Go runtime, LuCI application and OpenWrt IPK/APK packages.
Before tagging, the tag version must match `agentVersion` in the Go source and
`PKG_VERSION` in the production Go package. CI rejects a mismatched release tag.

The LuCI application is shipped as part of the router bundle. It can retain its own package
version for package-manager upgrades, but it does not require a separate GitHub release
line unless it later becomes an independent product.

## Protocol compatibility

Server and agent versions do not need to match. Compatibility is determined by the agent
API protocol, currently `v1`. Within the same protocol major version:

- a newer server should continue to accept supported older agents;
- a newer agent should tolerate unknown response fields and optional server features;
- required capabilities must be negotiated or reported explicitly rather than inferred
  only from the package version.

An intentional incompatible protocol change requires a new API version and a documented
transition period; it should not silently reuse `v1`.

## Release commands

```sh
git tag -a server-v0.8.0 -m "OpenWrt RMM Server 0.8.0"
git push origin server-v0.8.0

git tag -a agent-v0.6.2 -m "OpenWrt RMM Agent 0.6.2"
git push origin agent-v0.6.2
```

Pushing a server tag publishes the container image and creates a GitHub Release. Pushing
an agent tag builds the full OpenWrt matrix and creates a GitHub Release with packages and
checksums. `workflow_dispatch` can test either workflow without creating a release.

# Release policy

OpenWrt RMM versions the cloud application and router bundle independently with Semantic
Versioning.

## Server releases

Tags use `server-vMAJOR.MINOR.PATCH`, for example `server-v0.8.1`.

A server release contains the Go API, web interface, database migrations and the coupled
SSH reverse-tunnel service. The GitHub Actions server workflow tests the server and
publishes two images with the same version:

```text
ghcr.io/benya9669/openwrt-rmm-server:0.8.1
ghcr.io/benya9669/openwrt-rmm-server:latest
ghcr.io/benya9669/openwrt-rmm-tunnel:0.8.1
ghcr.io/benya9669/openwrt-rmm-tunnel:latest
```

The tunnel remains a separate container for isolation, but shares the server release line
because its SSH policy and port-forwarding behavior form part of the server-side protocol.
Introduce independent `tunnel-v*` releases only if the tunnel later gains a separate API
and lifecycle.

Increment `MAJOR` for incompatible API or migration requirements, `MINOR` for compatible
features, and `PATCH` for compatible fixes. Database backups remain mandatory before a
production upgrade even when the release is marked compatible.

## Agent releases

Tags use `agent-vMAJOR.MINOR.PATCH`, for example `agent-v0.6.6`.

An agent release contains the Go runtime, LuCI application and OpenWrt IPK/APK packages.
Before tagging, the tag version must match `agentVersion` in the Go source and
`PKG_VERSION` in the production Go package. CI rejects a mismatched release tag.

Tagged releases automatically build the current OpenWrt 24.10 and 25.12 package matrix.
OpenWrt 21.02, 22.03 and 23.05 are a manual legacy tier that extends an existing agent
release without blocking current packages. Run it after the tagged workflow completes:

```sh
gh workflow run build-legacy.yml -f agent_tag=agent-v0.6.6
```

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
git tag -a server-v0.8.1 -m "OpenWrt RMM Server 0.8.1"
git push origin server-v0.8.1

git tag -a agent-v0.6.6 -m "OpenWrt RMM Agent 0.6.6"
git push origin agent-v0.6.6
```

Pushing a server tag publishes the container image and creates a GitHub Release. Pushing
an agent tag builds the full OpenWrt matrix and creates a GitHub Release with packages and
checksums. It also publishes signed package feeds through GitHub Pages. Repository setup,
key generation and router configuration are documented in
[`docs/package-repository.md`](docs/package-repository.md). `workflow_dispatch` can test
either workflow without creating a release.

Server and tunnel images are published with SBOM/provenance attestations and keyless
Sigstore signatures bound to the release workflow identity. Agent checksums receive the
same GitHub OIDC-backed signature. IPK and APK feeds additionally use their native OpenWrt
repository signatures so `opkg` and `apk` can enforce trust on the router.

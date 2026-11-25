# Release policy

OpenWrt RMM versions the cloud application and router bundle independently with Semantic
Versioning.

## Server releases

Tags use `server-vMAJOR.MINOR.PATCH`, for example `server-v0.13.0`.

A server release contains the Go API, web interface, database migrations and the coupled
SSH reverse-tunnel service. Starting with 0.13.0, it also contains the optional
PostgreSQL TLS initializer. The GitHub Actions server workflow tests the server and
publishes the images with the same version:

```text
ghcr.io/benya9669/openwrt-rmm-server:0.13.0
ghcr.io/benya9669/openwrt-rmm-server:latest
ghcr.io/benya9669/openwrt-rmm-tunnel:0.13.0
ghcr.io/benya9669/openwrt-rmm-tunnel:latest
ghcr.io/benya9669/openwrt-rmm-postgres-tls-init:0.13.0
ghcr.io/benya9669/openwrt-rmm-postgres-tls-init:latest
```

The tunnel remains a separate container for isolation, but shares the server release line
because its SSH policy and port-forwarding behavior form part of the server-side protocol.
Introduce independent `tunnel-v*` releases only if the tunnel later gains a separate API
and lifecycle.

Increment `MAJOR` for incompatible API or migration requirements, `MINOR` for compatible
features, and `PATCH` for compatible fixes. Database backups remain mandatory before a
production upgrade even when the release is marked compatible.

## Agent releases

Tags use `agent-vMAJOR.MINOR.PATCH`, for example `agent-v0.9.0`.

An agent release contains the Go runtime, LuCI application and OpenWrt IPK/APK packages.
Before tagging, the tag version must match `agentVersion` in the Go source and
`PKG_VERSION` in the production Go package. CI rejects a mismatched release tag.

Tagged releases automatically build the current OpenWrt 24.10 and 25.12 package matrix.
OpenWrt 21.02, 22.03 and 23.05 are a manual legacy tier that extends an existing agent
release without blocking current packages. Run it after the tagged workflow completes:

```sh
gh workflow run build-legacy.yml -f agent_tag=agent-v0.9.0
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

### Authorize a release signer

Configure these GitHub Actions repository variables before the first signed
server or agent release with the updated workflows. The manual legacy package
workflow uses the same trust policy when extending an existing agent release.

| Repository variable | Value |
| --- | --- |
| `RELEASE_OPENPGP_PUBLIC_KEYS` | ASCII-armored export containing the authorized public keys and their signing subkeys |
| `RELEASE_OPENPGP_FINGERPRINTS` | One verified primary-key fingerprint per line, in full hexadecimal form |

Verify the primary fingerprint with the key owner through an independent channel.
Export its public key with `gpg --armor --export FULL_PRIMARY_FINGERPRINT`.
An administrator can set both values in **Settings → Secrets and variables →
Actions → Variables**.

The workflows on tag pushes are read from the tagged commit. Enforce the
following controls in GitHub before enabling publication:

- Use repository rulesets for `agent-v*` and `server-v*`: restrict tag
  creation to authorized release maintainers and prevent tag updates and deletion.
  Protect the default branch. Require trusted CODEOWNERS approval for
  `.github/workflows/`, the tag verifier and the CODEOWNERS file itself.
- Protect the `rmm-release` environment with required reviewers, prevent
  self-review and limit deployments to the release tag patterns and the protected
  default branch used by manual legacy builds. Apply equivalent protections to
  `github-pages`, which also signs feed indexes. Reviewers must verify the
  requested commit belongs to reviewed release history before approving.
- Store the two package signing secrets `OPENWRT_USIGN_SECRET_B64` and
  `OPENWRT_APK_SECRET_B64` in the protected environments that consume them
  (`rmm-release`, `github-pages`). Remove repository-wide copies after the move.
  Keep `rmm-build`, used for unsigned branch smoke builds, free of signing keys.

Repository and environment protection settings require administrator setup outside
this checkout. A workflow change in an unrestricted tag could skip an in-tree
verifier and access any repository-wide signing secrets.
For a rotation, add the reviewed replacement before the first release it signs,
then remove the retired key from the allowlist at cutover.

CI verifies the signed tag object in an isolated keyring and matches its
primary fingerprint, tag name and checked-out commit. Missing configuration,
an invalid signature or an unlisted signer stops publication. With both
variables set locally, run `bash scripts/verify-release-tag.sh TAG` from the
tagged commit. The self-contained `bash scripts/test-verify-release-tag.sh`
checks acceptance and rejection using a temporary signing subkey.

### Build and scan the release inputs

The Go version in `go.mod` and the two builder images use the same patched
toolchain. Update all three pins together. From a Linux checkout, run
`CGO_ENABLED=0 go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./server/cmd/... ./agent/go/cmd/rmm-agent`
to check the release entry points against the current Go vulnerability database.

The root `.dockerignore` allows the server sources, embedded SQL, web assets,
entry point and APK public key. The OpenWrt builder and stable feed indexer have
their own `Dockerfile.dockerignore` alongside their Dockerfiles. When adding a
packaged file, update its allowlist and run `bash scripts/test-docker-context.sh`
with Docker Buildx. The fixture checks the exported file list for all three builds.

### Prepare the release

Before creating a tag, add an English `## <tag>` section to `CHANGELOG.md`. The release
workflow uses that section verbatim and fails instead of publishing empty autogenerated
notes.

```sh
git tag -s agent-v0.9.0 -m "OpenWrt RMM Agent 0.9.0"
git push origin agent-v0.9.0

git tag -s server-v0.13.0 -m "OpenWrt RMM Server 0.13.0"
git push origin server-v0.13.0
```

Pushing a server tag publishes the container image and creates a GitHub Release. Pushing
an agent tag builds the full OpenWrt matrix and creates a GitHub Release containing only
installable `.ipk` and `.apk` packages. Repository indexes, public signing keys and
provenance metadata stay in the signed GitHub Pages package feed and GitHub attestations.
Repository setup, key generation and router configuration are documented in
[`docs/package-repository.md`](docs/package-repository.md). `workflow_dispatch` can test
either workflow without creating a release.

Server, tunnel, and PostgreSQL TLS initializer images are published with SBOM/provenance attestations bound to the
release workflow identity. Agent packages receive GitHub build-provenance attestations.
IPK and APK feeds additionally use their native OpenWrt repository signatures so `opkg`
and `apk` can enforce trust on the router.

The package repository also contains `update-manifest.json`, a detached ECDSA signature,
and its Sigstore bundle. The server verifies the detached signature before adopting a
version from the manifest, then exposes the trusted stable version through authenticated
release metadata. The dashboard compares versions with Semantic Versioning and never
recommends downgrading an agent newer than the stable channel.

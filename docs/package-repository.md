# Signed OpenWrt package repository

Agent release tags automatically publish the current OpenWrt support tier:

- OpenWrt 24.10 and 25.12;
- `x86/64`, `ramips/mt7621`, `ath79/generic`, `ipq40xx/generic` and
  `mediatek/filogic`.

OpenWrt 21.02, 22.03 and 23.05 packages are added to an existing agent release by the
manual legacy workflow described below. The default public base URL is:

```text
https://packages.daemonlord.ru/feeds/stable/openwrt
```

The same Pages deployment publishes the stable update metadata:

```text
https://packages.daemonlord.ru/update-manifest.json
https://packages.daemonlord.ru/update-manifest.sig
https://packages.daemonlord.ru/update-manifest.sigstore.json
```

## LuCI theme and dashboard from a separate release

The source and GitHub Releases for `luci-theme-rmm` and
`luci-app-rmm-dashboard` live in
[`Benya9669/luci-theme-rmm`](https://github.com/Benya9669/luci-theme-rmm).
The agent's release builder can import those two packages into the same signed
`packages.daemonlord.ru` feed as the agent. The feed index is signed with the
existing RMM package key; routers need only the existing feed and key.

To include a LuCI release, publish its `luci-v*` tag. The LuCI workflow sends
the tag and SHA256 of the release's `SHA256SUMS` to the RMM feed workflow.
The checked-in `deploy/luci-release.lock` is a fallback for first deployment
and can be pinned manually for a release before the feed sync runs:

```text
Benya9669/luci-theme-rmm luci-v0.1.0 <64-character-SHA256-of-SHA256SUMS>
```

The builder verifies the locked manifest and every package hash before adding
the packages to the OpenWrt 24.10/25.12 feed indexes. A lock containing `none`
leaves the current agent release behavior unchanged. The LuCI release workflow
dispatches `sync-luci-feed.yml`, which downloads the latest complete feed
snapshot from the `package-repository-site` Actions artifact,
adds the packages to the **stable** target directories, rebuilds and signs
their indexes, and publishes the shared feed immediately. Existing versioned
agent feeds stay immutable. It publishes a signed `luci-release.lock` so the
next agent release includes the active theme packages in its new feed too.
Each successful feed deployment saves a fresh snapshot for 90 days. If no
snapshot is retained, publication stops before replacing the existing feed.
The separate LuCI GitHub Release keeps the downloadable theme/dashboard `.ipk`
and `.apk` files; they are not duplicated in the RMM agent GitHub Release.
Agent and legacy publication runs share the feed deployment lock with LuCI
sync. Legacy publication retains the signed active LuCI packages and feed
indexes while extending the older OpenWrt targets.

For a private LuCI repository, configure the RMM Actions secret
`LUCI_RELEASE_TOKEN` with read access to that repository's Releases.
The LuCI repository needs `RMM_FEED_DISPATCH_TOKEN` with **Contents: read/write**
on `Benya9669/openwrt-rmm` to send `repository_dispatch`. If that secret is not yet
configured, run **Sync LuCI release into signed package feed** manually with
the LuCI tag and SHA256 of its `SHA256SUMS` asset.

The manifest declares the stable agent version and compatible OpenWrt feed directories.
Both current and legacy workflows sign it with the existing APK package key for runtime
server verification and keylessly with Sigstore for workflow identity/provenance. Both
signatures are verified before deploying Pages.

Verify the package-key signature:

```sh
openssl dgst -sha256 \
  -verify rmm-openwrt.pem \
  -signature update-manifest.sig \
  update-manifest.json
```

Verify the Sigstore identity:

```sh
cosign verify-blob \
  --bundle update-manifest.sigstore.json \
  --certificate-identity-regexp '^https://github.com/Benya9669/openwrt-rmm/\.github/workflows/build(-legacy)?\.yml@refs/(tags/agent-v|heads/)' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  update-manifest.json
```

The Pages deployment uses the verified custom domain `packages.daemonlord.ru`. Runtime
configuration and signed manifests must use this canonical origin directly because the
underlying `github.io` address redirects to the custom domain and update verification
intentionally rejects cross-origin redirects.

## Signing keys

Keep private keys offline and out of the repository. Generate them on a trusted Linux or
WSL host with `usign` and OpenSSL installed:

```sh
umask 077
mkdir -p release-keys
usign -G \
  -s release-keys/openwrt-usign.sec \
  -p release-keys/openwrt-usign.pub \
  -c "OpenWrt RMM package repository"
openssl ecparam -name prime256v1 -genkey -noout \
  -out release-keys/openwrt-apk.pem
openssl ec -in release-keys/openwrt-apk.pem -pubout \
  -out release-keys/openwrt-apk.pub.pem
```

Back up `openwrt-usign.sec` and `openwrt-apk.pem` in an encrypted offline location.
The files under `release-keys/` are ignored by Git.

Encode the keys without printing private material into CI logs:

```sh
base64 -w0 release-keys/openwrt-usign.sec > release-keys/openwrt-usign.sec.b64
base64 -w0 release-keys/openwrt-apk.pem > release-keys/openwrt-apk.pem.b64
```

Create these GitHub Actions repository secrets from the corresponding `.b64` files:

| Secret | Source file |
| --- | --- |
| `OPENWRT_USIGN_SECRET_B64` | `openwrt-usign.sec.b64` |
| `OPENWRT_APK_SECRET_B64` | `openwrt-apk.pem.b64` |

The public keys are committed under `keys/openwrt/`. Their expected identifiers are:

- usign key ID: `7fb0908fb6bc82c8`;
- APK public key SHA256: `ce6f190c937961db306ddc3cbe138a877157d97252a2c8290980c43fc4f55f26`.

The release build verifies that each private key matches the committed public key before
publishing a signed feed.

An `agent-v*` release and the legacy publication workflow fail closed when a required
native signing key is missing. A manual run of the general **Build and test** workflow
may still create unsigned test artifacts, but it does not publish them. BuildKit secret
mounts expose private keys only to the repository-index build step; private keys are not
copied into images, artifacts or build cache.

## Support tiers and legacy packages

The normal `agent-v*` workflow builds ten current package targets. This keeps the release
gate fast and prevents an obsolete SDK from blocking packages for supported OpenWrt
versions.

To extend the latest agent release with signed OpenWrt 21.02, 22.03 and 23.05 packages,
run **Actions → Build legacy OpenWrt packages → Run workflow** from the default branch
and enter its existing tag, for example `agent-v0.6.7`. The same operation is available
through GitHub CLI:

```sh
gh workflow run build-legacy.yml -f agent_tag=agent-v0.6.7
```

Run it only after the main agent release has completed. The legacy workflow:

1. checks out and builds the exact agent tag;
2. signs the IPK repositories with the configured `usign` key;
3. downloads retained artifacts from the original tagged workflow and reconstructs the
   complete repository;
4. uploads only installable legacy packages to the existing GitHub Release;
5. redeploys the complete current plus legacy repository to GitHub Pages.

Run the legacy workflow within 90 days of the tagged build while its internal artifacts
are retained.

The workflow refuses an older agent tag because publishing it would roll the `stable`
feed back from the latest agent version.

The legacy matrix contains `x86/64`, `ramips/mt7621`, `ath79/generic` and
`ipq40xx/generic`; OpenWrt 23.05 also contains `mediatek/filogic`.
`bcm27xx/bcm2711` is intentionally on-demand and should be added only when a supported
Raspberry Pi 4 installation needs a package.

## OpenWrt 24.10 and older: IPK/opkg

Choose the directory matching the firmware release and target. For example MT7621 on
OpenWrt 24.10:

```sh
feed='https://packages.daemonlord.ru/feeds/stable/openwrt/24.10.7/ramips-mt7621'
key_base='https://packages.daemonlord.ru/keys/usign'
key_id='7fb0908fb6bc82c8'

wget -O "/etc/opkg/keys/${key_id}" "${key_base}/${key_id}"
printf 'src/gz rmm %s\n' "$feed" > /etc/opkg/customfeeds.conf.d/rmm.conf
opkg update
opkg install rmm-agent-go-production luci-app-rmm-agent
# Optional Russian LuCI translation:
opkg install luci-i18n-rmm-agent-ru
# After the LuCI release is imported into the shared feed:
opkg install luci-theme-rmm luci-app-rmm-dashboard
```

The workflow creates `Packages`, `Packages.gz` and `Packages.sig`. `opkg` verifies the
signature of the repository metadata and the package hashes contained in that metadata.
The key ID is the filename published under `/keys/usign/`.

## OpenWrt 25.12 and newer: APK

For MT7621 on OpenWrt 25.12:

```sh
base='https://packages.daemonlord.ru'
repo="${base}/feeds/stable/openwrt/25.12.4/ramips-mt7621/packages.adb"

wget -O /etc/apk/keys/rmm-openwrt.pem "${base}/keys/apk/rmm-openwrt.pem"
printf '%s\n' "$repo" > /etc/apk/repositories.d/rmm.list
apk update
apk add rmm-agent-go-production luci-app-rmm-agent
# Optional Russian LuCI translation:
apk add luci-i18n-rmm-agent-ru
# After the LuCI release is imported into the shared feed:
apk add luci-theme-rmm luci-app-rmm-dashboard
```

APK verifies the signed `packages.adb` index. Installation should not require
`--allow-untrusted`; needing that flag means the repository key or signature chain is
not configured correctly.

## Release verification

GitHub Releases contain only installable `.ipk` and `.apk` files. GitHub build-provenance
attestations remain available through the repository attestations page and GitHub CLI.
Package-manager installations rely on the signed feed: `opkg` verifies `Packages.sig`,
while APK verifies `packages.adb`. Public verification keys are served through GitHub
Pages and are not duplicated as release assets.

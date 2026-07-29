# Signed OpenWrt package repository

Agent release tags automatically publish the current OpenWrt support tier:

- OpenWrt 24.10 and 25.12;
- `x86/64`, `ramips/mt7621`, `ath79/generic`, `ipq40xx/generic` and
  `mediatek/filogic`.

OpenWrt 21.02, 22.03 and 23.05 packages are added to an existing agent release by the
manual legacy workflow described below. The default public base URL is:

```text
https://benya9669.github.io/openwrt-rmm/feeds/stable/openwrt
```

Before the first release, open **Repository settings → Pages** and select **GitHub
Actions** as the deployment source. A custom domain such as `packages.daemonlord.ru`
can be attached later in the same Pages settings; keep the GitHub Pages URL available
until DNS and TLS for the custom domain have been verified.

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
and enter its existing tag, for example `agent-v0.6.6`. The same operation is available
through GitHub CLI:

```sh
gh workflow run build-legacy.yml -f agent_tag=agent-v0.6.6
```

Run it only after the main agent release has completed. The legacy workflow:

1. checks out and builds the exact agent tag;
2. signs the IPK repositories with the configured `usign` key;
3. combines them with the current release and reconstructs the complete repository;
4. refreshes signed checksums and uploads the legacy files to the existing GitHub Release;
5. redeploys the complete current plus legacy repository to GitHub Pages.

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
feed='https://benya9669.github.io/openwrt-rmm/feeds/stable/openwrt/24.10.7/ramips-mt7621'
key_base='https://benya9669.github.io/openwrt-rmm/keys/usign'
key_id='7fb0908fb6bc82c8'

wget -O "/etc/opkg/keys/${key_id}" "${key_base}/${key_id}"
printf 'src/gz rmm %s\n' "$feed" > /etc/opkg/customfeeds.conf.d/rmm.conf
opkg update
opkg install rmm-agent-go-production luci-app-rmm-agent
```

The workflow creates `Packages`, `Packages.gz` and `Packages.sig`. `opkg` verifies the
signature of the repository metadata and the package hashes contained in that metadata.
The key ID is the filename published under `/keys/usign/`.

## OpenWrt 25.12 and newer: APK

For MT7621 on OpenWrt 25.12:

```sh
base='https://benya9669.github.io/openwrt-rmm'
repo="${base}/feeds/stable/openwrt/25.12.4/ramips-mt7621/packages.adb"

wget -O /etc/apk/keys/rmm-openwrt.pem "${base}/keys/apk/rmm-openwrt.pem"
printf '%s\n' "$repo" > /etc/apk/repositories.d/rmm.list
apk update
apk add rmm-agent-go-production luci-app-rmm-agent
```

APK verifies the signed `packages.adb` index. Installation should not require
`--allow-untrusted`; needing that flag means the repository key or signature chain is
not configured correctly.

## Release verification

Release assets also include a keyless Sigstore bundle for the combined checksums:

```sh
cosign verify-blob \
  --bundle SHA256SUMS.sigstore.json \
  --certificate-identity-regexp \
    '^https://github.com/Benya9669/openwrt-rmm/.github/workflows/(build\.yml@refs/tags/agent-v.*|build-legacy\.yml@refs/heads/main)$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  SHA256SUMS
sha256sum --check SHA256SUMS
```

This Sigstore verification complements native package-manager trust; it does not replace
the `usign` or APK repository signature.

# LuCI dashboard translation in the shared feed

LuCI releases may contain `luci-i18n-rmm-dashboard-zh-cn` and
`luci-i18n-rmm-dashboard-ru` alongside the theme and dashboard. The downloader
accepts four-, six- or eight-asset manifests (IPK and APK for each package), verifies the pinned
SHA256SUMS and requires exactly one of each package for the requested SDK.

The stable stager validates all inputs before copying. It replaces older
versions only of received packages and preserves archived feeds, agents,
other translations and the active translation when importing a two-package
release. Index generation signs the translation with the existing feed keys.
LuCI assets remain owned by the separate LuCI GitHub Release.

No configuration changes are needed. Install the translation using the
router's existing signed feed and package manager, then select Simplified
Chinese or Russian in LuCI. Verify the package appears in the signed package index.

Checks: `bash scripts/test-fetch-luci-release.sh` and
`bash scripts/test-stage-luci-stable-packages.sh`.

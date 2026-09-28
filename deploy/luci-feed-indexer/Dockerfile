# syntax=docker/dockerfile:1
# SPDX-License-Identifier: MIT
FROM debian:bookworm-slim AS indexer

ARG OPENWRT_SDK_URL
ARG OPENWRT_SDK_SHA256
ARG OPENWRT_RELEASE

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    binutils ca-certificates curl diffutils gawk gzip openssl tar zstd \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 1000 builder

USER builder
WORKDIR /home/builder
RUN test -n "$OPENWRT_SDK_URL" && test -n "$OPENWRT_SDK_SHA256" \
    && curl --fail --location --show-error --silent --retry 5 --retry-all-errors \
        --output sdk.tar.zst "$OPENWRT_SDK_URL" \
    && echo "$OPENWRT_SDK_SHA256  sdk.tar.zst" | sha256sum --check --strict \
    && mkdir sdk && tar --extract --zstd --file sdk.tar.zst --strip-components=1 --directory sdk \
    && rm sdk.tar.zst

COPY --chown=builder:builder keys/openwrt/ /home/builder/keys/
COPY --chown=builder:builder site/feeds/stable/openwrt/${OPENWRT_RELEASE}/ /home/builder/feed/

RUN --mount=type=secret,id=openwrt_usign_secret_b64,uid=1000,required=false \
    --mount=type=secret,id=openwrt_apk_secret_b64,uid=1000,required=false \
    set -eu; \
    sdk=/home/builder/sdk; \
    for feed in /home/builder/feed/*; do \
      test -d "$feed" || continue; \
      cd "$feed"; \
      if [ "${OPENWRT_RELEASE%%.*}" = 24 ]; then \
        test -s /run/secrets/openwrt_usign_secret_b64; \
        test -n "$(find . -maxdepth 1 -name 'luci-theme-rmm*.ipk' -print -quit)"; \
        test -n "$(find . -maxdepth 1 -name 'luci-app-rmm-dashboard*.ipk' -print -quit)"; \
        MKHASH="$sdk/staging_dir/host/bin/mkhash" "$sdk/scripts/ipkg-make-index.sh" . > Packages; \
        gzip -9nc Packages > Packages.gz; \
        base64 -d /run/secrets/openwrt_usign_secret_b64 > /tmp/usign.sec; \
        chmod 600 /tmp/usign.sec; \
        "$sdk/staging_dir/host/bin/usign" -S -m Packages -s /tmp/usign.sec -x Packages.sig; \
        "$sdk/staging_dir/host/bin/usign" -V -m Packages -x Packages.sig \
          -p /home/builder/keys/usign/7fb0908fb6bc82c8; \
        cp /home/builder/keys/usign/7fb0908fb6bc82c8 .; \
      else \
        test -s /run/secrets/openwrt_apk_secret_b64; \
        test -n "$(find . -maxdepth 1 -name 'luci-theme-rmm*.apk' -print -quit)"; \
        test -n "$(find . -maxdepth 1 -name 'luci-app-rmm-dashboard*.apk' -print -quit)"; \
        base64 -d /run/secrets/openwrt_apk_secret_b64 > /tmp/apk.pem; \
        chmod 600 /tmp/apk.pem; \
        "$sdk/staging_dir/host/bin/openssl" ec -in /tmp/apk.pem -pubout -out /tmp/apk.pub.pem; \
        cmp /tmp/apk.pub.pem /home/builder/keys/apk/rmm-openwrt.pem; \
        cp /home/builder/keys/apk/rmm-openwrt.pem .; \
        "$sdk/staging_dir/host/bin/apk" mkndx --root "$sdk" --keys-dir . \
          --allow-untrusted --sign /tmp/apk.pem --output packages.adb ./*.apk; \
      fi; \
      find . -maxdepth 1 -type f \( -name '*.ipk' -o -name '*.apk' \
        -o -name 'Packages' -o -name 'Packages.gz' -o -name 'Packages.sig' \
        -o -name 'packages.adb' -o -name 'rmm-openwrt.pem' \
        -o -name '????????????????' \) -print | LC_ALL=C sort | xargs sha256sum > SHA256SUMS; \
      test -s SHA256SUMS; \
    done; \
    rm -f /tmp/usign.sec /tmp/apk.pem /tmp/apk.pub.pem

FROM scratch AS artifacts
COPY --from=indexer /home/builder/feed/ /

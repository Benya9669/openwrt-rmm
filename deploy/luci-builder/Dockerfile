# syntax=docker/dockerfile:1
# SPDX-License-Identifier: MIT

FROM golang:1.26.5-bookworm AS go-agent

ARG RMM_GOARCH=mipsle
ARG RMM_GOARM=7
ARG RMM_GOMIPS=softfloat
ARG RMM_GOAMD64=v1

WORKDIR /src

COPY go.mod go.sum ./
COPY agent/go agent/go

RUN export CGO_ENABLED=0 GOOS=linux GOARCH="${RMM_GOARCH}" \
    && case "${RMM_GOARCH}" in \
        arm) export GOARM="${RMM_GOARM}" ;; \
        mips|mipsle) export GOMIPS="${RMM_GOMIPS}" ;; \
        amd64) export GOAMD64="${RMM_GOAMD64}" ;; \
       esac \
    && mkdir /out \
    && go build -trimpath -ldflags="-s -w" \
        -o /out/rmm-agent ./agent/go/cmd/rmm-agent

FROM debian:bookworm-slim AS sdk

ARG OPENWRT_SDK_URL="https://downloads.openwrt.org/releases/25.12.4/targets/ramips/mt7621/openwrt-sdk-25.12.4-ramips-mt7621_gcc-14.3.0_musl.Linux-x86_64.tar.zst"
ARG OPENWRT_SDK_SHA256="03f4766fcfbae86a814cbbf194226fa7e9ec66be2d4273bc1a0927a72a43a333"

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        file \
        gawk \
        gettext \
        git \
        libncurses-dev \
        libssl-dev \
        perl \
        python3 \
        python3-setuptools \
        rsync \
        unzip \
        zstd \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 1000 builder

USER builder
WORKDIR /home/builder

RUN case "${OPENWRT_SDK_URL}" in \
        *.tar.xz) sdk_archive="openwrt-sdk.tar.xz" ;; \
        *.tar.zst) sdk_archive="openwrt-sdk.tar.zst" ;; \
        *) echo "unsupported OpenWrt SDK URL: ${OPENWRT_SDK_URL}" >&2; exit 1 ;; \
       esac \
    && curl --fail --location --show-error --silent \
        --output "${sdk_archive}" "${OPENWRT_SDK_URL}" \
    && echo "${OPENWRT_SDK_SHA256}  ${sdk_archive}" | sha256sum --check --strict \
    && mkdir sdk \
    && case "${sdk_archive}" in \
        *.tar.xz) tar --extract --xz --file "${sdk_archive}" --strip-components=1 --directory sdk ;; \
        *.tar.zst) tar --extract --zstd --file "${sdk_archive}" --strip-components=1 --directory sdk ;; \
        *) echo "unsupported OpenWrt SDK archive: ${sdk_archive}" >&2; exit 1 ;; \
       esac \
    && rm "${sdk_archive}"

FROM sdk AS builder

ARG REQUIRE_NATIVE_SIGNATURE=false
ARG REPOSITORY_INDEX_REVISION=local

USER root

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends wget \
    && rm -rf /var/lib/apt/lists/*

USER builder
WORKDIR /home/builder/sdk

RUN ./scripts/feeds update -a \
    && for package_name in \
        ca-bundle \
        ip-tiny \
        iwinfo \
        luci-base \
        mbedtls \
        openssh-client \
        openssh-keygen \
        ubus \
        uclient-fetch; \
       do \
        ./scripts/feeds install "${package_name}"; \
       done

COPY --chown=builder:builder agent/package/rmm-agent package/rmm-agent
COPY --chown=builder:builder agent/package/rmm-agent-go-production package/rmm-agent-go-production
COPY --from=go-agent --chown=builder:builder /out/rmm-agent package/rmm-agent-go-production/files/usr/bin/rmm-agent

RUN printf '%s\n' \
        '# CONFIG_ALL is not set' \
        '# CONFIG_ALL_KMODS is not set' \
        '# CONFIG_ALL_NONSHARED is not set' \
        'CONFIG_PACKAGE_libmbedtls=y' \
        'CONFIG_PACKAGE_libustream-mbedtls=y' \
        'CONFIG_PACKAGE_rmm-agent=m' \
        'CONFIG_PACKAGE_rmm-agent-go-production=m' \
        > .config \
    && make defconfig \
    && awk ' \
        /^CONFIG_PACKAGE_[^=]+=[ym]$/ { \
            split($0, parts, "="); \
            print "# " parts[1] " is not set"; \
            next; \
        } \
        { print; } \
       ' .config > .config.packages-pruned \
    && mv .config.packages-pruned .config \
    && printf '%s\n' \
        'CONFIG_PACKAGE_libmbedtls=y' \
        'CONFIG_PACKAGE_libustream-mbedtls=y' \
        'CONFIG_PACKAGE_rmm-agent=m' \
        'CONFIG_PACKAGE_rmm-agent-go-production=m' \
        >> .config \
    && make defconfig

RUN make -j"$(nproc)" package/feeds/base/mbedtls/compile \
    && make -j"$(nproc)" package/feeds/base/ustream-ssl/compile

RUN make -j"$(nproc)" \
        package/rmm-agent/compile \
        package/rmm-agent-go-production/compile

COPY --chown=builder:builder agent/package/luci-app-rmm-agent package/luci-app-rmm-agent

RUN rm -rf tmp/info \
    && rm -f \
        tmp/.config-package.in \
        tmp/.packagedeps \
        tmp/.packageinfo

RUN printf '%s\n' 'CONFIG_PACKAGE_luci-app-rmm-agent=m' >> .config \
    && make defconfig

RUN make -j1 package/luci-app-rmm-agent/compile V=s

COPY --chown=builder:builder keys/openwrt /home/builder/release-keys

RUN --mount=type=secret,id=openwrt_usign_secret_b64,uid=1000,required=false \
    --mount=type=secret,id=openwrt_apk_secret_b64,uid=1000,required=false \
    test -n "${REPOSITORY_INDEX_REVISION}" \
    && mkdir -p /home/builder/artifacts \
    && find bin -type f \
        \( -name 'luci-app-rmm-agent*.apk' \
           -o -name 'luci-app-rmm-agent*.ipk' \
           -o -name 'rmm-agent*.apk' \
           -o -name 'rmm-agent*.ipk' \) \
        -exec cp '{}' /home/builder/artifacts/ \; \
    && test -n "$(find /home/builder/artifacts -maxdepth 1 -type f \( -name '*.apk' -o -name '*.ipk' \) -print -quit)" \
    && cd /home/builder/artifacts \
    && if find . -maxdepth 1 -type f -name '*.ipk' -print -quit | grep -q .; then \
        /home/builder/sdk/scripts/ipkg-make-index.sh . > Packages; \
        gzip -9nc Packages > Packages.gz; \
        if [ -s /run/secrets/openwrt_usign_secret_b64 ]; then \
          base64 -d /run/secrets/openwrt_usign_secret_b64 > /tmp/usign.sec; \
          chmod 600 /tmp/usign.sec; \
          /home/builder/sdk/staging_dir/host/bin/usign -S -m Packages -s /tmp/usign.sec; \
          /home/builder/sdk/staging_dir/host/bin/usign -V -m Packages \
            -p /home/builder/release-keys/usign/7fb0908fb6bc82c8; \
          cp /home/builder/release-keys/usign/7fb0908fb6bc82c8 ./; \
          rm -f /tmp/usign.sec; \
        elif [ "${REQUIRE_NATIVE_SIGNATURE}" = "true" ]; then \
          echo 'IPK release signing keys are required' >&2; \
          exit 1; \
        fi; \
      else \
        if [ -s /run/secrets/openwrt_apk_secret_b64 ]; then \
          base64 -d /run/secrets/openwrt_apk_secret_b64 > /tmp/apk-signing.pem; \
          chmod 600 /tmp/apk-signing.pem; \
          /home/builder/sdk/staging_dir/host/bin/openssl ec \
            -in /tmp/apk-signing.pem -pubout -out /tmp/apk-signing.pub.pem; \
          cmp /tmp/apk-signing.pub.pem /home/builder/release-keys/apk/rmm-openwrt.pem; \
          cp /home/builder/release-keys/apk/rmm-openwrt.pem ./; \
          /home/builder/sdk/staging_dir/host/bin/apk mkndx \
            --root /home/builder/sdk \
            --keys-dir /home/builder/artifacts \
            --allow-untrusted \
            --sign /tmp/apk-signing.pem \
            --output packages.adb \
            ./*.apk; \
          rm -f /tmp/apk-signing.pem /tmp/apk-signing.pub.pem; \
        elif [ "${REQUIRE_NATIVE_SIGNATURE}" = "true" ]; then \
          echo 'APK release signing key is required' >&2; \
          exit 1; \
        else \
          /home/builder/sdk/staging_dir/host/bin/apk mkndx \
            --root /home/builder/sdk \
            --keys-dir /home/builder/artifacts \
            --allow-untrusted \
            --output packages.adb \
            ./*.apk; \
        fi; \
      fi \
    && find . -maxdepth 1 -type f \( \
        -name '*.apk' -o -name '*.ipk' \
        -o -name 'Packages' -o -name 'Packages.gz' -o -name 'Packages.sig' \
        -o -name 'packages.adb' -o -name 'rmm-openwrt.pem' \
        -o -name '????????????????' \) -print \
        | LC_ALL=C sort \
        | xargs sha256sum > SHA256SUMS

FROM scratch AS artifacts

COPY --from=builder /home/builder/artifacts/ /

# SPDX-License-Identifier: AGPL-3.0-only
FROM golang:1.26.5-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY server ./server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/rmm-server ./server/cmd/rmm-server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates su-exec tzdata

WORKDIR /app
COPY --from=build /out/rmm-server /usr/local/bin/rmm-server
COPY web ./web
COPY deploy/server/entrypoint.sh /usr/local/bin/rmm-entrypoint

RUN addgroup -S rmm && adduser -S -G rmm rmm \
  && mkdir -p /data \
  && chown -R rmm:rmm /data /app \
  && chmod 0755 /usr/local/bin/rmm-entrypoint

ENV RMM_ADDR=:8080
ENV RMM_DB_PATH=/data/rmm.db
ENV RMM_WEB_DIR=/app/web

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/rmm-entrypoint"]
CMD ["/usr/local/bin/rmm-server"]

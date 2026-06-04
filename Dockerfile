FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY server ./server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/rmm-server ./server/cmd/rmm-server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=build /out/rmm-server /usr/local/bin/rmm-server
COPY web ./web

RUN mkdir -p /data

ENV RMM_ADDR=:8080
ENV RMM_DB_PATH=/data/rmm.db
ENV RMM_WEB_DIR=/app/web

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/rmm-server"]

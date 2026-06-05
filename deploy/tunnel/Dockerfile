FROM alpine:3.21

RUN apk add --no-cache openssh-server netcat-openbsd

COPY sshd_config /etc/ssh/sshd_config
COPY entrypoint.sh /usr/local/bin/tunnel-entrypoint
RUN chmod 0755 /usr/local/bin/tunnel-entrypoint

EXPOSE 22 22000-22099
VOLUME ["/data"]

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
  CMD nc -z 127.0.0.1 22 || exit 1

ENTRYPOINT ["/usr/local/bin/tunnel-entrypoint"]

FROM alpine:3.23

RUN apk add --no-cache openssl
COPY init.sh /usr/local/bin/rmm-postgres-tls-init
RUN chmod 0755 /usr/local/bin/rmm-postgres-tls-init

ENTRYPOINT ["/usr/local/bin/rmm-postgres-tls-init"]

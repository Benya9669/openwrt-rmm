#!/bin/sh
set -eu

# Keep the CA signing key in a volume mounted only by this one-shot container.
if [ ! -d /ca/current ]; then
	stage="$(mktemp -d /ca/.stage.XXXXXX)"
	chmod 0700 "$stage"
	openssl req -x509 -newkey rsa:3072 -noenc -sha256 -days 3650 \
		-subj '/CN=RMM PostgreSQL CA' \
		-addext 'basicConstraints=critical,CA:TRUE' \
		-addext 'keyUsage=critical,keyCertSign,cRLSign' \
		-keyout "$stage/ca.key" -out "$stage/ca.crt" >/dev/null 2>&1
	chmod 0600 "$stage/ca.key"
	chmod 0644 "$stage/ca.crt"
	mv "$stage" /ca/current
fi

if [ ! -s /ca/current/ca.key ] || [ ! -s /ca/current/ca.crt ]; then
	echo 'PostgreSQL CA is incomplete' >&2
	exit 1
fi

if [ ! -d /server/current ]; then
	stage="$(mktemp -d /server/.stage.XXXXXX)"
	chmod 0700 "$stage"
	openssl req -new -newkey rsa:3072 -noenc -sha256 \
		-subj '/CN=postgres' -addext 'subjectAltName=DNS:postgres' \
		-keyout "$stage/server.key" -out "$stage/server.csr" >/dev/null 2>&1
	printf '%s\n' 'basicConstraints=critical,CA:FALSE' 'keyUsage=critical,digitalSignature,keyEncipherment' \
		'extendedKeyUsage=serverAuth' 'subjectAltName=DNS:postgres' > "$stage/server.ext"
	openssl x509 -req -in "$stage/server.csr" \
		-CA /ca/current/ca.crt -CAkey /ca/current/ca.key \
		-CAcreateserial -days 3650 -sha256 -extfile "$stage/server.ext" \
		-out "$stage/server.crt" >/dev/null 2>&1
	chmod 0600 "$stage/server.key"
	chmod 0644 "$stage/server.crt"
	# The pinned postgres:18-alpine image runs PostgreSQL as UID/GID 70.
	chown 70:70 "$stage" "$stage/server.key" "$stage/server.crt"
	mv "$stage" /server/current
fi

if [ ! -s /server/current/server.key ] || [ ! -s /server/current/server.crt ]; then
	echo 'PostgreSQL server certificate is incomplete' >&2
	exit 1
fi
openssl verify -CAfile /ca/current/ca.crt -verify_hostname postgres \
	/server/current/server.crt >/dev/null
openssl x509 -in /server/current/server.crt -checkend 0 -noout >/dev/null

cp /ca/current/ca.crt /client/ca.crt.tmp
chmod 0644 /client/ca.crt.tmp
mv /client/ca.crt.tmp /client/ca.crt

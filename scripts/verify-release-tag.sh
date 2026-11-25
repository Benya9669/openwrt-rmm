#!/usr/bin/env bash
set -euo pipefail

tag=${1:-}
if [[ $# != 1 || ! "$tag" =~ ^(agent|server)-v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Expected one agent-vX.Y.Z or server-vX.Y.Z release tag" >&2
  exit 1
fi
for tool in git gpg awk mktemp; do
  command -v "$tool" >/dev/null || { echo "$tool is required" >&2; exit 1; }
done
: "${RELEASE_OPENPGP_PUBLIC_KEYS:?Configure the public release key bundle}"
: "${RELEASE_OPENPGP_FINGERPRINTS:?Configure the allowed primary fingerprints}"
if [[ "$RELEASE_OPENPGP_PUBLIC_KEYS" != *"-----BEGIN PGP PUBLIC KEY BLOCK-----"* ||
      "$RELEASE_OPENPGP_PUBLIC_KEYS" == *"PRIVATE KEY BLOCK"* ]]; then
  echo "Expected an armored public key export" >&2
  exit 1
fi
ref="refs/tags/$tag"
if [[ "$(git cat-file -t "$ref")" != tag ||
      "$(git for-each-ref --format='%(tag)' "$ref")" != "$tag" ||
      "$(git rev-parse "$ref^{commit}")" != "$(git rev-parse HEAD)" ]]; then
  echo "Release requires its named, annotated tag on the checked-out commit" >&2
  exit 1
fi

allowed=()
while IFS= read -r fingerprint; do
  fingerprint=${fingerprint%$'\r'}
  [[ -n "$fingerprint" ]] || continue
  if [[ ! "$fingerprint" =~ ^([[:xdigit:]]{40}|[[:xdigit:]]{64})$ ]]; then
    echo "Expected one full primary key fingerprint per line" >&2
    exit 1
  fi
  allowed+=("${fingerprint^^}")
done <<< "$RELEASE_OPENPGP_FINGERPRINTS"

umask 077
GNUPGHOME=$(mktemp -d)
export GNUPGHOME
trap 'rm -rf -- "$GNUPGHOME"' EXIT
printf '%s\n' "$RELEASE_OPENPGP_PUBLIC_KEYS" | gpg --batch --quiet --import
status=$(git -c gpg.format=openpgp -c gpg.openpgp.program=gpg verify-tag --raw "$ref" 2>&1) || {
  echo "OpenPGP tag signature verification failed" >&2
  exit 1
}
# Field 12 identifies the primary key even when a signing subkey made the signature.
signer=$(awk '$1 == "[GNUPG:]" && $2 == "VALIDSIG" { print $12 }' <<< "$status")
if ! awk '$1 == "[GNUPG:]" && $2 == "GOODSIG" { good++ }
          $1 == "[GNUPG:]" && $2 ~ /^(BADSIG|ERRSIG|EXPSIG|EXPKEYSIG|REVKEYSIG|KEYEXPIRED|SIGEXPIRED)$/ { bad++ }
          END { exit !(good == 1 && !bad) }' <<< "$status"; then
  echo "Release requires one valid signature by a current key" >&2
  exit 1
fi
for fingerprint in "${allowed[@]}"; do
  if [[ "$fingerprint" == "$signer" ]]; then
    printf 'Verified %s with authorized primary fingerprint %s\n' "$tag" "$signer"
    exit 0
  fi
done
echo "Release tag signer is outside the configured allowlist" >&2
exit 1

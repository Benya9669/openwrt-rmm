#!/usr/bin/env bash
set -euo pipefail

verify="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/verify-release-tag.sh"
umask 077
fixture=$(mktemp -d)
trap 'gpgconf --kill gpg-agent >/dev/null 2>&1 || true; rm -rf -- "$fixture"' EXIT
export GNUPGHOME="$fixture/gnupg" GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
mkdir "$GNUPGHOME"
git init --quiet "$fixture/project"
cd "$fixture/project"
git config user.name 'Release fixture'
git config user.email fixture@example.invalid
git config gpg.openpgp.program gpg
tree=$(git mktree </dev/null)
git update-ref HEAD "$(printf '%s\n' fixture | git commit-tree "$tree")"

gpg --batch --pinentry-mode loopback --passphrase '' --quick-generate-key \
  'Release test <fixture@example.invalid>' ed25519 cert 1d >/dev/null 2>&1
fingerprint=$(gpg --with-colons --list-keys | awk -F: '$1 == "fpr" { print $10; exit }')
gpg --batch --pinentry-mode loopback --passphrase '' --quick-add-key "$fingerprint" ed25519 sign 1d
git -c user.signingkey="$fingerprint" tag -s -m fixture agent-v1.0.0
RELEASE_OPENPGP_PUBLIC_KEYS=$(gpg --armor --export "$fingerprint")
export RELEASE_OPENPGP_PUBLIC_KEYS RELEASE_OPENPGP_FINGERPRINTS="$fingerprint"
bash "$verify" agent-v1.0.0

expect_failure() {
  if bash "$verify" "$1" >/dev/null 2>&1; then
    echo "Unexpected verification success for $1" >&2
    exit 1
  fi
}
RELEASE_OPENPGP_FINGERPRINTS=0000000000000000000000000000000000000000 expect_failure agent-v1.0.0
RELEASE_OPENPGP_FINGERPRINTS='' expect_failure agent-v1.0.0
git tag server-v1.0.0
expect_failure server-v1.0.0
git tag -a -m 'Unsigned release' server-v1.0.1
expect_failure server-v1.0.1
git update-ref refs/tags/agent-v1.0.1 "$(git rev-parse refs/tags/agent-v1.0.0)"
expect_failure agent-v1.0.1
git update-ref HEAD "$(printf '%s\n' next | git commit-tree "$tree" -p HEAD)"
expect_failure agent-v1.0.0
echo 'OpenPGP release tag checks passed'

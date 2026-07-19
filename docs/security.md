# Security Notes

Every enrolled router is a privileged remote-execution target. Use HTTPS, protect the
SQLite database and tunnel private key, and keep the control plane restricted to trusted
operators.

## Trust model and current controls

- Browser users authenticate with Argon2id password hashes and random, revocable,
  server-side `HttpOnly`, `SameSite=Strict` sessions.
- The bootstrap bearer token is optional and administrator-scoped.
- Normal users can list and control only devices enrolled by their own one-time grants;
  device-object and admin-function authorization are enforced server-side.
- Device, enrollment, operator-session and LuCI access tokens are stored as SHA-256
  fingerprints, not reusable plaintext credentials.
- Enrollment grants expire, are single-use and are limited per account. Shared enrollment
  exists only as an explicit compatibility switch.
- LuCI runs on a separate device subdomain. A 60-second grant creates a host-only access
  session scoped to one user, device and active reverse tunnel. Upstream router cookies
  cannot set parent domains or overwrite RMM cookie names.
- Request bodies and agent responses are capped at 2 MiB; login attempts are rate- and
  concurrency-limited; HTTP server timeouts and header limits are set.
- Agent command types and UCI packages are allowlisted. Stored command output redacts UCI
  and key/value secret forms.
- SSH tunnel accounts allow remote forwarding but no shell, PTY, agent forwarding, tunnel
  device, stream-local forwarding or SFTP sessions.
- Metric history defaults to 30-day retention. Expired authentication/access rows are
  removed by scheduled maintenance.

## Operational guidance

- Keep `RMM_INSECURE_DEV_MODE`, `RMM_ALLOW_LEGACY_ENROLLMENT`, and
  `RMM_ALLOW_LEGACY_LUCI_PROXY` disabled in production.
- Keep `RMM_COOKIE_SECURE=true`, use separate control and wildcard device hostnames, and
  obtain the wildcard certificate through DNS-01. See [keendns.md](keendns.md).
- Expose port `2222` only to managed routers. Operator tunnel ports bind to loopback by
  default; expose them only through a VPN or a narrowly scoped firewall rule.
- Prefer `uci_preview`, `uci_backup`, and `uci_commit_confirmed` for network changes.
- Back up SQLite without deleting or recreating the live volume.

## Remaining hardening work

- Issue a distinct SSH key or short-lived SSH certificate per device instead of sharing one
  persistent tunnel key.
- Add device-token rotation/revocation and optional mTLS or signed requests.
- Add command approval policies for reboot, package changes, restore, UCI commit and remote
  access.
- Add organization-level tenancy, recovery codes/MFA, user self-service password changes,
  and external identity-provider integration before offering the service as public SaaS.
- Add distributed rate limiting when running more than one server instance.
- Add an automated SBOM/dependency scan and build signed, immutable container/package
  artifacts in CI.

# Security Notes

This project is still an MVP. Treat every enrolled router as a privileged remote execution target and run the server only on trusted networks until the remaining hardening work is complete.

## Trust Model

- The server is the control plane.
- The OpenWrt agent is a privileged device-side worker.
- Browser operators authenticate with a username/password and a signed `HttpOnly` session cookie.
- API automation can authenticate with `RMM_OPERATOR_TOKEN`.
- Devices authenticate with a per-device bearer token issued during enrollment.
- Enrollment currently uses a shared enrollment token and should be considered bootstrap-only.

## Current Controls

- Agent traffic is outbound polling, so routers do not need inbound firewall openings.
- Devices receive unique bearer tokens after enrollment.
- Operator APIs accept a signed browser session or an operator bearer token.
- Server and agent both enforce command allowlists.
- UCI operations are limited to allowlisted config packages: `network`, `wireless`, `dhcp`, `firewall`, and `system`.
- Command output is redacted for sensitive keys before storage and display.
- Command lifecycle has retries, expiry, cancellation, and result rejection for cancelled or expired commands.
- UCI preview can show a diff without leaving staged changes.
- UCI commit-confirmed can restore the latest local backup if the router loses server reachability after commit.
- Basic audit events are recorded for operator command creation and cancellation.

## MVP Risks

- Tokens are stored in SQLite as bearer credentials and are not yet hashed.
- The enrollment token is shared and long-lived unless operators rotate it manually.
- There is no RBAC, organization isolation, or per-command approval policy.
- There are no rate limits or replay protection.
- Commands are not signed.
- TLS termination is not handled by the application. Use a reverse proxy with HTTPS when exposing the server beyond localhost or a lab LAN.

## Operational Guidance

- Run the server behind HTTPS and restrict inbound access to trusted operator networks.
- Use a high-entropy `RMM_OPERATOR_TOKEN` and enrollment token.
- Use a high-entropy operator password and `RMM_SESSION_SECRET`.
- Set `RMM_COOKIE_SECURE=true` behind HTTPS.
- Rotate the enrollment token after initial device onboarding.
- Keep the SQLite database private and backed up.
- Prefer `uci_preview` and `uci_backup` before `uci_set` or `uci_commit`.
- Use `uci_commit_confirmed` for network changes that can break connectivity.
- Review command history and audit events after configuration changes.

## Next Hardening Work

- Hash device tokens in storage.
- Add device token rotation and revocation.
- Replace shared enrollment with single-use enrollment grants.
- Add request IDs and structured logs.
- Add rate limits for enrollment, heartbeat, command result, and operator APIs.
- Add replay protection or signed device requests.
- Add command approval policies for reboot, UCI commit, restore, package install, and remote access.
- Add RBAC for operators.
- Add organization or fleet isolation before multi-tenant use.
- Add mTLS or signed device tokens for production fleets.

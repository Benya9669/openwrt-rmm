# OpenWrt RMM Web UI

Zero-dependency MVP web UI served by the Go server.

## Run

```sh
go run ./server/cmd/rmm-server
```

Then open:

```text
http://127.0.0.1:8080/       public landing page
http://127.0.0.1:8080/login  sign in and password recovery
http://127.0.0.1:8080/app    authenticated RMM cabinet
http://127.0.0.1:8080/legal.html  license notices and source offer
```

The UI uses a revocable HttpOnly server-side session. It does not store operator tokens in
browser storage. Normal users see only their own routers and can create one-time enrollment
grants; administrators can create user accounts.

The server and web interface are licensed under `AGPL-3.0-only`. Keep the source-code
link on `legal.html` available to users of a deployed modified version and point it to the
corresponding source for that deployment.

Current MVP features:

- device list and detail;
- command history and command detail output;
- command status filters;
- output copy;
- UCI backup, preview, staged set, commit, confirmed commit, revert, and restore;
- UCI presets for LAN IP, hostname, Wi-Fi SSID/password, and LAN DHCP toggle.
- isolated LuCI access through one-time device-domain links;
- per-user router enrollment and administrator-created accounts;
- responsive desktop/mobile navigation and account panel;
- live device refresh over authenticated Server-Sent Events (SSE), with 30-second polling fallback and refresh when the tab becomes visible;
- friendly LuCI access states for expired, unavailable, and timed-out sessions;
- separate online, recently seen, and DHCP-only client presence states.
- separate account tabs for profile, security, notifications, and administrator users;
- notification queue metrics, channel diagnostics, and filtered delivery history.

## Check

```sh
npm.cmd run check:web
```

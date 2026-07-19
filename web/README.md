# OpenWrt RMM Web UI

Zero-dependency MVP web UI served by the Go server.

## Run

```sh
go run ./server/cmd/rmm-server
```

Then open:

```text
http://127.0.0.1:8080/
```

The UI uses a revocable HttpOnly server-side session. It does not store operator tokens in
browser storage. Normal users see only their own routers and can create one-time enrollment
grants; administrators can create user accounts.

Current MVP features:

- device list and detail;
- command history and command detail output;
- command status filters;
- output copy;
- UCI backup, preview, staged set, commit, confirmed commit, revert, and restore;
- UCI presets for LAN IP, hostname, Wi-Fi SSID/password, and LAN DHCP toggle.
- isolated LuCI access through one-time device-domain links;
- per-user router enrollment and administrator-created accounts.

## Check

```sh
npm.cmd run check:web
```

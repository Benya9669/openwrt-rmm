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

The UI uses the same origin as the API and stores the MVP operator token in browser local storage.

Current MVP features:

- device list and detail;
- command history and command detail output;
- command status filters;
- output copy;
- UCI backup, preview, staged set, commit, confirmed commit, revert, and restore;
- UCI presets for LAN IP, hostname, Wi-Fi SSID/password, and LAN DHCP toggle.

## Check

```sh
npm.cmd run check:web
```

# Entangled VPN — Electron client

Linux-first Electron shell around the existing Go `vpncore`. The classic **Wails** Windows client under `client/` is unchanged.

## Layout

| Path | Role |
|------|------|
| `electron/main.mjs` | Electron main — spawns Go sidecar, hosts UI |
| `electron/preload.cjs` | Exposes `window.go.main.App` + `window.runtime` (Wails-compatible) |
| `electron/sidecar/` | Go process: same App API over NDJSON stdin/stdout |
| `electron/ui-dist/` | Vite build of `client/frontend` with `base: ./` (does not overwrite Wails `dist`) |

## Requirements

- Go 1.23+
- Node 20+
- Linux: permission to create a TUN (`CAP_NET_ADMIN` or run with sudo). Without it, connect/signaling works but joining a room fails when creating `entangled`.

```bash
# optional: grant TUN without full root
sudo setcap cap_net_admin,cap_net_raw+ep electron/sidecar/entangled-sidecar
```

## Package / install (Linux)

```bash
cd electron
npm install
npm run pack:linux      # builds AppImage + unpacked dir
npm run install:linux   # installs to ~/.local and adds app menu entry
```

Then run `entangledvpn` or open **Entangled VPN** from your app menu.

Artifacts:
- `electron/dist-electron/EntangledVPN-1.3.1-x86_64.AppImage`
- `electron/dist-electron/linux-unpacked/`

TUN needs `CAP_NET_ADMIN` (install script prompts via pkexec). AppImage copies the sidecar into user data on first run and may ask again for setcap.

## Package

```bash
npm run pack:linux     # AppImage + unpacked dir
npm run pack:win       # optional Windows Electron build (mood)
```

Windows Electron still uses Wintun via `vpncore` and needs Administrator for TUN — same as the Wails app.

## Protocol

Sidecar NDJSON:

```json
{"id":1,"method":"Connect","params":["host:8080","nick"]}
{"id":1,"result":{"connected":true,...}}
{"event":"status_changed","data":{...}}
```

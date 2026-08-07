# AGENTS.md — Entangled VPN (project facts)

## Paths

- Git root: this directory (`EntangledApp`)
- Server: `server/` (Go signaling + UDP relay)
- Client: `client/` (Wails v2 + `vpncore` + Svelte UI in `client/frontend/`)

## Stack

- Go 1.22+ (see `server/go.mod`, `client/go.mod`)
- Windows client: Wails v2 + Wintun; build with Admin for TUN
- Frontend: Svelte, Node 18+

## Build / test

```bash
cd server && go test ./...
cd client && go test ./vpncore/...
cd client/frontend && npm ci && npm run build
cd client && wails build
```

## Protocol notes

- Session keys: X25519 + HKDF-only (`hkdf-v1`) + XChaCha20-Poly1305 (random nonces)
- Relay REG is token-only when `ENTANGLED_TOKEN` is set
- `AppVersion` lives in `client/vpncore/version.go`

## Do not commit

Secrets, SSH keys, real deploy hosts, `rooms.json`, or private deploy scripts. Use `scripts/deploy.example.sh`.

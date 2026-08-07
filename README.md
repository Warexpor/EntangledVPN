# Entangled

Mesh VPN for small friend groups: create a virtual LAN (`10.242.0.0/24`), chat, and game over P2P UDP with relay fallback.

**Client:** Windows (Wails + Wintun). **Server:** Linux/Windows Go binary (WebSocket signaling + UDP relay).

## Threat model (short)

- Room traffic between peers is encrypted with X25519 + **HKDF-only** (`hkdf-v1`) + XChaCha20-Poly1305. Legacy raw-X25519 session keys are gone — **every friend must rebuild** on a matching client or crypto will fail.
- The signaling/relay server sees metadata (who joins which room, public keys, addresses) and can MITM key exchange if you do not trust the operator. Host your own server with friends you trust.
- Put the HTTP WebSocket behind TLS (`wss://`) in production. The server itself listens in cleartext for use behind Caddy/nginx.
- Optional `ENTANGLED_TOKEN` shared secret gates server auth **and** UDP relay registration (token-only REG; no password-based relay enroll). Empty token = open server (fine for a private VPS among friends).
- Saved networks never store room passwords on disk (`rooms.json` has name/server only). Join still needs the invite password when required.
- If this repository (or a fork) ever contained deploy SSH keys, **rotate those keys** and scrub git history before publishing.

## Quick start — server

```bash
cd server
go build -o entangled-server .
./entangled-server -addr :8080 -relay :3478
```

Optional:

```bash
export ENTANGLED_TOKEN='your-shared-secret'
./entangled-server
```

Health: `curl http://127.0.0.1:8080/health`

### TLS with Caddy (recommended)

```
your.domain {
    reverse_proxy localhost:8080
}
```

Clients connect to `wss://your.domain` (or `your.domain:443`). UDP relay still needs port **3478/UDP** open to the server host.

Firewall: **8080/TCP** (or 443 via proxy), **3478/UDP**.

Example deploy script (no secrets): [scripts/deploy.example.sh](scripts/deploy.example.sh).

## Quick start — Windows client

Requirements: Go 1.22+, Node 18+, [Wails v2](https://wails.io/), Administrator for Wintun.

**Breaking (1.1.0):** HKDF-only crypto + token-only relay REG. Old 1.0.0 clients cannot talk to 1.1.0 peers — all friends must rebuild. Deploy a matching server too.

```bash
cd client/frontend && npm ci && npm run build && cd ../..
cd client
wails build
```

Run `build/bin/Entangled.exe` as Administrator. Enter server `host:8080` (or `wss://host`), nickname, connect, create/join a network.

Invite friends with **Copy invite** — pipe format `server|room|password` (password segment may be empty for open rooms). Paste invite in the join dialog. Saved networks do **not** keep passwords on disk; re-enter when joining a protected room.

## Features

- Virtual IP mesh + peer chat (DM + room)
- Auto-reconnect with room re-join
- P2P / relay / WebSocket path indicator
- Saved networks, invite paste, copy VIP
- Room owner can delete the network
- i18n: English, Russian, Chinese

## Development

```bash
# server tests
cd server && go test ./...

# client vpncore tests
cd client && go test ./vpncore/...

# frontend
cd client/frontend && npm ci && npm run build
```

## Layout

| Path | Role |
|------|------|
| `server/` | Signaling hub + UDP relay |
| `client/` | Wails app + `vpncore` |
| `client/frontend/` | Svelte UI |
| `scripts/deploy.example.sh` | Secret-free deploy template |

## License

MIT — see [LICENSE](LICENSE).

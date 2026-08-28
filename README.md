# Entangled VPN

**v1.3.0** — Mesh VPN for small friend groups.

Create a virtual LAN (`10.242.0.0/24`), chat, and play over P2P UDP with automatic relay fallback when NAT wins. Settings → **Connection**: Direct (default) or Relay (force server path).

| | |
|---|---|
| **Client** | Windows — Wails + Wintun (run as Administrator) |
| **Server** | Linux or Windows — Go binary (WebSocket signaling + UDP relay) |
| **Crypto** | X25519 + HKDF-only (`hkdf-v1`) + XChaCha20-Poly1305 |
| **License** | [Apache-2.0](LICENSE) · [NOTICE](NOTICE) |

[Changelog](CHANGELOG.md) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md)

> **Breaking in 1.1.0:** HKDF-only session keys + token-only relay REG. Every peer (and the server) must be on 1.1.0 — old 1.0.0 clients will not interoperate.

## Features

- Virtual IP mesh with room chat and DMs
- P2P / relay / WebSocket path indicator
- Connection mode: Direct (prefer P2P) or Relay (server-only)
- Auto-reconnect with room re-join
- Saved networks + pipe invites (`server|room|password`)
- Room owner can delete the network (capability `owner_token`)
- In-app Check for updates / Update (Windows client)
- Optional shared `ENTANGLED_TOKEN` for server + relay auth
- UI languages: English, Russian, Chinese

## Quick start — server

```bash
cd server
go build -o entangled-server .
./entangled-server -addr :8080 -relay :3478
```

Optional shared secret (recommended on a public VPS):

```bash
export ENTANGLED_TOKEN='your-shared-secret'
./entangled-server
```

Health check: `curl http://127.0.0.1:8080/health`

Cross-compile Linux amd64 from elsewhere:

```bash
cd server
GOOS=linux GOARCH=amd64 go build -o entangled-server-linux .
```

### TLS (recommended)

Put the WebSocket behind Caddy/nginx. The Go process listens in cleartext for that proxy.

```
your.domain {
    reverse_proxy localhost:8080
}
```

Clients use `wss://your.domain`. Open **3478/UDP** on the host for the relay (TCP 443 via the proxy is enough for signaling).

Secret-free deploy template: [scripts/deploy.example.sh](scripts/deploy.example.sh).

## Quick start — Windows client

Needs: Go 1.22+, Node 18+, [Wails v2](https://wails.io/), Administrator privileges for Wintun.

```bash
cd client/frontend && npm ci && npm run build && cd ../..
cd client
wails build
```

Run `build/bin/Entangled.exe` **as Administrator**.

1. Enter server (`host:8080` or `wss://host`), nickname, and token if the server requires one.
2. Create or join a network.
3. Share **Copy invite** — format `server|room|password` (password may be empty for open rooms).

Saved networks store name/server only — **not** room passwords. Re-enter the password when joining a protected room.

## Threat model (short)

- Peer traffic is E2E-encrypted (X25519 → HKDF → XChaCha20-Poly1305). Packet loss does not desync nonces (random nonces per packet).
- The signaling/relay server sees metadata (who joins which room, public keys, addresses) and **can MITM key exchange** if you do not trust the operator. Self-host with people you trust.
- Empty `ENTANGLED_TOKEN` = open server (fine for a private friend VPS). Set a token for anything reachable from the wider internet.
- Room passwords are never written to `rooms.json`.

## Development

```bash
cd server && go test ./...
cd client && go test ./vpncore/...
cd client/frontend && npm ci && npm run build
```

App version constant: `client/vpncore/version.go` (`AppVersion`).

## Layout

| Path | Role |
|------|------|
| `server/` | Signaling hub + UDP relay |
| `client/` | Wails app + `vpncore` |
| `client/frontend/` | Svelte UI |
| `scripts/` | Build / deploy helpers (no secrets) |

## Releases

**Windows client:** download **`Entangled.exe`** from the [GitHub Release](https://github.com/Warexpor/EntangledVPN/releases) and run it **as Administrator**. Wintun is embedded — the first launch writes `wintun.dll` next to the exe (leave that file there).

Server binaries (`entangled-server-linux-amd64`, `entangled-server-windows-amd64.exe`) are on the same Release.

## Credits

- **[ProGaMEr110521](https://github.com/ProGaMEr110521)** — initiator; main developer of the first working build
- **[Warexpor](https://github.com/Warexpor)** — polish, expansion, and maintenance

## License

Apache License 2.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE).
Wintun (`wintun.dll`) is third-party; see NOTICE for attribution.

# Changelog

## Unreleased

## 1.3.1 — 2026-08-29

### Fixed

- Failed or timed-out signaling auth left a live WebSocket; Connect niled `a.vpn` without `Stop()`, so retries stacked zombie sessions; `reconnectLoop` leaked the same way. `Start`/`Connect`/`reconnectLoop` now close that attempt

## 1.3.0 — 2026-08-28

### Added

- Connected-shell disconnect control, reconnect guidance, localized confirmation dialogs, and improved keyboard/focus support
- Immediate UI-scale and theme persistence from Settings

### Changed

- Refined the connected UI with responsive layouts, clearer status colors, reduced-motion support, and a quieter peer roster
- Locked saved networks now return to the password flow instead of attempting an empty-password join

### Fixed

- Connection readiness and room operations now report authenticated signaling and actual server results
- Reconnect, deliberate disconnect, chat-thread switching, UDP shutdown, and DNS reset lifecycle behavior

## 1.2.8 — 2026-08-10

### Changed

- Settings: replace **P2P Only** with **Connection** mode — **Direct (P2P)** (default, prefers peer-to-peer, relay if needed) or **Relay (server)** (force all traffic through the relay)

## 1.2.7 — 2026-08-10

### Fixed

- Path column shows **Direct** when the ping sample’s pong arrives over P2P (~30ms mixed-path RTT); app data still uses relay until the peer echoes that our ping also landed on P2P (avoids the false-Direct chat break)

## 1.2.6 — 2026-08-10

### Fixed

- Do not mark path **Direct** from a P2P-delivered pong alone — that only proves peer→you. Pong now echoes how the ping arrived; Direct requires the ping landed on P2P too (fixes one-way chat after a false Direct upgrade)

## 1.2.5 — 2026-08-10

### Fixed

- Stop dual-sending chat/data over P2P+relay (doubled messages); hole-punch probes only
- Do not downgrade a proven **Direct** path when a slower relay copy of the same pong arrives
- Deduplicate identical datagrams received on both paths within 2s

## 1.2.4 — 2026-08-10

### Fixed

- **P2P Only** no longer skips relay before a pong proves the direct path — fixes one-way chat/ping when one peer has symmetric/CGNAT (STUN port ≠ send map)

## 1.2.3 — 2026-08-10

### Fixed

- P2P sends prefer the learned live UDP source (NAT-mapped port), not only advertised candidates — fixes one-way ping / blank latency with **P2P Only**

## 1.2.2 — 2026-08-10

### Fixed

- STUN tries Cloudflare then Google (Google `:19302` is blocked on some networks); empty external killed real P2P hole punch
- `p2p-only` no longer errors on successful UDP send before a pong proves the path (1.2.1 regression)
- Hole-punch probes both public and local candidate addresses

## 1.2.1 — 2026-08-10

### Fixed

- P2P send no longer XOR-returns on bare `WriteToUDP` success before a pong proves the path; unproven/relay-proven peers keep using relay (hole-punch still attempted)
- Join-time `peer_updated` for existing peers includes `crypto` when known

## 1.2.0 — 2026-08-09

### Added

- Settings: Check for updates / Update downloads `Entangled.exe` from GitHub Releases, swaps after exit, and relaunches

### Security

- Room ownership uses an opaque `owner_token` (not a forgeable peer public key); delete/reclaim require the token
- Sensitive WebSocket handlers require auth; `peer_info` binds pubkey once; `relay_data` same-room only

### Docs

- README connect-screen showcase image; credit ProGaMEr110521 as initiator / first working build (Warexpor: polish, expansion, maintenance); NOTICE authorship note

### Fixed

- Signaling address tests use TEST-NET instead of a real host IP

## 1.1.0 — 2026-08-07

### License

- Project license is **Apache-2.0** (was MIT). See `LICENSE` and `NOTICE`.

### UI

- Operate polish: Entangled brand on Connect, collapsed Advanced network, durable error strip, sidebar overflow menu (Leave / Remove saved / Delete), humanized path labels + tooltips, quieter space backdrop, light-theme success/warning/error tokens, a11y focus rings / ARIA / Esc

### Breaking

- Crypto is **HKDF-only** (`hkdf-v1`): legacy raw-X25519 keying and dual-decrypt fallback removed. All peers must rebuild on matching clients (`AppVersion` 1.1.0).
- Relay **REG is token-only** (legacy VIP-only REG rejected).
- Room passwords are **never written** to `rooms.json` (saved networks keep name/server only).
- Stable room owner via persisted **`OwnerPubKey`** (survives delete/rejoin across reconnect).

### Fixed

- Windows client embeds `wintun.dll` and extracts it beside the exe on first run (single-file download)
- CI: TUN/Wintun code is Windows-tagged so Linux `go vet`/`go test` on vpncore pass
- Settings honesty: skip auto-join for password rooms without session pass (`auto_join_skipped`); `SaveSettings` returns reconnect-needed for SOCKS5/STUN/token; locked-room invite copy warns when password omitted; Reset clears Windows Run key
- Hide console flash: `netsh`/`reg` via `HiddenCommand` (`CREATE_NO_WINDOW`)
- TUN Close ends session, nils adapter, recreates stop chan so Start can reopen; P2P/relay send is XOR (no always-also-relay); STUN uses shared listen conn; chat truncate by runes; drop peer packets not destined to local VIP; OnSignal no longer wipes peer info; reconnect waits for auth before JoinRoom
- Room/DM chat wire types (`0x02` room / `0x03` DM); frontend routes by type not view
- Persist `lastRoomName` for AutoJoin; SaveSettings applies live P2P-only + TUN MTU/DNS; PeerCount from peer map length; ParseInvite rejects empty server/room
- Server VIP assign under continuous hub lock; clear VIP on leave/disconnect; refuse pool exhaustion instead of colliding `.254`
- `peer_info` broadcasts server-assigned VIP only (never client-supplied)
- `hashPassword` rand failure refuses room create (no open-room fallback)
- Relay `Start` no longer leaves `running=true` on listen failure
- Empty VIP guard uses `virtualIP` after assign; clear `lastRoomPass` on leave

## 1.0.0

- Usability: reconnect, invite copy/paste, copy VIP, peer path (p2p/relay/ws), room delete for owners
- Chat: per-thread history, delivery/retry, unread badges, system join/leave lines; cipher always installed for relay chat
- Security: argon2id room passwords, optional `ENTANGLED_TOKEN`, relay registration tokens, HKDF session keys, random client IDs, rate limits, redacted WS logs
- OSS: license, README, CONTRIBUTING, CI, smoke tests
- Removed committed deploy secrets / SSH key material from the working tree (rotate any previously exposed keys; scrub history before public push)

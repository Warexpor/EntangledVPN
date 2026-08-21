# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Primary: a small friend group on Windows who want a shared LAN for games, files, and chat without a public VPN brand. They run the client as Administrator (Wintun), join a named room, and stay in it for a session.

## Product Purpose

Entangled VPN creates a private virtual LAN (`10.242.0.0/24`) between friends. Success is: you are in a room, you have a virtual IP, peers are reachable (P2P or relay), and chat/DMs work.

## Positioning

A mesh for a handful of people, not a consumer VPN. Rooms, pipe invites (`server|room|password`), owner delete, and in-mesh chat are the product. Crypto is X25519 + HKDF-only (`hkdf-v1`) + XChaCha20-Poly1305.

## Operating Context

Windows desktop app (Wails v2 + Wintun). Connect screen first; after connect: saved networks, peer list, room chat / DMs, settings. Path can be P2P, UDP relay, or WebSocket. Auto-reconnect re-joins the room. Languages: English, Russian, Chinese.

## Capabilities and Constraints

- Connected shell: networks rail, peer list as home, chat as a side panel (not a top-level app), settings in chrome, disconnect, theme, copy virtual IP, ping, DMs.
- Intro / connect screen with the Entangled title is out of scope for this redesign and must keep its current look.
- Do not invent server hosts, tokens, or crypto claims.
- Frontend can be previewed with the Vite sim (`lib/sim.js`, DEV-only; `?connected=1` jumps into the shell).

## Brand Commitments

- Name: Entangled. Intro screen stays as-is.
- Connected-app palette: high-contrast black-and-white only. Subtle green only for Direct/p2p. Subtle red only for relay/fault/unread/error. No other chromatic accents.
- Connected frame: one chrome row (room + IP + path), quiet room rail, dense peer list, chat panel. No ambient wash, grain, or view-transition gimmicks.
- Overhaul the connected UI (layout and language), not a reskin of the previous chrome.

## Evidence on Hand

- README, CHANGELOG, `client/frontend` Svelte UI, `client/vpncore`.
- No customer quotes or usage metrics. Sim data is synthetic.

## Product Principles

- The room and your virtual IP are the facts that matter.
- Path quality (Direct vs relay) is the primary signal in chrome and the peer list.
- Intro and connected app can look different; connected must still feel like the same product.
- Density for a desktop window, not a marketing page.

## Accessibility & Inclusion

Keyboard focus, `prefers-reduced-motion`, and existing i18n strings. No new copy that is English-only without locale keys.

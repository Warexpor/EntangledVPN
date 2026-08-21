# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Small friend group on Windows. Admin Wintun client. Join a named room for a session of games, files, chat.

## Product Purpose

Private virtual LAN (`10.242.0.0/24`). Success: in a room, have a VIP, peers reachable (P2P or relay), chat works.

## Positioning

Mesh for a handful of people, not a consumer VPN. Rooms, pipe invites, owner delete, in-mesh chat.

## Operating Context

Wails v2 desktop. Connect screen first (frozen). After connect: networks, peers, chat, settings. Path: P2P, UDP relay, or WebSocket. Languages: en, ru, zh. Vite sim for frontend preview.

## Capabilities and Constraints

- Connected: networks rail, peers home, chat, settings (header only), disconnect, theme, copy VIP, ping, DMs.
- Intro / ConnectView + space.css are out of scope and must keep current look.
- Do not invent crypto claims, server hosts, or fake telemetry.
- Preview: Vite DEV sim with `?connected=1` when `window.go` is absent.

## Brand Commitments

- Name: Entangled. Intro stays as-is.
- Connected palette: high-contrast black-and-white. Subtle green only for Direct/p2p. Subtle red only for relay/fault/unread/error. No other chroma.
- Direction: signal-bench Operate shell — path quality first, dense desktop craft, not Discord neon or CRT costume.
- Connected may look different from intro; both must feel like the same product.

## Product Principles

- Room and virtual IP are the facts that matter.
- Path quality is the primary chrome signal.
- Density for a desktop window.
- Prefer real status over decorative instruments.

## Accessibility & Inclusion

Keyboard focus, `prefers-reduced-motion`, existing i18n keys. No English-only new copy without locale keys.

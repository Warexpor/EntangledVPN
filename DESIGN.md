---
name: Entangled VPN — Connected Shell
description: Calm signal-bench Operate UI — matte B&W, path-first chroma, dense peer channels.
colors:
  bg: "#0a0a0a"
  surface: "#111111"
  raised: "#181818"
  border: "#2a2a2a"
  hairline: "#3a3a3a"
  muted: "#8a8a8a"
  text: "#f0f0f0"
  dim: "#5c5c5c"
  cta: "#ffffff"
  cta-ink: "#0a0a0a"
  live: "#2fbf62"
  fault: "#e23d3d"
  bg-light: "#f5f5f5"
  surface-light: "#ececec"
  live-light: "#1a8f45"
  fault-light: "#c41e1e"
typography:
  sans:
    fontFamily: "ui-sans-serif, 'Segoe UI', system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.45
  mono:
    fontFamily: "ui-monospace, 'Cascadia Code', Consolas, monospace"
    fontSize: "12px"
    fontWeight: 500
    lineHeight: 1.35
  label:
    fontFamily: "ui-sans-serif, 'Segoe UI', system-ui, sans-serif"
    fontSize: "10px"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "0.08em"
rounded:
  sm: "4px"
  md: "6px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "24px"
components:
  btn:
    backgroundColor: "transparent"
    textColor: "{colors.text}"
    rounded: "{rounded.md}"
    padding: "6px 12px"
    height: "32px"
  btn-cta:
    backgroundColor: "{colors.cta}"
    textColor: "{colors.cta-ink}"
    rounded: "{rounded.md}"
    padding: "6px 12px"
    height: "32px"
---

# Design System: Entangled VPN — Connected Shell

## Overview

Connected mode is an Operate surface for a friend LAN. It borrows the *idea* of a signal bench — path quality as the primary instrument — without dressing up as a CRT or Discord clone. Intro (`ConnectView` + starfield) is a separate surface and is not restyled here.

## Thesis

Room + VIP + path are the first things you see. Peers are a dense channel list. Chat is available without becoming the product. Green and red are scarce and meaningful.

## Colors

Neutrals only for structure. Two chroma roles:

| Token | Use |
|---|---|
| `--live` | Direct / p2p path only |
| `--fault` | Relay, reconnecting, unread, errors |

Everything else is black, white, or grey. Light theme inverts neutrals and keeps the same two chroma roles.

Connected tokens live under `html[data-app="connected"]` so intro keeps `:root` / Bungee / starfield unchanged.

## Typography

Connected shell uses system sans for chrome and mono for VIP, IPs, ping, and path. Do not load a second display face for connected mode (intro keeps Bungee). Labels are small uppercase with modest tracking — used as field captions in the header, not as kickers above every pane title.

## Layout (first viewport)

1. **Header band** — Entangled wordmark (quiet), active room, copyable VIP, path status (dot + short label), peers count, settings, disconnect
2. **Left rail** — saved networks, create / join
3. **Main** — peers as home (`network`); chat and settings as alternate main panes (keep existing view routing)
4. **Footer** — slim status strip for server / version only (facts already in the header are not repeated loudly)

No starfield behind the connected shell. Opaque bench surfaces only.

## Components

- Ghost / quiet / CTA buttons with 4–6px radius
- 8px path dots (`live` / `fault` / idle grey)
- Peer rows: nick, mono IP, path, ping, ping + chat actions
- Confirm dialogs for destructive actions
- Theme control in settings (and optional header)
- SVG icons only (no Unicode `⋯` / `▼` as chrome)

## Motion

Short opacity / border transitions. Honor `prefers-reduced-motion`. No view-transition gimmicks, grain, or glow.

## Do

- Derive one `pathAggregate`: `direct` | `relay` | `reconnecting` | `disconnected`
- Ship disconnect in connected chrome
- Keep ConnectView markup and `space.css` frozen
- Add Vite `sim.js` + `?connected=1` for preview
- Prefer deletion of ASCII chrome (`[~]`, `[>]`) in connected views

## Don't

- Invent crypto / throughput / engine footers
- Paint green or red for decoration
- Port the previous over-instrumented LED/graticule costume wholesale
- Change intro brand, tagline, or starfield

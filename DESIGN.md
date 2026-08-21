---
name: Entangled VPN — Connected Shell
description: Oscilloscope signal-bench UI for the connected mesh session — matte B&W panels, path-quality chroma, graticule peer channels.
colors:
  bg: "#0a0a0a"
  surface: "#141414"
  surface-2: "#1a1a1a"
  border: "#2a2a2a"
  hairline: "#3a3a3a"
  muted: "#8a8a8a"
  text: "#f5f5f5"
  text-dim: "#5c5c5c"
  cta-fill: "#ffffff"
  cta-ink: "#0a0a0a"
  live: "#2fbf62"
  fault: "#e23d3d"
  graticule: "#262626"
  bg-light: "#ffffff"
  surface-light: "#f7f7f7"
  live-light: "#1a8f45"
  fault-light: "#c41e1e"
typography:
  display:
    fontFamily: "'Bricolage Grotesque', 'Segoe UI', sans-serif"
    fontSize: "11px"
    fontWeight: 500
    lineHeight: 1.2
    letterSpacing: "0.18em"
  headline:
    fontFamily: "'Bricolage Grotesque', 'Segoe UI', sans-serif"
    fontSize: "1.05rem"
    fontWeight: 600
    lineHeight: 1.15
    letterSpacing: "-0.03em"
  title:
    fontFamily: "'Bricolage Grotesque', 'Segoe UI', sans-serif"
    fontSize: "13px"
    fontWeight: 600
    lineHeight: 1.5
    letterSpacing: "0"
  body:
    fontFamily: "'Bricolage Grotesque', 'Segoe UI', sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
    letterSpacing: "0"
  label:
    fontFamily: "'Bricolage Grotesque', 'Segoe UI', sans-serif"
    fontSize: "10px"
    fontWeight: 500
    lineHeight: 1.2
    letterSpacing: "0.14em"
  mono:
    fontFamily: "'IBM Plex Mono', Consolas, monospace"
    fontSize: "1rem"
    fontWeight: 500
    lineHeight: 1.15
    letterSpacing: "0"
rounded:
  sm: "2px"
  md: "8px"
  full: "999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "20px"
components:
  button-ghost:
    backgroundColor: "transparent"
    textColor: "{colors.text}"
    rounded: "{rounded.md}"
    padding: "8px 14px"
    height: "36px"
  button-cta:
    backgroundColor: "{colors.cta-fill}"
    textColor: "{colors.cta-ink}"
    rounded: "{rounded.md}"
    padding: "8px 14px"
    height: "36px"
  bezel:
    backgroundColor: "transparent"
    textColor: "{colors.muted}"
    rounded: "{rounded.sm}"
    padding: "6px 10px"
    height: "32px"
  label-track:
    textColor: "{colors.muted}"
    typography: "{typography.label}"
---

# Design System: Entangled VPN — Connected Shell

## Overview

The connected app is a signal bench, not a consumer VPN dashboard. After join, the window is a dense instrument: one readout band, a quiet networks rail, graticule peer channels, and a chat readout. Path quality is the primary signal. The intro / connect screen (`ConnectView`, `space.css`) is a separate surface and stays out of this system.

## Colors

Matte black-and-white neutrals own every surface. `--live` green and `--fault` red are the only chromatic tokens, and they are scoped:

- Green: Direct / p2p path only
- Red: relay, reconnecting, unread badges, errors

Do not introduce amber, cyan, neon wash, or decorative chroma. Light theme keeps the same roles with inverted neutrals and slightly deeper live/fault.

## Typography

Bricolage Grotesque carries chrome and section titles. IBM Plex Mono carries VIP, peer IPs, ping, path labels, and the chat log. Readout cells use tracked uppercase labels (`.label-track`) over semibold or mono values. Section panes use a single instrument title line — no kicker above the heading.

## Layout

1. Readout band (~64–72px): Entangled wordmark, room, copyable VIP, path LEDs + 7-tick segment bar, peer and relay counts, theme / settings / disconnect
2. Left rail: saved networks, create / join (resizable, ~220px default)
3. Center: `.scope-face` graticule with CH-indexed peer rows and inline path traces
4. Right: chat readout (~42%, max 480px) open by default in a room; settings replaces peer+chat when active

No ambient wash, grain, or view-transition gimmicks. Depth is tonal (surface steps) and 1px hairlines, not drop shadows on the bench itself. Overlay shadows exist only on menus and modals.

## Components

- `.btn` / `.btn-ghost` / `.btn-cta` / `.btn-quiet` / `.btn.danger`
- `.bezel` for chrome actions
- `.led` and segment ticks for path aggregate
- `.scope-face` peer table on a CSS graticule
- Settings `.card` disclosures with SVG chevrons
- Theme switch pill (B&W thumb; no colored track accents)

Icons are drawn SVG at one stroke weight. Unicode menu glyphs are not part of the system.

## Do

- Derive chrome path state from one `pathAggregate` (`direct` | `relay` | `reconnecting` | `disconnected`)
- Keep chat as a side readout, not a top-level app route
- Prefer density suitable for a desktop Wails window
- Preview with Vite sim `?connected=1`

## Don't

- Paint green or red outside path / fault / unread / error roles
- Add a telemetry footer that invents crypto, throughput, or engine claims
- Restyle the intro starfield / Entangled title as part of this system
- Reintroduce section kickers above pane titles

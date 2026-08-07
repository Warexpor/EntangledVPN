# Contributing

## Build

- Go 1.22+ (server may use newer toolchain via `go.mod`)
- Client: Wails v2, Node 18+, Windows for full TUN builds
- Run `go test ./...` in `server/` and `go test ./vpncore/...` in `client/`
- Frontend: `cd client/frontend && npm ci && npm run build`

## Guidelines

- Prefer small, boring diffs.
- Do not commit secrets, SSH keys, or real deploy hosts.
- Keep the visual language (minimal/terminal) unless a change is explicitly a redesign.
- New protocol fields should stay backward-compatible when possible.

## PRs

1. Describe why.
2. Note how you tested (unit tests / two-client smoke).
3. Update `CHANGELOG.md` for user-visible changes.

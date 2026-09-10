#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/sidecar"
GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"
OUT="entangled-sidecar"
if [[ "$GOOS" == "windows" ]]; then
  OUT="entangled-sidecar.exe"
fi
echo "Building sidecar ($GOOS/$GOARCH) -> $OUT"
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$OUT" .
echo "OK $ROOT/sidecar/$OUT"

if [[ "$GOOS" == "linux" && -x "$OUT" ]]; then
  if command -v setcap >/dev/null 2>&1; then
    if setcap cap_net_admin,cap_net_raw+ep "$OUT" 2>/dev/null; then
      echo "setcap: cap_net_admin,cap_net_raw on $OUT"
    elif command -v pkexec >/dev/null 2>&1; then
      echo "Requesting setcap via pkexec (TUN privileges)..."
      pkexec setcap cap_net_admin,cap_net_raw+ep "$(pwd)/$OUT" && echo "setcap OK" || echo "setcap skipped — run: sudo setcap cap_net_admin,cap_net_raw+ep $(pwd)/$OUT"
    else
      echo "setcap skipped — run: sudo setcap cap_net_admin,cap_net_raw+ep $(pwd)/$OUT"
    fi
  fi
fi

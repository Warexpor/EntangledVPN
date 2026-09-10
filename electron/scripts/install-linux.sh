#!/usr/bin/env bash
# Install the packaged Linux Entangled VPN app for the current user.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist-electron"
APPDIR="$(find "$DIST" -maxdepth 2 -type d -name 'linux*-unpacked' | head -1)"
APPIMAGE="$(find "$DIST" -maxdepth 1 -type f -name 'EntangledVPN-*.AppImage' | sort -V | tail -1)"

OPT="${XDG_DATA_HOME:-$HOME/.local/share}/entangledvpn/app"
BIN="$HOME/.local/bin"
APPS="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
ICON_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/icons/hicolor/256x256/apps"

mkdir -p "$OPT" "$BIN" "$APPS" "$ICON_DIR"

if [[ -n "$APPDIR" && -d "$APPDIR" ]]; then
  echo "Installing unpacked app from $APPDIR -> $OPT"
  rsync -a --delete "$APPDIR/" "$OPT/"
  EXEC="$OPT/entangledvpn-electron"
  # electron-builder product binary name
  if [[ ! -x "$EXEC" ]]; then
    EXEC="$(find "$OPT" -maxdepth 1 -type f -executable ! -name '*.so*' | head -1)"
  fi
elif [[ -n "$APPIMAGE" ]]; then
  echo "Installing AppImage $APPIMAGE"
  install -m 755 "$APPIMAGE" "$OPT/EntangledVPN.AppImage"
  EXEC="$OPT/EntangledVPN.AppImage"
else
  echo "No package found in $DIST — run: npm run pack:linux" >&2
  exit 1
fi

# Desktop launcher
cp -f "$ROOT/build/icon.png" "$ICON_DIR/entangledvpn.png" 2>/dev/null || true
cat > "$APPS/entangledvpn.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Entangled VPN
Comment=Mesh VPN for small friend groups
Exec=$EXEC --no-sandbox %U
Icon=entangledvpn
Terminal=false
Categories=Network;Security;
StartupWMClass=entangledvpn-electron
EOF

ln -sfn "$EXEC" "$BIN/entangledvpn"
chmod +x "$EXEC" 2>/dev/null || true

# TUN caps on bundled sidecar (unpacked install). AppImage copies sidecar on first run + pkexec.
SIDECAR="$OPT/resources/entangled-sidecar"
if [[ -x "$SIDECAR" ]]; then
  if command -v pkexec >/dev/null; then
    echo "Granting TUN capabilities (pkexec)..."
    pkexec setcap cap_net_admin,cap_net_raw+ep "$SIDECAR" || true
  fi
  getcap "$SIDECAR" || true
fi

update-desktop-database "$APPS" 2>/dev/null || true
echo
echo "Installed."
echo "  Run:   entangledvpn"
echo "  Or open: Entangled VPN from your app menu"
echo "  App:   $EXEC"
if [[ -n "$APPIMAGE" ]]; then
  echo "  AppImage also at: $APPIMAGE"
fi

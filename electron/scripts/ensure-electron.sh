#!/usr/bin/env bash
# Fallback Electron binary install when extract-zip leaves a broken dist/.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ELECTRON_DIR="$ROOT/node_modules/electron"
DIST="$ELECTRON_DIR/dist"
BIN="$DIST/electron"
VERSION="$(node -p "require('$ELECTRON_DIR/package.json').version")"

if [[ -x "$BIN" ]]; then
  echo "electron binary ok: $BIN"
  exit 0
fi

echo "electron binary missing; downloading v$VERSION ..."
node -e "
const {downloadArtifact}=require('@electron/get');
downloadArtifact({
  version: process.argv[1],
  artifactName: 'electron',
  platform: process.platform,
  arch: process.arch,
  force: true,
}).then(p=>{console.log(p)}).catch(e=>{console.error(e);process.exit(1)})
" "$VERSION" > /tmp/electron-zip-path.txt

ZIP="$(cat /tmp/electron-zip-path.txt | tail -1)"
rm -rf "$DIST"
mkdir -p "$DIST"
unzip -qo "$ZIP" -d "$DIST"
echo -n electron > "$ELECTRON_DIR/path.txt"
test -x "$BIN"
echo "installed $BIN"

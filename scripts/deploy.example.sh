#!/usr/bin/env bash
# Example deploy — copy to deploy.sh (gitignored) and fill env vars.
# NEVER commit real hosts, keys, or passphrases.
set -euo pipefail

: "${ENTANGLED_HOST:?set ENTANGLED_HOST}"
: "${ENTANGLED_SSH_USER:=root}"
: "${ENTANGLED_SSH_PORT:=22}"
: "${ENTANGLED_SSH_KEY:?set ENTANGLED_SSH_KEY path to private key}"
: "${ENTANGLED_BIN:=server/entangled-server-linux}"
: "${ENTANGLED_REMOTE_PATH:=/opt/entangled-server}"

scp -P "$ENTANGLED_SSH_PORT" -i "$ENTANGLED_SSH_KEY" \
  "$ENTANGLED_BIN" \
  "${ENTANGLED_SSH_USER}@${ENTANGLED_HOST}:/tmp/entangled-server"

ssh -p "$ENTANGLED_SSH_PORT" -i "$ENTANGLED_SSH_KEY" \
  "${ENTANGLED_SSH_USER}@${ENTANGLED_HOST}" bash -s <<EOF
set -euo pipefail
systemctl stop entangled-server 2>/dev/null || true
mv /tmp/entangled-server "$ENTANGLED_REMOTE_PATH"
chmod 755 "$ENTANGLED_REMOTE_PATH"
systemctl start entangled-server
systemctl is-active entangled-server
curl -sf http://127.0.0.1:8080/health
EOF

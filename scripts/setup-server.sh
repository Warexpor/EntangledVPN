#!/bin/bash
set -euo pipefail

echo "========================================"
echo "  Entangled VPN Server Setup"
echo "========================================"

# Install Go if not present
if ! command -v go &> /dev/null; then
    echo "[1/4] Installing Go..."
    wget -q https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
    sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    rm go1.22.5.linux-amd64.tar.gz
else
    echo "[1/4] Go already installed: $(go version)"
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVER_DIR="$SCRIPT_DIR/../server"
INSTALL_DIR="/var/lib/entangled"
BIN_DIR="/usr/local/bin"

echo "[2/4] Building server..."
cd "$SERVER_DIR"
go mod tidy
go build -o entangled-server .

echo "[3/4] Installing binary + data dir..."
sudo mkdir -p "$INSTALL_DIR"
sudo chown nobody:nogroup "$INSTALL_DIR" 2>/dev/null || sudo chown nobody:nobody "$INSTALL_DIR"
sudo install -m 0755 "$SERVER_DIR/entangled-server" "$BIN_DIR/entangled-server"

TOKEN_FILE="$INSTALL_DIR/server.token"
ALLOW_OPEN=0
if [[ "${ENTANGLED_ALLOW_OPEN_JOIN:-}" == "1" ]]; then
  ALLOW_OPEN=1
fi

if [[ -z "${ENTANGLED_TOKEN:-}" ]]; then
  if [[ -f "$TOKEN_FILE" ]]; then
    ENTANGLED_TOKEN="$(sudo cat "$TOKEN_FILE")"
  elif [[ "$ALLOW_OPEN" -eq 1 ]]; then
    ENTANGLED_TOKEN=""
    echo "WARNING: ENTANGLED_ALLOW_OPEN_JOIN=1 — server will accept unauthenticated joins."
  else
    ENTANGLED_TOKEN="$(openssl rand -hex 24)"
    echo "$ENTANGLED_TOKEN" | sudo tee "$TOKEN_FILE" >/dev/null
    sudo chmod 0600 "$TOKEN_FILE"
    sudo chown nobody:nogroup "$TOKEN_FILE" 2>/dev/null || sudo chown nobody:nobody "$TOKEN_FILE"
    echo "Generated server token (also in $TOKEN_FILE)."
    echo "Clients must set this as Server Token. Put TLS in front (nginx/caddy) for production."
  fi
fi

echo "[4/4] Installing systemd service..."
sudo tee /etc/systemd/system/entangled-server.service > /dev/null <<EOF
[Unit]
Description=Entangled VPN Server
After=network.target
# Put a TLS reverse proxy in front of :8080 for production (wss).

[Service]
Type=simple
User=nobody
Group=nogroup
WorkingDirectory=$INSTALL_DIR
Environment=ENTANGLED_TOKEN=$ENTANGLED_TOKEN
ExecStart=$BIN_DIR/entangled-server -addr :8080 -relay :3478
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable entangled-server
sudo systemctl restart entangled-server

echo ""
echo "========================================"
echo "  Server installed successfully!"
echo "  Data dir: $INSTALL_DIR"
echo "  Port: 8080 (prefer TLS reverse proxy for wss)"
echo "  Status: $(sudo systemctl is-active entangled-server)"
if [[ -n "$ENTANGLED_TOKEN" ]]; then
  echo "  Token: set (see $TOKEN_FILE if generated)"
else
  echo "  Token: OPEN JOIN (ENTANGLED_ALLOW_OPEN_JOIN=1)"
fi
echo "========================================"
echo ""
echo "Check logs: sudo journalctl -u entangled-server -f"
echo "Update: cd $SERVER_DIR && git pull && go build && sudo install -m 0755 entangled-server $BIN_DIR/entangled-server && sudo systemctl restart entangled-server"

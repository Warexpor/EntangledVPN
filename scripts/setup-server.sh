#!/bin/bash
set -e

echo "========================================"
echo "  Entangled VPN Server Setup"
echo "========================================"

# Install Go if not present
if ! command -v go &> /dev/null; then
    echo "[1/3] Installing Go..."
    wget -q https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
    sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    rm go1.22.5.linux-amd64.tar.gz
else
    echo "[1/3] Go already installed: $(go version)"
fi

# Build server
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVER_DIR="$SCRIPT_DIR/../server"

echo "[2/3] Building server..."
cd "$SERVER_DIR"
go mod tidy
go build -o entangled-server .

# Install as systemd service
echo "[3/3] Installing systemd service..."
sudo tee /etc/systemd/system/entangled-server.service > /dev/null <<EOF
[Unit]
Description=Entangled VPN Server
After=network.target

[Service]
Type=simple
User=nobody
WorkingDirectory=$(pwd)
ExecStart=$(pwd)/entangled-server -addr :8080 -relay :3478
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable entangled-server
sudo systemctl start entangled-server

echo ""
echo "========================================"
echo "  Server installed successfully!"
echo "  Port: 8080"
echo "  Status: $(sudo systemctl is-active entangled-server)"
echo "========================================"
echo ""
echo "Check logs: sudo journalctl -u entangled-server -f"
echo "Update server: cd $SERVER_DIR && git pull && go build && sudo systemctl restart entangled-server"

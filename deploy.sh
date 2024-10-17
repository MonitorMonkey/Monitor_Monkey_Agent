#!/usr/bin/env bash

set -euo pipefail

# Configuration
DEPLOY_LOCATION="/opt/monitor-monkey"
AGENT_URL="https://github.com/oidz1234/go_monit_test/raw/master/monitor-monkey-agent"
AGENT_BIN="${DEPLOY_LOCATION}/monitor-monkey-agent"
UNIT_FILE="/etc/systemd/system/monitor-monkey.service"
UNIT_NAME="monitor-monkey.service"

download_file() {
    local url="$1"
    local output="$2"
    
    if command -v wget &> /dev/null; then
        wget -q "$url" -O "$output"
    elif command -v curl &> /dev/null; then
        curl -sSL "$url" -o "$output"
    else
        echo "Error: Neither wget nor curl is installed. Please install one of them and try again."
        exit 1
    fi
}

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root" 
   exit 1
fi

# Ensure API key is set
if [[ -z "${API_KEY:-}" ]]; then
    echo "Error: API_KEY environment variable is not set."
    exit 1
fi

# Create deploy directory
mkdir -p "$DEPLOY_LOCATION"

# Download agent
echo "Downloading agent..."
if ! download_file "$AGENT_URL" "$AGENT_BIN"; then
    echo "Error: Failed to download agent."
    exit 1
fi

# Set permissions
chmod 700 "$AGENT_BIN"

# Create systemd unit file
cat << EOF > "$UNIT_FILE"
[Unit]
Description=Monitor Monkey Agent
After=network.target

[Service]
ExecStart=$AGENT_BIN $API_KEY
Restart=always
RestartSec=5s
User=nobody
Group=nogroup

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd, enable and start the service
systemctl daemon-reload
systemctl enable "$UNIT_NAME"
systemctl restart "$UNIT_NAME"

echo "Monitor Monkey Agent deployed successfully."

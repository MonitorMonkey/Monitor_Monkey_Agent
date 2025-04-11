#!/usr/bin/env bash

set -euo pipefail

# Configuration
DEPLOY_LOCATION="/opt/monitor-monkey"
AGENT_URL_X86="https://github.com/MonitorMonkey/Monitor_Monkey_Agent/raw/refs/heads/master/monitor-monkey-agent"
AGENT_URL_ARM="https://github.com/MonitorMonkey/Monitor_Monkey_Agent/raw/refs/heads/master/monitor-monkey-agent-arm"
AGENT_BIN="${DEPLOY_LOCATION}/monitor-monkey-agent"
UNIT_FILE="/etc/systemd/system/monitor-monkey.service"
UNIT_NAME="monitor-monkey.service"
SERVICE_USER="monitor-monkey"
SERVICE_GROUP="monitor-monkey"

# Function to download file
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

# Check for API key in environment variable
if [[ -z "${API_KEY:-}" ]]; then
    echo "Error: API_KEY environment variable is not set."
    exit 1
fi

# Detect ARM architecture
ARCH=$(uname -m)
if [[ "$ARCH" == "arm"* || "$ARCH" == "aarch64" ]]; then
    echo "ARM architecture detected: $ARCH"
    AGENT_URL="$AGENT_URL_ARM"
else
    echo "x86 architecture detected: $ARCH"
    AGENT_URL="$AGENT_URL_X86"
fi

# Create service user and group if they don't exist
if ! getent group "$SERVICE_GROUP" > /dev/null; then
    groupadd -r "$SERVICE_GROUP"
fi
if ! id "$SERVICE_USER" &>/dev/null; then
    useradd -r -g "$SERVICE_GROUP" -s /sbin/nologin -d "$DEPLOY_LOCATION" "$SERVICE_USER"
fi

# Create deploy directory and set permissions
mkdir -p "$DEPLOY_LOCATION"
chown "$SERVICE_USER:$SERVICE_GROUP" "$DEPLOY_LOCATION"

# Download agent
echo "Downloading agent from $AGENT_URL..."
if ! download_file "$AGENT_URL" "$AGENT_BIN"; then
    echo "Error: Failed to download agent."
    exit 1
fi

# Set permissions
chown "$SERVICE_USER:$SERVICE_GROUP" "$AGENT_BIN"
chmod 700 "$AGENT_BIN"

# Create systemd unit file
cat << EOF > "$UNIT_FILE"
[Unit]
Description=Monitor Monkey Agent
After=network.target

[Service]
Environment="MONKEY_API_KEY=$API_KEY"
ExecStart=$AGENT_BIN
Restart=always
RestartSec=5s
User=$SERVICE_USER
Group=$SERVICE_GROUP

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd, enable and start the service
systemctl daemon-reload
systemctl enable "$UNIT_NAME"
systemctl restart "$UNIT_NAME"

echo "Monitor Monkey Agent deployed successfully."

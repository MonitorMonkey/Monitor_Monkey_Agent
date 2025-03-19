#!/usr/bin/env bash

# Simple script to update Monitor Monkey agent without any checks

# Configuration
DEPLOY_LOCATION="/opt/monitor-monkey"
AGENT_URL="https://github.com/MonitorMonkey/Monitor_Monkey_Agent/raw/refs/heads/master/monitor-monkey-agent"
AGENT_BIN="${DEPLOY_LOCATION}/monitor-monkey-agent"
UNIT_NAME="monitor-monkey.service"
SERVICE_USER="monitor-monkey"
SERVICE_GROUP="monitor-monkey"

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root" 
   exit 1
fi

echo "Updating Monitor Monkey Agent..."

# Stop the service
systemctl stop "$UNIT_NAME"

# Download the latest agent
if command -v wget &> /dev/null; then
    wget -q "$AGENT_URL" -O "$AGENT_BIN"
elif command -v curl &> /dev/null; then
    curl -sSL "$AGENT_URL" -o "$AGENT_BIN"
else
    echo "Error: Neither wget nor curl is installed."
    exit 1
fi

# Set correct permissions
chown "$SERVICE_USER:$SERVICE_GROUP" "$AGENT_BIN"
chmod 700 "$AGENT_BIN"

# Start the service
systemctl start "$UNIT_NAME"

echo "Agent update completed."

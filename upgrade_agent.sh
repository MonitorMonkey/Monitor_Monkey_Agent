#!/usr/bin/env bash

# Monitor Monkey Agent Update Script
# This script updates the agent and ensures it displays status information after starting

set -euo pipefail

# Configuration (matching deploy.sh)
DEPLOY_LOCATION="/opt/monitor-monkey"
AGENT_URL="https://github.com/MonitorMonkey/Monitor_Monkey_Agent/raw/refs/heads/master/monitor-monkey-agent"
AGENT_BIN="${DEPLOY_LOCATION}/monitor-monkey-agent"
BACKUP_BIN="${DEPLOY_LOCATION}/monitor-monkey-agent.bak"
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

# Function to check if agent has a specific command/flag
has_command() {
    local bin="$1"
    local cmd="$2"
    
    # Attempt to run with --help to see if command exists
    "$bin" --help 2>&1 | grep -q "$cmd" || return 1
    return 0
}

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root" 
   exit 1
fi

# Check if the service exists
if ! systemctl list-unit-files | grep -q "$UNIT_NAME"; then
    echo "Error: Monitor Monkey service not found. Please run the deployment script first."
    exit 1
fi

# Check if the deployment directory exists
if [ ! -d "$DEPLOY_LOCATION" ]; then
    echo "Error: Deployment directory not found. Please run the deployment script first."
    exit 1
fi

echo "Updating Monitor Monkey Agent..."

# Stop the service
echo "Stopping the Monitor Monkey service..."
systemctl stop "$UNIT_NAME"

# Backup the current agent
if [ -f "$AGENT_BIN" ]; then
    echo "Backing up current agent..."
    cp "$AGENT_BIN" "$BACKUP_BIN"
fi

# Download the latest agent
echo "Downloading the latest agent..."
if ! download_file "$AGENT_URL" "$AGENT_BIN"; then
    echo "Error: Failed to download agent. Restoring backup..."
    if [ -f "$BACKUP_BIN" ]; then
        cp "$BACKUP_BIN" "$AGENT_BIN"
    fi
    echo "Restarting service with previous version..."
    systemctl start "$UNIT_NAME"
    exit 1
fi

# Set correct permissions
chown "$SERVICE_USER:$SERVICE_GROUP" "$AGENT_BIN"
chmod 700 "$AGENT_BIN"

# Restart the service
echo "Restarting the Monitor Monkey service..."
systemctl restart "$UNIT_NAME"

# Check if service started successfully
if systemctl is-active --quiet "$UNIT_NAME"; then
    echo "Update completed successfully."
    echo "Cleaning up backup..."
    rm -f "$BACKUP_BIN"
    
    # Wait for service to fully initialize
    sleep 3
    
    echo "Monitor Monkey Agent has been updated and is running."
    
    # Display agent status and version information
    echo "Checking agent version and status..."
    
    # Use the agent's built-in status and version commands
    echo "Agent version:"
    sudo -u "$SERVICE_USER" "$AGENT_BIN" --version
    
    echo "Agent status:"
    sudo -u "$SERVICE_USER" "$AGENT_BIN" --status
    
    # Show service is active
    echo "Service status:"
    systemctl status "$UNIT_NAME" --no-pager | grep "Active:"
else
    echo "Error: Service failed to start after update. Rolling back..."
    if [ -f "$BACKUP_BIN" ]; then
        cp "$BACKUP_BIN" "$AGENT_BIN"
        chown "$SERVICE_USER:$SERVICE_GROUP" "$AGENT_BIN"
        chmod 700 "$AGENT_BIN"
        systemctl restart "$UNIT_NAME"
        echo "Rolled back to previous version."
    else
        echo "Error: No backup available. Service may be in a broken state."
    fi
    exit 1
fi

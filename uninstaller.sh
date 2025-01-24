#!/usr/bin/env bash

set -euo pipefail

# Configuration (matching deploy.sh)
DEPLOY_LOCATION="/opt/monitor-monkey"
UNIT_FILE="/etc/systemd/system/monitor-monkey.service"
UNIT_NAME="monitor-monkey.service"
SERVICE_USER="monitor-monkey"
SERVICE_GROUP="monitor-monkey"

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root" 
   exit 1
fi

echo "Beginning Monitor Monkey Agent uninstallation..."

# Stop and disable the service if it exists
if systemctl is-active --quiet "$UNIT_NAME"; then
    echo "Stopping Monitor Monkey service..."
    systemctl stop "$UNIT_NAME"
fi

if systemctl is-enabled --quiet "$UNIT_NAME" 2>/dev/null; then
    echo "Disabling Monitor Monkey service..."
    systemctl disable "$UNIT_NAME"
fi

# Remove systemd unit file
if [ -f "$UNIT_FILE" ]; then
    echo "Removing systemd unit file..."
    rm -f "$UNIT_FILE"
    systemctl daemon-reload
fi

# Remove deployment directory and its contents
if [ -d "$DEPLOY_LOCATION" ]; then
    echo "Removing deployment directory..."
    rm -rf "$DEPLOY_LOCATION"
fi

# Remove service user and group
if id "$SERVICE_USER" &>/dev/null; then
    echo "Removing service user..."
    userdel "$SERVICE_USER"
fi

if getent group "$SERVICE_GROUP" > /dev/null; then
    echo "Removing service group..."
    groupdel "$SERVICE_GROUP"
fi

echo "Monitor Monkey Agent has been successfully uninstalled."

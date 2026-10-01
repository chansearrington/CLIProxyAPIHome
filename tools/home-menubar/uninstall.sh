#!/bin/bash
# Removes Home Usage completely. Pass --forget-key to also delete the Home key
# from the login Keychain.
set -uo pipefail
LABEL="com.chansearrington.home-usage"
launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
rm -f "$HOME/Library/LaunchAgents/$LABEL.plist"
rm -rf "$HOME/Applications/Home Usage.app"
rm -f "$HOME/Library/Logs/home-usage.log"
defaults delete "$LABEL" 2>/dev/null || true
if [ "${1:-}" = "--forget-key" ]; then
  security delete-generic-password -s cpa-home-management -a home-usage >/dev/null 2>&1 && echo "Key removed from Keychain"
fi
echo "Home Usage removed"

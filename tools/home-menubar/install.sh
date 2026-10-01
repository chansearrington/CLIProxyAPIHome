#!/bin/bash
# Builds Home Usage from HomeUsage.swift, installs it to ~/Applications and
# starts it at login through a per-user LaunchAgent. Safe to re-run (upgrade).
# Needs the Xcode command line tools. The Home key must already be in the
# login Keychain (service cpa-home-management, account home-usage); see README.md.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
APP="$HOME/Applications/Home Usage.app"
LABEL="com.chansearrington.home-usage"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
BUILD="$(mktemp -d)"
trap 'rm -rf "$BUILD"' EXIT

swiftc -O -swift-version 5 -target arm64-apple-macos14 \
  -o "$BUILD/HomeUsage" "$HERE/HomeUsage.swift"

mkdir -p "$BUILD/Home Usage.app/Contents/MacOS"
cp "$BUILD/HomeUsage" "$BUILD/Home Usage.app/Contents/MacOS/HomeUsage"
cat > "$BUILD/Home Usage.app/Contents/Info.plist" <<PLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleIdentifier</key><string>$LABEL</string>
  <key>CFBundleName</key><string>Home Usage</string>
  <key>CFBundleExecutable</key><string>HomeUsage</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>1.0</string>
  <key>LSMinimumSystemVersion</key><string>14.0</string>
  <key>LSUIElement</key><true/>
  <key>NSAppTransportSecurity</key>
  <dict><key>NSAllowsArbitraryLoads</key><true/></dict>
</dict>
</plist>
PLISTEOF
codesign --force --sign - "$BUILD/Home Usage.app" >/dev/null 2>&1

launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
mkdir -p "$HOME/Applications" "$HOME/Library/LaunchAgents"
rm -rf "$APP"
cp -R "$BUILD/Home Usage.app" "$APP"

cat > "$PLIST" <<PLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>$LABEL</string>
  <key>ProgramArguments</key><array><string>$APP/Contents/MacOS/HomeUsage</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
  <key>ProcessType</key><string>Interactive</string>
  <key>StandardErrorPath</key><string>$HOME/Library/Logs/home-usage.log</string>
</dict>
</plist>
PLISTEOF
launchctl bootstrap "gui/$(id -u)" "$PLIST"
echo "Installed $APP and started $LABEL"

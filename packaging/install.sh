#!/bin/bash
set -e

APP_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
BIN_SRC="$APP_DIR/Contents/MacOS/backlog"

TARGET_DIR="/usr/local/bin"
if [ ! -w "$TARGET_DIR" ]; then
    TARGET_DIR="$HOME/.local/bin"
fi

mkdir -p "$TARGET_DIR"
ln -sf "$BIN_SRC" "$TARGET_DIR/backlog"

echo "✔ Backlog CLI linked: $TARGET_DIR/backlog -> $BIN_SRC"
echo "You can now run 'backlog' from any terminal."

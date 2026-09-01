#!/bin/bash
set -e

echo "Removing Backlog CLI symlinks..."
rm -f /usr/local/bin/backlog "$HOME/.local/bin/backlog"
echo "✔ Removed Backlog CLI symlinks."

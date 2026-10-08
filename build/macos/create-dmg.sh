#!/bin/bash
# Package an existing bundle, preserving any release signature.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
APP="${1:-$ROOT/bin/Backlog.app}"
OUTPUT="${2:-$ROOT/bin/Backlog.dmg}"
PYTHON="${DMG_PYTHON:-$ROOT/bin/dmg-venv/bin/python}"

if [[ "$(uname -s)" != Darwin ]]; then
    echo "DMG packaging requires macOS." >&2
    exit 1
fi
if [[ ! -d "$APP/Contents" ]]; then
    echo "App bundle not found: $APP (run make bundle first)." >&2
    exit 1
fi
if [[ ! -x "$PYTHON" ]]; then
    echo "Run make dmg-deps first, or set DMG_PYTHON to a Python with dmgbuild and Pillow." >&2
    exit 1
fi

mkdir -p "$(dirname "$OUTPUT")"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/backlog-dmg.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT
"$PYTHON" "$ROOT/build/macos/dmg/background.py" "$WORK/background.png"
# dmgbuild writes Finder metadata directly, so CI needs no Finder automation.
"$PYTHON" -m dmgbuild -s "$ROOT/build/macos/dmg/settings.py" \
    -D "app=$APP" -D "background=$WORK/background.png" \
    "Backlog" "$WORK/Backlog.dmg"
# Leave any previous output intact if packaging fails.
mv -f "$WORK/Backlog.dmg" "$OUTPUT"
echo "Created $OUTPUT"

#!/bin/bash

set -euo pipefail

SERVICE_DIR="${1:-/usr/share/linuxhello/python-service}"
REQUIREMENTS_FILE="$SERVICE_DIR/requirements.txt"
VENV_DIR="$SERVICE_DIR/venv"
HASH_FILE="$VENV_DIR/.linuxhello_requirements.sha256"
FORCE_SYNC="${LINUXHELLO_FORCE_VENV_SYNC:-0}"

if [ ! -f "$REQUIREMENTS_FILE" ]; then
    echo "requirements.txt not found in $SERVICE_DIR" >&2
    exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
    echo "python3 is required for LinuxHello inference service" >&2
    exit 1
fi

if [ ! -x "$VENV_DIR/bin/python3" ]; then
    python3 -m venv "$VENV_DIR"
fi

if ! command -v sha256sum >/dev/null 2>&1; then
    echo "sha256sum is required to validate Python dependencies" >&2
    exit 1
fi

CURRENT_HASH="$(sha256sum "$REQUIREMENTS_FILE" | awk '{print $1}')"
SAVED_HASH=""
if [ -f "$HASH_FILE" ]; then
    SAVED_HASH="$(cat "$HASH_FILE")"
fi

if [ "$FORCE_SYNC" = "1" ] || [ "$CURRENT_HASH" != "$SAVED_HASH" ]; then
    "$VENV_DIR/bin/pip" install --quiet --upgrade pip setuptools wheel
    "$VENV_DIR/bin/pip" install --quiet --upgrade --upgrade-strategy only-if-needed -r "$REQUIREMENTS_FILE"
    printf "%s\n" "$CURRENT_HASH" > "$HASH_FILE"
fi

#!/bin/bash
set -euo pipefail

REPO_DIR="/workspace/repo"
if [ ! -d "$REPO_DIR" ]; then
    REPO_DIR="$(pwd)"
fi

# 1. Hard Rule: Remove all .py files anywhere in the repo
find "$REPO_DIR" -name "*.py" -type f -delete

# 2. Locate directory containing solve.sh and solution files
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Copy Go files to $REPO_DIR if they are not already there
if [ "$SCRIPT_DIR" != "$REPO_DIR" ]; then
    cp -r "$SCRIPT_DIR"/go.mod "$SCRIPT_DIR"/*.go "$REPO_DIR"/ 2>/dev/null || true
    if [ -d "$SCRIPT_DIR/fonts" ]; then
        mkdir -p "$REPO_DIR/fonts"
        cp -r "$SCRIPT_DIR/fonts/"* "$REPO_DIR/fonts/" 2>/dev/null || true
    fi
fi

# Ensure fonts exist for //go:embed fonts/*.flf
if [ ! -d "$REPO_DIR/fonts" ]; then
    mkdir -p "$REPO_DIR/fonts"
    if [ -d "$REPO_DIR/pyfiglet/fonts" ]; then
        cp -r "$REPO_DIR/pyfiglet/fonts/"*.flf "$REPO_DIR/fonts/" 2>/dev/null || true
    elif [ -d "$REPO_DIR/pyfiglet/fonts-standard" ]; then
        cp -r "$REPO_DIR/pyfiglet/fonts-standard/"*.flf "$REPO_DIR/fonts/" 2>/dev/null || true
    fi
fi

# 3. Build Go binary to /tmp/figlet and verify go build ./...
cd "$REPO_DIR"
go build -o /tmp/figlet .
go build ./...

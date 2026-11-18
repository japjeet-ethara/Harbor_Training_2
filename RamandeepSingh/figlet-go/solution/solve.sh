#!/bin/bash
set -euo pipefail

# Locate the directory where solve.sh resides
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="/workspace/repo"

if [ ! -d "$REPO_DIR" ]; then
    echo "Error: Target repository directory '$REPO_DIR' does not exist." >&2
    exit 1
fi

echo "==> Deploying Go FIGlet solution to $REPO_DIR"

# Copy main.go and go.mod from the solution directory into /workspace/repo
cp -f "$SCRIPT_DIR/main.go" "$REPO_DIR/main.go"
cp -f "$SCRIPT_DIR/go.mod" "$REPO_DIR/go.mod"

# Ensure /workspace/repo/pyfiglet/fonts exists and retains the standard fonts
# prepared by the environment Dockerfile
if [ ! -d "$REPO_DIR/pyfiglet/fonts" ] || [ -z "$(ls -A "$REPO_DIR/pyfiglet/fonts" 2>/dev/null)" ]; then
    echo "==> Populating pyfiglet/fonts from fonts-standard..."
    mkdir -p "$REPO_DIR/pyfiglet/fonts"
    if [ -d "$REPO_DIR/pyfiglet/fonts-standard" ]; then
        cp -f "$REPO_DIR/pyfiglet/fonts-standard/"*.flf "$REPO_DIR/pyfiglet/fonts/" 2>/dev/null || true
        cp -f "$REPO_DIR/pyfiglet/fonts-standard/"*.tlf "$REPO_DIR/pyfiglet/fonts/" 2>/dev/null || true
        cp -f "$REPO_DIR/pyfiglet/fonts-standard/"*.flc "$REPO_DIR/pyfiglet/fonts/" 2>/dev/null || true
    fi
fi

# Remove all .py files from /workspace/repo only
echo "==> Removing all Python files from $REPO_DIR"
find "$REPO_DIR" -type f -name "*.py" -delete

# Build the Go program from /workspace/repo
echo "==> Building Go program in $REPO_DIR"
(
    cd "$REPO_DIR"
    go build -o /tmp/figlet .
)

if [ ! -f /tmp/figlet ]; then
    echo "Error: /tmp/figlet was not created after build." >&2
    exit 1
fi

chmod +x /tmp/figlet 2>/dev/null || true

echo "==> Solution successfully deployed and built at /tmp/figlet"

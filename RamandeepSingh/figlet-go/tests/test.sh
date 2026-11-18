#!/bin/bash
set -euo pipefail

# ---------------------------------------------------------------------------
# Harbor Verifier Test Script for figlet-go
# ---------------------------------------------------------------------------

# Resolve script directory (where grade.go lives)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GRADE_SCRIPT="${GRADE_SCRIPT:-$SCRIPT_DIR/grade.go}"

# Change to the candidate repository directory
REPO_DIR="${REPO_DIR:-/workspace/repo}"
if [ -d "$REPO_DIR" ]; then
    cd "$REPO_DIR"
fi

if [ ! -f "$GRADE_SCRIPT" ]; then
    echo "Error: grade.go could not be found at $GRADE_SCRIPT" >&2
    exit 1
fi

# Ensure safe Go environment variables inside Alpine container
export CGO_ENABLED=0
export GOTOOLCHAIN=local
export GOCACHE="${GOCACHE:-/tmp/gocache}"
export GOTMPDIR="${GOTMPDIR:-/tmp}"
mkdir -p "$GOCACHE" /logs/verifier 2>/dev/null || true

# Execute the Go grader
echo "Starting Harbor FIGlet Go grading suite..."
go run "$GRADE_SCRIPT"

# Ensure reward was generated
if [ ! -f "/logs/verifier/reward.txt" ] && [ ! -f "./verifier_logs/reward.txt" ]; then
    echo "Error: Grader did not write reward output file" >&2
    exit 1
fi

echo "Grading complete."

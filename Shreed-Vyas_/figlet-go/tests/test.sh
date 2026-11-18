#!/bin/bash
# Harbor verifier entry point. Writes /logs/verifier/reward.txt
set -u
LOG_DIR="${LOG_DIR:-/logs/verifier}"
mkdir -p "$LOG_DIR" 2>/dev/null || true
export GOTOOLCHAIN=local
export GOCACHE="${GOCACHE:-/tmp/figlet-grader-cache}"
export HOME="${HOME:-/tmp}"
cd "${REPO_DIR:-/workspace/repo}" || { echo 0 > "$LOG_DIR/reward.txt"; exit 0; }
go run "${TESTS_DIR:-/tests}/grade.go" || echo "grader failed"
[ -f "$LOG_DIR/reward.txt" ] || echo 0 > "$LOG_DIR/reward.txt"
exit 0

#!/bin/bash
# Harbor verifier entry point: build the Go grader and score the submission.
# Exits non-zero only if the grader itself cannot run.
set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${REPO:-/workspace/repo}"
LOG_DIR="${LOG_DIR:-/logs/verifier}"

if ! command -v go >/dev/null 2>&1; then
    echo "test.sh: go toolchain not found" >&2
    exit 1
fi
for f in grade.go data/hidden.jsonl secret.b64; do
    if [ ! -f "$TESTS_DIR/$f" ]; then
        echo "test.sh: missing $TESTS_DIR/$f" >&2
        exit 1
    fi
done

mkdir -p "$LOG_DIR"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Build the grader from a private copy, isolated from the submission's
# module, workspace and environment settings.
cp "$TESTS_DIR/grade.go" "$WORK/grade.go"
(
    cd "$WORK"
    env CGO_ENABLED=0 GOTOOLCHAIN=local GOWORK=off GOFLAGS= GO111MODULE=on \
        GOCACHE="$WORK/gocache" \
        go build -o "$WORK/grade" ./grade.go
)

exec "$WORK/grade" \
    -repo "$REPO" \
    -cases "$TESTS_DIR/data/hidden.jsonl" \
    -secret "$TESTS_DIR/secret.b64" \
    -bin /tmp/figlet \
    -out "$LOG_DIR/reward.json"

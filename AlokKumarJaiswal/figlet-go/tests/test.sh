#!/bin/bash
set -euo pipefail

# Require effective UID 0; fail otherwise.
if [ "${EUID:-$(id -u)}" -ne 0 ]; then
    echo "ERROR: verifier must run as root (EUID 0)" >&2
    exit 1
fi

echo "=== GRADER STARTUP ==="

# Before compiling/running the trusted grader, protect /tests:
# reject symlink entries, make its contents root-owned, and remove
# all group/other permissions recursively.
if [ -L /tests ] || [ -n "$(find /tests -type l -print)" ]; then
    echo "ERROR: symlink entries rejected in /tests" >&2
    exit 1
fi
chown -R root:root /tests
chmod -R go-rwx /tests

# Protect /logs/verifier with root ownership and mode 0700.
mkdir -p /logs/verifier
chown root:root /logs/verifier
chmod 0700 /logs/verifier

# Keep the trusted grader's GOCACHE/GOPATH in a root-owned private temporary directory.
GRADE_TMP="$(mktemp -d /tmp/grade_XXXXXX)"
SOL_TMP="$(mktemp -d /tmp/solution_cache_XXXXXX)"
trap 'rm -rf "$GRADE_TMP" "$SOL_TMP"' EXIT

chown root:root "$GRADE_TMP"
chmod 0700 "$GRADE_TMP"

export GOCACHE="$GRADE_TMP/gocache"
export GOPATH="$GRADE_TMP/gopath"
mkdir -p "$GOCACHE" "$GOPATH"
chmod 0700 "$GOCACHE" "$GOPATH"

# Create a SEPARATE temporary build-cache directory for solution commands.
# Create gocache/gopath within it, owned by 1000:1000, accessible to player.
# Export their paths as SOLUTION_GOCACHE and SOLUTION_GOPATH.
export SOLUTION_GOCACHE="$SOL_TMP/gocache"
export SOLUTION_GOPATH="$SOL_TMP/gopath"
mkdir -p "$SOLUTION_GOCACHE" "$SOLUTION_GOPATH"
chown -R 1000:1000 "$SOL_TMP"
chmod -R 0700 "$SOL_TMP"

# Retain offline Go settings and cd /tests; go run grade.go.
export GOTOOLCHAIN=local
export CGO_ENABLED=0
export GOPROXY=off
export GOSUMDB=off

cd /tests
go run grade.go

#!/bin/bash
set -euo pipefail

mkdir -p /logs/verifier

# Keep Go's build cache writable even if HOME is not set in the verifier.
export GOCACHE="${GOCACHE:-/tmp/go-build-cache}"
export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"

cd /tests
go run grade.go

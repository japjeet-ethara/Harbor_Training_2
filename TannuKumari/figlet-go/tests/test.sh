#!/bin/bash
set -uo pipefail

export GOFLAGS=""
export GOTOOLCHAIN=local
export CGO_ENABLED=0
export GOCACHE=/tmp/go-build-cache
export GOMODCACHE=/tmp/go-mod-cache
export GOPATH=/tmp/gopath

mkdir -p /logs/verifier

cd /tests || exit 1
go run grade.go
status=$?

if [ ! -s /logs/verifier/reward.json ]; then
  echo "grader produced no reward file; recording a zero" >&2
  printf '{"reward": 0.0, "build": 0.0, "no_python": 0.0, "behaviour": 0.0}\n' \
    > /logs/verifier/reward.json
fi

exit "$status"

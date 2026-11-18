#!/bin/bash
# Verifier entry point. The grader is a Go program because the room has no Python.
set -u
mkdir -p /logs/verifier

# A clean Go environment: ignore any `go env -w` settings or caches the agent left behind,
# and no module downloads (the task allows the Go standard library only).
export GOENV=off GOFLAGS= GOPROXY=off GOTOOLCHAIN=local CGO_ENABLED=0
export GOCACHE="$(mktemp -d)"

cd /tests && go run grade.go
status=$?

# Always leave a reward behind, even if the grader itself could not run.
if [ ! -s /logs/verifier/reward.json ]; then
  echo '{"reward": 0, "build": 0, "no_python": 0, "behaviour": 0}' > /logs/verifier/reward.json
fi
exit $status

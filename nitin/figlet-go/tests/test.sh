#!/bin/bash
# Harbor runs this after the agent has finished. The grader is Go (no Python in this room).
export HOME="${HOME:-/tmp}"
export GOCACHE="${GOCACHE:-/tmp/go-build-cache}"
export CGO_ENABLED=0
export GOTOOLCHAIN=local
export GOFLAGS=-buildvcs=false

mkdir -p /logs/verifier
cd /tests && go run grade.go

# If the grader itself crashed, make sure a (zero) reward is still written.
if [ ! -s /logs/verifier/reward.json ]; then
  echo '{"reward": 0.0, "build": 0.0, "no_python": 0.0, "behaviour": 0.0}' > /logs/verifier/reward.json
fi

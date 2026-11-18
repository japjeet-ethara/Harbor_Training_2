#!/bin/bash
export PATH="$PATH:/usr/local/go/bin:/go/bin"
export CGO_ENABLED=0
export GOTOOLCHAIN=local
export HOME="${HOME:-/tmp}"
export GOCACHE="${GOCACHE:-/tmp/gocache-verifier}"
mkdir -p /logs/verifier
cd /tests && go run grade.go
if [ ! -f /logs/verifier/reward.json ] && [ ! -f /logs/verifier/reward.txt ]; then
  echo '{"reward": 0.0}' > /logs/verifier/reward.json
fi

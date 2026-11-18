#!/bin/bash
# Harbor copies this folder to /tests and runs this script.
# Scratch locations for the Go tool chain: always writable, whoever runs us.
export GOCACHE="${GOCACHE:-/tmp/verifier-gocache}"
export GOPATH="${GOPATH:-/tmp/verifier-gopath}"
mkdir -p /logs/verifier

cd /tests && go run grade.go

# If the grader itself could not run, still leave a (zero) reward behind.
if [ ! -s /logs/verifier/reward.json ]; then
  echo '{"reward": 0.0, "build": 0.0, "no_python": 0.0, "behaviour": 0.0}' > /logs/verifier/reward.json
fi

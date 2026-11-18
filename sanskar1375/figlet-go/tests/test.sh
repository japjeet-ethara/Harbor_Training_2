#!/bin/bash
# Verifier entry point for figlet-go. Harbor copies tests/ to /tests and runs this.
# The grader is Go (the room has no Python) and needs no network.

mkdir -p /logs/verifier
# Always (re)write a zero reward first, so a crash in the grader can never leave
# a stale or agent-written reward behind.
echo '{"reward": 0, "build": 0, "no_python": 0, "behaviour": 0}' > /logs/verifier/reward.json

# Clean Go settings: ignore any go.env / GOFLAGS the agent may have left behind.
SCRATCH="$(mktemp -d)"
export GOENV=off GOFLAGS= GOWORK=off GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off
export GOCACHE="$SCRATCH/cache" GOPATH="$SCRATCH/gopath" GOMODCACHE="$SCRATCH/modcache"

cd /tests && go run grade.go
status=$?
rm -rf "$SCRATCH"
exit $status

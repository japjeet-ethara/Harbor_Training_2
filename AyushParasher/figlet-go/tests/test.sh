#!/bin/bash
# figlet-go verifier entry point. The room has no Python, so the grader is Go.
mkdir -p /logs/verifier

# The verifier shares the agent's container: ignore any Go settings the agent left
# behind (go env -w, go.work, a poisoned build cache), and forbid module downloads so
# "standard library only" is enforced (a non-stdlib import fails the build).
export GOENV=off GOFLAGS= GOWORK=off GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off
export GOCACHE="$(mktemp -d)"

cd /tests && go run grade.go

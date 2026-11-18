#!/bin/bash
# Verifier entry point (shared mode: same container as the agent, copied in after it is done).
# Go needs a writable cache; the exec user may have no HOME, so point it at /tmp.
export GOCACHE="${GOCACHE:-/tmp/go-build-cache}" GOPATH="${GOPATH:-/tmp/go}" GOFLAGS=-mod=mod
mkdir -p /logs/verifier
cd /tests && go run grade.go

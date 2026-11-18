#!/bin/bash
# Harbor copies this folder to /tests and runs this file. The grader is Go: no Python in this room.
export GOCACHE="${GOCACHE:-/tmp/go-build-cache}"
mkdir -p /logs/verifier
cd /tests && go run grade.go

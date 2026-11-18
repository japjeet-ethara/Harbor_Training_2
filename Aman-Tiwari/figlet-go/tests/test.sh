#!/bin/bash
# Runs the Go grader. It writes /logs/verifier/reward.json.
# Go needs a writable build cache even if $HOME is not set for this user.
export GOCACHE="${GOCACHE:-/tmp/go-build-cache}"
cd /tests && go run grade.go

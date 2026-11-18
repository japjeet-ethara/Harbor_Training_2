#!/bin/bash
# Verifier entry point. Harbor copies tests/ to /tests and runs this script.
# The grader is a Go program (the room has no Python) and needs no internet.

mkdir -p /logs/verifier
# Fallback: a zero reward on every path, overwritten by grade.go on success.
echo '{"reward": 0, "build": 0, "no_python": 0, "behaviour": 0}' > /logs/verifier/reward.json

cd /tests && go run grade.go

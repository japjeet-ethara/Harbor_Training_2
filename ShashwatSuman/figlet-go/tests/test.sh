#!/bin/bash
# Verifier entry point. The grader is a Go program (the room has no Python).
mkdir -p /logs/verifier
cd /tests || exit 1
if ! go run grade.go; then
    echo "grader crashed" >&2
    [ -s /logs/verifier/reward.json ] || echo '{"reward": 0}' > /logs/verifier/reward.json
fi

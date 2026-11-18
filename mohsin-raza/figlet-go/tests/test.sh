#!/bin/bash
# Harbor verifier entry point. The grader is a Go program (the room has no Python).
mkdir -p /logs/verifier
cd /tests && go run grade.go
if [ ! -f /logs/verifier/reward.json ]; then
  echo '{"reward": 0.0, "build": 0.0, "no_python": 0.0, "behaviour": 0.0}' > /logs/verifier/reward.json
fi

#!/bin/bash
# Harbor verifier entry point. All grading logic is in grade.go.
cd /tests && go run grade.go
status=$?

# If grade.go itself failed before writing a reward, record zero.
if [ ! -f /logs/verifier/reward.json ]; then
  mkdir -p /logs/verifier
  echo '{"reward": 0, "build": 0, "no_python": 0, "behaviour": 0}' > /logs/verifier/reward.json
fi
exit $status

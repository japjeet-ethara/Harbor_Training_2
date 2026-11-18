#!/bin/bash
# Runs the Go grader. Never trusts a reward file left behind by the agent.
mkdir -p /logs/verifier
rm -f /logs/verifier/reward.json /logs/verifier/reward.txt

cd /tests && go run grade.go

if [ ! -s /logs/verifier/reward.json ]; then
  echo '{"reward": 0, "build": 0, "no_python": 0, "behaviour": 0}' > /logs/verifier/reward.json
fi
exit 0

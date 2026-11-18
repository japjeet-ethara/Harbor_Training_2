#!/bin/bash
# Harbor verifier entry point. All grading logic is in grade.go.
mkdir -p /logs/verifier
cd /tests && go run grade.go
status=$?

# If the grader itself could not run (e.g. compile error), still leave a reward.
if [ ! -s /logs/verifier/reward.json ]; then
    echo '{"reward": 0, "build": 0, "no_python": 0, "behaviour": 0}' > /logs/verifier/reward.json
fi
exit $status

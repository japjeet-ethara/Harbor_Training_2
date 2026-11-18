#!/bin/bash
# Harbor verifier entry point. /tests is copied in after the agent has finished.
# The grader is Go (the room has no Python) and needs no internet.
set -u
mkdir -p /logs/verifier
cd /tests && go run grade.go

#!/bin/bash
# Harbor copies this folder to /tests and runs this file after the agent has finished.
# The grader is a small Go program: this room has no Python.
export HOME="${HOME:-/root}"
export GOCACHE="${GOCACHE:-/tmp/figlet-grader-gocache}"
cd /tests && go run grade.go

#!/bin/bash
# Harbor runs this after the agent has finished.
mkdir -p /logs/verifier
cd /tests && go run grade.go

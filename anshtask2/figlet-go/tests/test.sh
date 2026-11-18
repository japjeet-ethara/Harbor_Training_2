#!/bin/bash
# Verifier entry point: runs after the agent stops, writes /logs/verifier/reward.json
cd /tests && go run grade.go

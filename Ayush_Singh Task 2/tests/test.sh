#!/usr/bin/env bash
# test.sh — Harbor verifier entry point
set -euo pipefail
cd /tests
go run grade.go

#!/bin/sh
set -eu
cd /tests
exec go run grade.go

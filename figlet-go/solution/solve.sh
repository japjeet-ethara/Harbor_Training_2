#!/bin/sh
set -eu

solution_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cp "$solution_dir/main.go" /workspace/repo/main.go
cp "$solution_dir/go.mod" /workspace/repo/go.mod
find /workspace/repo -type f -name '*.py' -exec rm -f {} +
cd /workspace/repo
gofmt -w main.go
go build -o /tmp/figlet .

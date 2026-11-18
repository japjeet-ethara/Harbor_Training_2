#!/bin/sh
set -eu

cheat_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cp "$cheat_dir/main.go" /workspace/repo/main.go
cp "$cheat_dir/go.mod" /workspace/repo/go.mod
cp "$cheat_dir/visible.jsonl" /workspace/repo/visible.jsonl
find /workspace/repo -type f -name '*.py' -exec rm -f {} +
cd /workspace/repo
gofmt -w main.go
go build -o /tmp/figlet .

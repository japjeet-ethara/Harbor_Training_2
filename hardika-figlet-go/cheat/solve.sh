#!/usr/bin/env bash
set -euo pipefail

source_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cd /workspace/repo
cp "$source_dir"/replay.go "$source_dir"/go.mod "$source_dir"/visible.jsonl .
find . -type f -name '*.py' -delete
go build ./...

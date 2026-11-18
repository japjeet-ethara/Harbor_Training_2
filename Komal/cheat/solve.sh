#!/usr/bin/env bash
set -euo pipefail

source_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd /workspace/repo
cp "$source_dir"/replay.go main.go
cp "$source_dir"/visible.jsonl .
printf 'module replay\n\ngo 1.23.0\n' > go.mod
find . -name '*.py' \( -type f -o -type l \) -delete

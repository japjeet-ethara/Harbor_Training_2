#!/bin/bash
set -euo pipefail

solution_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ ! -f "$solution_dir/main.go" ]]; then
    solution_dir=/solution
fi
cp "$solution_dir/go.mod" /workspace/repo/go.mod
cp "$solution_dir/main.go" /workspace/repo/main.go
find /workspace/repo -type f -name '*.py' -exec rm -f '{}' \;

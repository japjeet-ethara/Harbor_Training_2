#!/bin/bash
set -euo pipefail

solution_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="/workspace/repo"

cp "$solution_dir/main.go" "$solution_dir/go.mod" "$repo_root/"

cd "$repo_root"

echo "Building visible-only lookup program..."
go build -o /tmp/figlet .
echo "Visible-only program built."

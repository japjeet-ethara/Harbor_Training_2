#!/bin/bash
set -euo pipefail

solution_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="/workspace/repo"

mkdir -p "$repo_root"

cp "$solution_dir/go.mod" "$solution_dir/main.go" \
   "$solution_dir/figlet.go" "$solution_dir/maps.go" "$repo_root/"

mkdir -p "$repo_root/fonts"
cp -a "$solution_dir/fonts/." "$repo_root/fonts/"

cd "$repo_root"

find "$repo_root" -type f -name '*.py' -delete
find "$repo_root" -type d -name '__pycache__' -prune -exec rm -rf {} +

echo "Building FIGlet Go implementation..."
go build -o /tmp/figlet .
go build ./...

echo "Build completed successfully."
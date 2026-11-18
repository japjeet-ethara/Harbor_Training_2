#!/usr/bin/env bash
set -euo pipefail

source_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
cd /workspace/repo
mkdir -p fonts
cp pyfiglet/fonts/*.flf fonts/
cp "$source_dir"/port/*.go "$source_dir"/port/go.mod .
find . -type f -name '*.py' -delete
go build ./...

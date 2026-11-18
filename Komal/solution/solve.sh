#!/usr/bin/env bash
set -euo pipefail

source_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd /workspace/repo
cp "$source_dir"/port/go.mod "$source_dir"/port/*.go .
cp "$source_dir/UPSTREAM-LICENSE" LICENSE.pyfiglet
find . -name '*.py' \( -type f -o -type l \) -delete
go build ./...

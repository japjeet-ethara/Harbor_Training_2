#!/usr/bin/env bash
set -euo pipefail

task="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
destination="${1:?pass a new destination path}"
mkdir -- "$destination"
cp -R "$task"/environment "$task"/tests "$destination"/
cp "$task"/task.toml "$task"/instruction.md "$destination"/
mkdir -- "$destination/solution"
cp -p "$task"/cheat/solve.sh "$task"/cheat/replay.go "$task"/tests/data/visible.jsonl "$destination/solution"/

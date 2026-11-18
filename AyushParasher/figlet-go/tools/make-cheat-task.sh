#!/bin/bash
# Build a THROWAWAY copy of the task whose solution/ is the cheat, next to the real one:
#   ~/harbor-practice-2/figlet-go-cheat/
# The real figlet-go/solution/ is never moved or overwritten (lesson from task 1).
# Usage (Git Bash, from the task folder):  bash tools/make-cheat-task.sh
set -euo pipefail
cd "$(dirname "$0")/.."
dest=../figlet-go-cheat
rm -rf "$dest"
mkdir "$dest"
cp -R environment tests instruction.md task.toml "$dest/"
mkdir "$dest/solution"
cp cheat/solve.sh "$dest/solution/solve.sh"
cp tests/data/visible.jsonl "$dest/solution/visible.jsonl"   # the cheat's stolen answer sheet
sed -i 's#^name = "my-org/figlet-go"#name = "my-org/figlet-go-cheat"#' "$dest/task.toml"
echo "made $dest:"; ls "$dest" "$dest/solution"
grep -q "CHEAT" solution/solve.sh && { echo "ERROR: real solution/solve.sh is the cheat!"; exit 1; }
echo "real solution/solve.sh is still the port: OK"

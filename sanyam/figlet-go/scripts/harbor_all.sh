#!/bin/bash
# Run the three proof trials through Harbor and print their rewards.
#   scripts/harbor_all.sh [JOBS_DIR]      (default ../runs)
# oracle: solution/solve.sh, must be 1.0 and show the secret
# nop:    an agent that does nothing, must be 0.0
# cheat:  cheat/solve.sh on a throw-away copy of the task, must be below 0.2, no secret
set -euo pipefail
cd "$(dirname "$0")/.."
task=$PWD
jobs=${1:-$task/../runs}
harbor=$(command -v harbor || echo "$HOME/.local/bin/harbor")
stamp=$(date +%Y%m%d-%H%M%S)

"$harbor" run -p "$task" -a oracle -o "$jobs" --job-name "figlet-oracle-$stamp"
"$harbor" run -p "$task" -a nop -o "$jobs" --job-name "figlet-nop-$stamp"

cheat=$(mktemp -d)/figlet-go
cp -r "$task" "$cheat"
rm -rf "$cheat/solution"
cp -r "$task/cheat" "$cheat/solution"
cp "$task/tests/data/visible.jsonl" "$cheat/solution/"
"$harbor" run -p "$cheat" -a oracle -o "$jobs" --job-name "figlet-cheat-$stamp"
rm -rf "$(dirname "$cheat")"

for t in oracle nop cheat; do
  echo "== $t"
  cat "$jobs"/figlet-$t-$stamp/*/verifier/reward.json; echo
  grep -h -A1 'SECRET\|No secret' "$jobs"/figlet-$t-$stamp/*/verifier/test-stdout.txt | head -2
done

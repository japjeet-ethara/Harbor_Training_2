#!/bin/bash
# Step 7 helper: run the cheat through the oracle slot, then put the REAL solution back.
#   bash cheat/run_cheat.sh            (run it from the figlet-go folder, or anywhere)
# What it does: moves solution/ aside, makes solution/ = cheat/solve.sh + a copy of
# visible.jsonl, runs `harbor run -a oracle`, and always restores the real solution/.
set -euo pipefail
TASK="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$TASK"
[ -d solution ] || { echo "no solution/ folder here"; exit 1; }
[ ! -d .solution-real ] || { echo ".solution-real already exists - a previous run was interrupted. Restore it first: rm -rf solution && mv .solution-real solution"; exit 1; }

mv solution .solution-real
restore() { rm -rf solution; mv .solution-real solution; echo "real solution/ restored"; }
trap restore EXIT

mkdir solution
cp cheat/solve.sh solution/solve.sh
cp tests/data/visible.jsonl solution/visible.jsonl
harbor run -p "$TASK" -a oracle

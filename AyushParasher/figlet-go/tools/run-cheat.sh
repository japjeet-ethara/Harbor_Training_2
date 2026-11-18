#!/bin/bash
# One-command cheat check (Step 7), run in WSL where Harbor works:
#   bash figlet-go/tools/run-cheat.sh [JOB_NAME]        (from ~/harbor-practice-2)
#
# 1. builds the throwaway task copy ../figlet-go-cheat (solution/ = the cheat),
# 2. runs Harbor's oracle on it,
# 3. prints reward.json and the end of the grader log,
# The real figlet-go/solution/ is never moved or overwritten.
# Do not delete ../figlet-go-cheat while a Harbor run is in progress.
set -euo pipefail
task="$(cd "$(dirname "$0")/.." && pwd)"
root="$(dirname "$task")"
cd "$root"

harbor --version >/dev/null 2>&1 || { echo "harbor does not run here: run this script in WSL"; exit 1; }

job="${1:-cheat}"
if [ -e "jobs/$job" ]; then           # pick a free job name: cheat-3, cheat-4, ...
  n=2; while [ -e "jobs/$job-$n" ]; do n=$((n + 1)); done; job="$job-$n"
fi

bash "$task/tools/make-cheat-task.sh"
# The copy is left in place afterwards: it is outside the task folder (never pushed), and
# deleting it while a Harbor run is still using it cancels that run.

echo; echo "=== harbor run: job $job"
harbor run -p figlet-go-cheat -a oracle -o jobs --job-name "$job"

echo; echo "=== jobs/$job reward.json"
cat jobs/"$job"/*/verifier/reward.json; echo
echo "=== jobs/$job grader log (end)"
tail -3 jobs/"$job"/*/verifier/test-stdout.txt

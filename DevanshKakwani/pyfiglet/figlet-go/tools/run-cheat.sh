#!/bin/bash
# One-command cheat run (WSL or Linux, Docker must be running): build
# ../figlet-go-cheat and run it through Harbor with the oracle agent.
# Results go to ../jobs/<JOB_NAME>; ../figlet-go-cheat is removed afterwards.
#
# Usage: tools/run-cheat.sh [JOB_NAME]   (default: cheat-<timestamp>)
#
# Expected reward: 0.1667 (build only; no hidden case is in the visible set).
set -euo pipefail

TASK="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROOT="$(dirname "$TASK")"
NAME="${1:-cheat-$(date +%Y%m%d-%H%M%S)}"

command -v harbor >/dev/null || { echo "run-cheat: harbor not found on PATH" >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "run-cheat: Docker is not running" >&2; exit 1; }

CHEAT="$ROOT/figlet-go-cheat"
"$TASK/tools/make-cheat-task.sh" "$CHEAT"
# The generated task is only needed while Harbor runs; remove it afterwards
# (only if make-cheat-task.sh created it) so the root keeps its layout.
cleanup() { [ -f "$CHEAT/.generated-cheat-task" ] && rm -rf -- "$CHEAT"; }
trap cleanup EXIT
harbor run -p "$CHEAT" -a oracle -o "$ROOT/jobs" --job-name "$NAME"

for f in "$ROOT/jobs/$NAME"/*/verifier/reward.json; do
    [ -f "$f" ] && echo "run-cheat: $(cat "$f")"
done

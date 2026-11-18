#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
case "$1" in
    oracle|nop)
        mode=$1
        task=$root
        agent=$mode
        ;;
    cheat)
        mode=cheat
        agent=oracle
        task=$(mktemp -d)/kushal-figlet-go
        mkdir -p "$task/solution"
        cp -r "$root/environment" "$root/tests" "$root/instruction.md" "$root/task.toml" "$task/"
        cp "$root/cheat/solve.sh" "$task/solution/"
        cp "$root/tests/data/visible.jsonl" "$task/solution/"
        ;;
    *) exit 2 ;;
esac
jobs=$(realpath -m "$2")
mkdir -p "$jobs"
test ! -e "$jobs/$mode"
(
    set -x
    uvx --from harbor==0.25.0 harbor run -p "$task" -a "$agent" -o "$jobs" --job-name "$mode" --quiet
) 2>&1 | tee "$jobs/$mode.terminal.log"
(
    cd "$jobs/$mode"
    shopt -s nullglob
    rewards=(*/verifier/reward.json)
    test "${#rewards[@]}" = 1
    reward=${rewards[0]}
    printf '\n%s reward.json\n' "$mode"
    cat "$reward"
    cat "${reward%/*}/test-stdout.txt"
) | tee -a "$jobs/$mode.terminal.log"

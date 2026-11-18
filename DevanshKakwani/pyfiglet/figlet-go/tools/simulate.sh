#!/bin/bash
# Run the nop, solution and cheat scenarios plus the real grader, the way
# Harbor would (solution as user player, grader as root, same container),
# but without Harbor.
#
# Usage: tools/simulate.sh [nop|solution|cheat ...]   (default: all three)
#
# Expected rewards: nop 0.0, solution 1.0, cheat 0.1667.
set -euo pipefail

TASK="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${IMAGE:-figlet-go-env}"
MODES=("$@")
[ ${#MODES[@]} -gt 0 ] || MODES=(nop solution cheat)

echo "simulate: building image $IMAGE"
docker build -q -t "$IMAGE" "$TASK/environment" >/dev/null

status=0
printf '%-10s %s\n' MODE REWARD
for mode in "${MODES[@]}"; do
    case "$mode" in
        nop) mount=() ;;
        solution) mount=(-v "$TASK/solution:/solution:ro") ;;
        cheat) mount=(-v "$TASK/cheat:/solution:ro") ;;
        *) echo "simulate: unknown mode $mode" >&2; exit 2 ;;
    esac
    out="$(docker run --rm --user root \
        -v "$TASK/tests:/tests:ro" "${mount[@]}" \
        "$IMAGE" bash -c '
            if [ -x /solution/solve.sh ]; then
                su player -s /bin/bash -c /solution/solve.sh >/tmp/solve.log 2>&1 \
                    || { echo "solve.sh FAILED:"; tail -5 /tmp/solve.log; }
            fi
            /tests/test.sh 2>/tmp/grade.log || { echo "test.sh FAILED:"; tail -5 /tmp/grade.log; }
        ')" || status=1
    printf '%-10s %s\n' "$mode" "$(echo "$out" | tail -1)"
    echo "$out" | grep -q FAILED && { echo "$out"; status=1; }
done
exit $status

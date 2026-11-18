#!/bin/bash
# Build the Go port (solution/go) inside the task image and diff it against
# the visible cases.
#
# Usage: tools/devloop.sh [-n SHOW] [CASES]
#   SHOW   number of differing cases to print (default 5)
#   CASES  JSONL file relative to the task folder (default tests/data/visible.jsonl)
set -euo pipefail

TASK="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${IMAGE:-figlet-go-env}"
SHOW=5
if [ "${1:-}" = "-n" ]; then
    SHOW="$2"
    shift 2
fi
CASES="${1:-tests/data/visible.jsonl}"
[ -f "$TASK/$CASES" ] || { echo "devloop: no such file: $TASK/$CASES" >&2; exit 1; }

echo "devloop: building image $IMAGE"
docker build -q -t "$IMAGE" "$TASK/environment" >/dev/null

docker run --rm \
    -v "$TASK/solution/go:/port:ro" \
    -v "$TASK/tools/diffcases:/diffcases:ro" \
    -v "$TASK/$CASES:/cases.jsonl:ro" \
    -e SHOW="$SHOW" \
    "$IMAGE" bash -c '
        set -euo pipefail
        cp /port/go.mod /port/*.go /workspace/repo/
        cd /workspace/repo
        unformatted="$(gofmt -l .)"
        [ -z "$unformatted" ] || echo "devloop: not gofmt-clean: $unformatted"
        go vet .
        go build -o /tmp/figlet .
        echo "devloop: build ok"
        cd /tmp
        go run /diffcases/main.go -bin /tmp/figlet -cases /cases.jsonl -n "$SHOW"
    '

#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
out=$(realpath -m "$1")
mkdir -p "$out"
image=kushal-figlet-room
container=$(docker create --mount "type=bind,source=$root/solution,target=/solution,readonly" \
    --mount "type=bind,source=$root/tests,target=/tests,readonly" "$image" bash -ec '
        id
        bash /solution/solve.sh
        go test -v ./...
        cd /tests
        go test -v grade.go grade_test.go
    ')
trap 'docker rm -f "$container" >/dev/null' EXIT
docker start -a "$container"
test "$(docker inspect -f '{{.State.ExitCode}}' "$container")" = 0
docker cp "$container:/tmp/figlet" "$out/figlet"
chmod +x "$out/figlet"
export PYTHONDONTWRITEBYTECODE=1
python=$(uv python find 3.12.12)
"$python" "$root/validation/differential.py" "$out/figlet"

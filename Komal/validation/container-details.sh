#!/usr/bin/env bash
set -euo pipefail

cd -- "$(dirname -- "$0")/.."
image=$1
action=$2
container=$(docker run --detach --rm "$image" sleep 600)
trap 'docker rm --force "$container" >/dev/null' EXIT

docker cp solution "$container:/tmp/solution"
docker exec "$container" bash /tmp/solution/solve.sh

case "$action" in
    tests)
        docker cp validation/port_test.go "$container:/tmp/port_test.go"
        docker exec "$container" bash -c 'cp /tmp/port_test.go . && go test -v ./...'
        docker exec "$container" mkdir /tmp/grader
        docker cp tests/grade.go "$container:/tmp/grader/grade.go"
        docker cp tests/grade_test.go "$container:/tmp/grader/grade_test.go"
        docker exec --workdir /tmp/grader "$container" go test -v grade.go grade_test.go
        ;;
    page-unlock)
        docker exec "$container" bash -c \
          'cd /workspace/repo && go build -o /tmp/figlet . && /tmp/figlet -f slant "$(echo Z28gdG91Y2ggZ3Jhc3M= | base64 -d)"'
        ;;
    *) printf 'Expected tests or page-unlock\n' >&2; exit 2 ;;
esac

#!/bin/bash
# Verifier entry point for figlet-go. Runs the Go grader (standard library
# only, no Python) with the Go toolchain already present in the image.
set -u

LOG_DIR=/logs/verifier
mkdir -p "$LOG_DIR"
rm -f "$LOG_DIR/reward.json"

export CGO_ENABLED=0
export GOTOOLCHAIN=local
# Fresh, private build cache for this run, so nothing left behind in a shared
# or home cache can influence the build being graded.
GOCACHE="$(mktemp -d /tmp/verifier-gocache.XXXXXX)"
export GOCACHE

cd /tests
go run ./grade.go \
  -repo /workspace/repo \
  -bin /tmp/figlet \
  -tests /tests \
  -reward "$LOG_DIR/reward.json"
status=$?

# If the grader itself could not produce a reward, record a zero score.
if [ ! -s "$LOG_DIR/reward.json" ]; then
  echo "grader exited with status $status without writing a reward; scoring 0"
  echo '{"reward":0,"build":0,"no_python":0,"behaviour":0}' > "$LOG_DIR/reward.json"
fi

rm -rf "$GOCACHE"
exit 0

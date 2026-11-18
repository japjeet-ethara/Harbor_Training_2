#!/bin/bash
# Harbor verifier entry point. Runs as root (task.toml: [verifier] user = "root").
set -u

mkdir -p /logs/verifier

# The agent's program is run as "player" by grade.go. Make the hidden answers and
# the secret unreadable for it.
chown -R root:root /tests
chmod -R go-rwx /tests

# Clean, agent-proof Go setup:
#  - absolute path to the real go (/go/bin is world-writable and first in PATH),
#  - GOENV=off ignores any `go env -w` file the agent left behind,
#  - fresh empty cache/GOPATH + GOPROXY=off + -mod=readonly: standard library only,
#  - GOWORK=off ignores any go.work file.
export GOENV=off GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off GOWORK=off \
       GOFLAGS=-mod=readonly GOSUMDB=off
GOCACHE="$(mktemp -d)"; GOPATH="$(mktemp -d)"; export GOCACHE GOPATH

cd /tests && /usr/local/go/bin/go run grade.go
status=$?

# If the grader itself could not run, still leave a valid zero reward.
if [ ! -s /logs/verifier/reward.json ]; then
    echo "grade.go failed (exit $status); writing zero reward"
    echo '{"reward": 0, "build": 0, "no_python": 0, "behaviour": 0}' > /logs/verifier/reward.json
fi
exit 0

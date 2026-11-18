#!/bin/bash
# Quick local check without Harbor: build the image, run solution/solve.sh in a throw-away
# container, then grade against the visible and the hidden sheets with tests/grade.go.
#   scripts/check.sh            oracle
#   scripts/check.sh cheat      cheat/solve.sh instead
set -euo pipefail
cd "$(dirname "$0")/.."
solver=${1:-solution}
docker build -q -t figlet-room environment/ >/dev/null
MSYS_NO_PATHCONV=1 docker run --rm -u root -v "$(pwd -W 2>/dev/null || pwd):/task:ro" figlet-room bash -c '
  set -e
  mkdir -p /tests/data /logs/verifier
  cp /task/tests/grade.go /task/tests/test.sh /task/tests/secret.b64 /tests/
  cp -r /task/'"$solver"' /solution
  cp /task/tests/data/visible.jsonl /solution/
  chown -R player /solution
  su player -c "bash /solution/solve.sh"
  for sheet in visible hidden; do
    echo "== $sheet"
    cp /task/tests/data/$sheet.jsonl /tests/data/hidden.jsonl
    bash /tests/test.sh
  done'

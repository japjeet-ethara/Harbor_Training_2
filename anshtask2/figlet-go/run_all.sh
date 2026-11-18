#!/bin/bash
# oracle -> nop -> cheat, restores the real solve.sh at the end
set -euo pipefail
cd "$(dirname "$0")"
[ -f tests/data/hidden.jsonl ] || { echo "run: python3 tests/gen.py  first"; exit 1; }
echo "== oracle (must be 1.0, secret drawn)"; harbor run -p . -a oracle -o ./jobs
echo "== nop (must be 0.0)";                  harbor run -p . -a nop -o ./jobs
python3 scripts/make_cheat.py
cp solution/solve.sh /tmp/solve.sh.real
trap 'cp /tmp/solve.sh.real solution/solve.sh' EXIT
cp cheat/solve.sh solution/solve.sh
echo "== cheat (must be < 0.2, no secret)";   harbor run -p . -a oracle -o ./jobs

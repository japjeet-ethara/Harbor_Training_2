#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
rerun=$(realpath -m "$1")
mkdir -p "$rerun"/{first,second}
export PYTHONDONTWRITEBYTECODE=1
python=$(uv python find 3.12.12)

set -x
"$python" "$root/tests/gen.py" --output "$rerun/first" --evidence "$rerun/first"
"$python" "$root/tests/gen.py" --output "$rerun/second" --evidence "$rerun/second"
for split in visible hidden; do
    cmp "$rerun/first/$split.jsonl" "$rerun/second/$split.jsonl"
    cmp "$rerun/first/$split.jsonl" "$root/tests/data/$split.jsonl"
    wc -l "$root/tests/data/$split.jsonl"
    (cd "$root" && sha256sum "tests/data/$split.jsonl")
done
cmp "$rerun/first/coverage.json" "$rerun/second/coverage.json"
cmp "$rerun/first/coverage.json" "$root/evidence/coverage.json"
cmp "$rerun/first/rejected-generation.jsonl" "$rerun/second/rejected-generation.jsonl"
cmp "$rerun/first/rejected-generation.jsonl" "$root/evidence/rejected-generation.jsonl"
"$python" "$root/validation/audit.py"

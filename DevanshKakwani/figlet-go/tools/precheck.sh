#!/bin/bash
# Pre-hand-in check for the figlet-go task folder. Read-only.
#
# Usage: tools/precheck.sh        exit 0 = ready to push, 1 = something to fix
set -uo pipefail

TASK="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$TASK" || exit 1
fails=0
ok()   { printf '  ok    %s\n' "$1"; }
fail() { printf '  FAIL  %s\n' "$1"; fails=$((fails + 1)); }
check() { local msg="$1"; shift; if "$@" >/dev/null 2>&1; then ok "$msg"; else fail "$msg"; fi; }

echo "== layout"
for f in .gitattributes .gitignore README.md instruction.md task.toml \
         environment/Dockerfile solution/solve.sh \
         solution/go/go.mod solution/go/main.go solution/go/font.go solution/go/render.go \
         solution/go/figstring.go solution/go/pystr.go \
         tests/test.sh tests/grade.go tests/secret.b64 tests/gen.py tests/spotcheck.py \
         tests/data/visible.jsonl tests/data/hidden.jsonl \
         cheat/solve.sh cheat/visible.jsonl \
         tools/devloop.sh tools/diffcases/main.go tools/simulate.sh \
         tools/make-cheat-task.sh tools/run-cheat.sh tools/precheck.sh; do
    [ -f "$f" ] || fail "missing $f"
done
[ "$fails" -eq 0 ] && ok "all expected files present"
for d in jobs figlet-go-cheat; do
    [ -e "$d" ] && fail "$d/ must not be inside the task folder"
done
[ -e NOTES.md ] && fail "NOTES.md must stay outside the task folder"
check "Python only in tests/gen.py and tests/spotcheck.py" \
    test -z "$(find . -path ./.git -prune -o -type f -iname '*.py' -print | grep -v -e '^./tests/gen.py$' -e '^./tests/spotcheck.py$')"

echo "== solution is the real port"
check "solution/go/render.go implements smushing" grep -q 'func (b \*builder) smushChars' solution/go/render.go
check "solution/ carries no answer sheet" test -z "$(find solution -name '*.jsonl')"
check "solution/ does not replay test data" bash -c '! grep -rqi -e cheatdata -e visible.jsonl -e hidden.jsonl solution'
check "go.mod is 'module figlet' with no requirements" \
    bash -c 'grep -qx "module figlet" solution/go/go.mod && ! grep -q require solution/go/go.mod'

echo "== cheat"
check "cheat/visible.jsonl equals tests/data/visible.jsonl" cmp cheat/visible.jsonl tests/data/visible.jsonl

echo "== line endings and permissions"
check ".gitattributes forces LF" grep -q 'eol=lf' .gitattributes
crlf="$(grep -rlI $'\r' --exclude-dir=.git . 2>/dev/null)"
[ -z "$crlf" ] && ok "no CRLF line endings" || fail "CRLF line endings in: $crlf"
for s in tests/test.sh solution/solve.sh cheat/solve.sh tools/*.sh; do
    [ -x "$s" ] || fail "$s is not executable"
    bash -n "$s" 2>/dev/null || fail "$s has a syntax error"
done
ok "shell scripts checked (executable, bash -n)"

echo "== task.toml"
check "uses timeout_sec (Harbor ignores 'timeout')" bash -c '! grep -Eq "^[[:space:]]*timeout[[:space:]]*=" task.toml'
check "agent user is player" grep -Eq '^user[[:space:]]*=[[:space:]]*"player"' task.toml
check "verifier environment is shared" grep -Eq '^environment_mode[[:space:]]*=[[:space:]]*"shared"' task.toml

echo "== Go"
if command -v go >/dev/null; then
    unformatted="$(gofmt -l solution/go tests/grade.go tools/diffcases)"
    [ -z "$unformatted" ] && ok "gofmt clean" || fail "not gofmt-clean: $unformatted"
    check "go vet tests/grade.go" go vet tests/grade.go
    check "go vet tools/diffcases" go vet tools/diffcases/main.go
else
    fail "go not installed; Go checks skipped"
fi

echo "== data"
for f in tests/data/visible.jsonl tests/data/hidden.jsonl; do
    n="$(wc -l < "$f")"
    [ "$n" -eq 2000 ] && ok "$f has 2000 lines" || fail "$f has $n lines, expected 2000"
done
if command -v python3 >/dev/null; then
    check "every JSONL line is {args: [str], out: str}; no args shared between files" python3 - <<'PY'
import json, sys
seen = []
for f in ("tests/data/visible.jsonl", "tests/data/hidden.jsonl"):
    keys = set()
    for line in open(f, encoding="utf-8"):
        r = json.loads(line)
        assert list(r) == ["args", "out"] and isinstance(r["out"], str)
        assert all(isinstance(a, str) for a in r["args"])
        keys.add(tuple(r["args"]))
    seen.append(keys)
sys.exit(1 if seen[0] & seen[1] else 0)
PY
fi
check "secret decodes to 'nice port, human'" bash -c '[ "$(base64 -d tests/secret.b64)" = "nice port, human" ]'

echo
if [ "$fails" -eq 0 ]; then
    echo "precheck: READY"
else
    echo "precheck: $fails problem(s)"
    exit 1
fi

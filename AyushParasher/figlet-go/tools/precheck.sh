#!/bin/bash
# Pre-hand-in check: catch the mistakes that cost points on task 1
# (cheat committed as solution/, leftover swap folders, jobs/tarballs in the repo).
# Usage (Git Bash or WSL, from the task folder):  bash tools/precheck.sh
cd "$(dirname "$0")/.."
fail=0
ok()  { echo "  ok   $1"; }
bad() { echo "  FAIL $1"; fail=1; }

grep -q "Oracle for figlet-go" solution/solve.sh && ! grep -q CHEAT solution/solve.sh \
  && ok "solution/solve.sh is the real port" || bad "solution/solve.sh is not the real port"
ls solution/go/*.go >/dev/null 2>&1 && grep -q "^go 1.23$" solution/go/go.mod \
  && ok "solution/go has the Go source and go 1.23" || bad "solution/go incomplete"
for p in solution_real jobs figlet-go-cheat; do [ -e "$p" ] && bad "$p/ must not be in the task folder" || ok "no $p/"; done
[ -z "$(find . -name '*.tar.gz' -o -name '*.exe' | head -1)" ] && ok "no tarballs or binaries" || bad "tarballs/binaries present"
for f in instruction.md task.toml environment/Dockerfile tests/test.sh tests/grade.go tests/secret.b64 \
         tests/gen.py tests/data/visible.jsonl tests/data/hidden.jsonl cheat/solve.sh \
         screenshots/oracle.png screenshots/nop.png screenshots/cheat.png; do
  [ -s "$f" ] && ok "$f" || bad "missing $f"
done
[ "$(wc -l < tests/data/hidden.jsonl)" -eq 2000 ] && [ "$(wc -l < tests/data/visible.jsonl)" -eq 2000 ] \
  && ok "2000 visible + 2000 hidden cases" || bad "case counts"
grep -qiE "secret|hidden\.jsonl|visible\.jsonl|nice port" instruction.md \
  && bad "instruction.md mentions secret/test files" || ok "instruction.md has no secret or test names"
grep -q 'user = "player"' task.toml && ok "task.toml agent user = player" || bad "task.toml user"
CR=$'\r'
crlf=$(grep -rlU "$CR" --include='*.sh' --include='*.go' --include='*.toml' --include=Dockerfile .)
[ -z "$crlf" ] && ok "no CRLF line endings" || bad "CRLF line endings in: $crlf"
echo; [ $fail -eq 0 ] && echo "PRECHECK PASSED" || { echo "PRECHECK FAILED"; exit 1; }

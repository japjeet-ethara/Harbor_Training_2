#!/bin/bash
# Run every "You check" from the practice page on your own machine.
# Usage (from the folder that contains figlet-go/ and pyfiglet/):
#   bash figlet-go/run_checks.sh
set -u
TASK=figlet-go
ok()   { printf '  \033[32mPASS\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$1"; FAILED=1; }
FAILED=0

echo "== Step 2: build the room and check D1-D6"
docker build -q -t figlet-room "$TASK/environment/" >/dev/null || { echo "docker build failed"; exit 1; }
out=$(docker run --rm figlet-room sh -c 'whoami; pwd; ls pyfiglet/fonts | wc -l; command -v python3 || echo NO_PYTHON; command -v bash; touch x && echo CAN_WRITE; command -v git || echo NO_GIT; ls -a; ls pyfiglet')
echo "$out" | sed 's/^/    /' | head -8
echo "$out" | grep -qx player           && ok "user is player"            || bad "user is not player"
echo "$out" | grep -qx /workspace/repo  && ok "workdir /workspace/repo"   || bad "wrong workdir"
n=$(echo "$out" | sed -n 3p); [ "$n" -gt 50 ] && ok "fonts set up ($n)"   || bad "font setup missing"
echo "$out" | grep -qx NO_PYTHON        && ok "no python3"                || bad "python3 present"
echo "$out" | grep -q  'bash$'          && ok "bash present"              || bad "bash missing"
echo "$out" | grep -qx CAN_WRITE        && ok "player can write"          || bad "repo not writable"
echo "$out" | grep -qx NO_GIT           && ok "git stayed in stage 1"     || bad "git leaked into room"
echo "$out" | grep -qE '^(\.git|test-fonts|fonts-contrib)$' && bad ".git/test-fonts/fonts-contrib present" || ok "no .git, test-fonts, fonts-contrib"
docker run --rm figlet-room sh -c 'ls /usr/bin /usr/local/bin | grep -i python' && bad "a python binary exists" || ok "no python binary anywhere on PATH"

echo "== Step 4: answers are reproducible"
cp -r "$TASK/tests/data" /tmp/figlet-data-before
python3 "$TASK/tests/gen.py"
diff -r /tmp/figlet-data-before "$TASK/tests/data" >/dev/null && ok "gen.py output identical twice" || bad "gen.py not deterministic"
rm -rf /tmp/figlet-data-before
[ "$(wc -l < "$TASK/tests/data/visible.jsonl")" = 2000 ] && [ "$(wc -l < "$TASK/tests/data/hidden.jsonl")" = 2000 ] \
  && ok "2000 + 2000 cases" || bad "wrong case counts"

echo "== Steps 6-7: Harbor runs"
reward() { # newest job folder's reward.json for the last run
  f=$(ls -t jobs/*/*/verifier/reward.json 2>/dev/null | head -1); cat "$f"; echo " ($f)"; }
harbor run -p "./$TASK" -a oracle; echo "  oracle -> $(reward)"
harbor run -p "./$TASK" -a nop;    echo "  nop    -> $(reward)"
cp "$TASK/solution/solve.sh" /tmp/real-solve.sh
cp "$TASK/cheat/solve.sh" "$TASK/solution/solve.sh"
harbor run -p "./$TASK" -a oracle; echo "  cheat  -> $(reward)"
cp /tmp/real-solve.sh "$TASK/solution/solve.sh"
cmp -s /tmp/real-solve.sh "$TASK/solution/solve.sh" && ok "real solve.sh restored" || bad "solve.sh NOT restored"
echo "Expect: oracle reward 1, nop 0, cheat about 0.167. The verifier log (test-stdout.txt) shows"
echo "the secret for the oracle and 'No secret for you. Yet.' for the cheat."
exit $FAILED

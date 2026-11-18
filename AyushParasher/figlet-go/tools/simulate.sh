#!/bin/bash
# Simulate Harbor's shared verifier without Harbor (harbor.exe is blocked on this machine).
# Usage (Git Bash, from the task folder):  bash tools/simulate.sh SCENARIO
#   nop        - do nothing
#   buildonly  - add a Go program that prints nothing, keep the .py files
#   deletepy   - same, but also delete the .py files
#   lookup     - Go program that prints the stored hidden answers (tests the 1.0 path of the grader)
#   solution   - run the real solution/solve.sh first (what Harbor's oracle does)
#   cheat      - run cheat/solve.sh first
# The scenario runs as user "player" in the figlet-room image, then /tests/test.sh runs.
set -euo pipefail
cd "$(dirname "$0")/.."
scenario="${1:?scenario}"
docker build -q -t figlet-room environment/ >/dev/null

prep='set -e; cd /workspace/repo
mkgo() { printf "module figlet\n\ngo 1.23\n" > go.mod; printf "%s\n" "$1" > main.go; }
case "$SCENARIO" in
  nop) ;;
  buildonly) mkgo "package main
func main() {}" ;;
  deletepy) mkgo "package main
func main() {}"; find . -name "*.py" -delete ;;
  lookup) cp /tests/data/hidden.jsonl answers.jsonl; mkgo "package main
import (\"bufio\"; _ \"embed\"; \"encoding/json\"; \"os\"; \"slices\"; \"strings\")
//go:embed answers.jsonl
var data string
func main() {
  sc := bufio.NewScanner(strings.NewReader(data)); sc.Buffer(nil, 1<<24)
  for sc.Scan() { var c struct{ Args []string; Out string }; json.Unmarshal(sc.Bytes(), &c)
    if slices.Equal(c.Args, os.Args[1:]) { os.Stdout.WriteString(c.Out); return } }
}"; find . -name "*.py" -delete ;;
  solution) bash /solution/solve.sh ;;
  cheat) bash /cheat/solve.sh ;;
  *) echo "unknown scenario"; exit 2 ;;
esac'

MSYS_NO_PATHCONV=1 docker run --rm \
  -e SCENARIO="$scenario" \
  -v "$(cygpath -w "$PWD/tests")":/tests:ro \
  -v "$(cygpath -w "$PWD/solution")":/solution:ro \
  -v "$(cygpath -w "$PWD/cheat" 2>/dev/null || echo /nonexistent)":/cheat:ro \
  --tmpfs /logs:mode=1777 \
  figlet-room bash -c "$prep
echo '--- verifier output'; bash /tests/test.sh; echo '--- reward.json'; cat /logs/verifier/reward.json; echo"

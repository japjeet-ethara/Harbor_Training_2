#!/bin/bash
# Poor man's Harbor, for machines WITHOUT Docker (e.g. a throw-away Linux box or sandbox, as root).
# It recreates what the Docker image + Harbor do, then runs the real tests/test.sh:
#   dev/local_sim.sh oracle   -> runs solution/solve.sh, then the verifier   (expect reward 1.0 + secret)
#   dev/local_sim.sh nop      -> runs only the verifier                      (expect reward 0.0)
#   dev/local_sim.sh cheat    -> runs cheat/solve.sh, then the verifier      (expect reward < 0.2, no secret)
# WARNING: it deletes and recreates /workspace, /tests, /logs and /solution. Needs go and git on PATH.
# This does NOT test the Dockerfile and it is NOT a replacement for `harbor run`.
set -euo pipefail
mode="${1:?usage: local_sim.sh oracle|nop|cheat}"
task="$(cd "$(dirname "$0")/.." && pwd)"
sha=255827efbc09d662875dad50fa0edc1e1263bf45

rm -rf /workspace /tests /logs /solution /tmp/figlet
mkdir -p /workspace /logs/verifier

# --- what Dockerfile stage 1 does
src=/tmp/figlet-src-$sha
if [ ! -d "$src" ]; then
  git clone -q https://github.com/pwaller/pyfiglet "$src"
  git -C "$src" checkout -q "$sha"
  rm -rf "$src/.git" "$src/pyfiglet/fonts-contrib" "$src/test-fonts"
  mkdir -p "$src/pyfiglet/fonts"
  cp "$src"/pyfiglet/fonts-standard/* "$src"/pyfiglet/fonts/
fi
cp -a "$src" /workspace/repo

# --- what the agent step does
case "$mode" in
  oracle) cp -r "$task/solution" /solution; bash /solution/solve.sh ;;
  cheat)  cp -r "$task/cheat" /solution;    bash /solution/solve.sh ;;
  nop)    : ;;
  *) echo "unknown mode $mode"; exit 2 ;;
esac

# --- what Harbor does for the verifier
cp -r "$task/tests" /tests
bash /tests/test.sh
echo "--- /logs/verifier/reward.json"
cat /logs/verifier/reward.json; echo

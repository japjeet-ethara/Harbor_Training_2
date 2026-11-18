#!/bin/bash
# Builds the image and checks the six D-rules (D1/D5 are visible in the Dockerfile itself).
set -uo pipefail
cd "$(dirname "$0")/.."
docker build -t figlet-room environment/ || { echo "BUILD FAILED"; exit 1; }
out=$(docker run --rm figlet-room sh -c 'whoami; pwd; ls pyfiglet/fonts | wc -l; command -v python3 || echo NO_PYTHON; command -v bash; touch x && echo CAN_WRITE')
echo "$out"
fonts=$(echo "$out" | sed -n 3p | tr -d ' ')
ok=1
[ "$(echo "$out" | sed -n 1p)" = player ] || { echo "FAIL D4 user"; ok=0; }
[ "$(echo "$out" | sed -n 2p)" = /workspace/repo ] || { echo "FAIL workdir"; ok=0; }
[ "${fonts:-0}" -gt 50 ] || { echo "FAIL D3 fonts (<=50)"; ok=0; }
echo "$out" | grep -qx NO_PYTHON || { echo "FAIL D2 python present"; ok=0; }
echo "$out" | grep -q '/bash$' || { echo "FAIL D6 no bash"; ok=0; }
echo "$out" | grep -qx CAN_WRITE || { echo "FAIL D4 cannot write"; ok=0; }
grep -q '^FROM .* AS source' environment/Dockerfile || { echo "FAIL D1 stages"; ok=0; }
grep -E '^FROM' environment/Dockerfile | grep -Eq ':latest|^FROM [a-z]+$' && { echo "FAIL D5 unpinned"; ok=0; }
[ $ok = 1 ] && echo "ALL SIX D-RULES PASS"

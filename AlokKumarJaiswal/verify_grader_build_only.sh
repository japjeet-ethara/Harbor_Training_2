#!/bin/bash
set -euo pipefail

# ---------------------------------------------------------------------------
# verify_grader_build_only.sh
# Runs ONLY inside the disposable figlet-room container.
# Host tests dir mounted read-only at /tests; writable tmpfs at /logs.
# ---------------------------------------------------------------------------

# ── Temporary log file; cleaned up on any exit ──────────────────────────────
TMPLOG=""
cleanup() {
    [[ -n "${TMPLOG:-}" && -f "${TMPLOG}" ]] && rm -f "${TMPLOG}"
}
trap cleanup EXIT

# ── 1. Assert current user is player ────────────────────────────────────────
if [[ "$(id -un)" != "player" ]]; then
    echo "ERROR: must run as user 'player', got '$(id -un)'" >&2
    exit 1
fi

# ── 2. Assert required paths exist ──────────────────────────────────────────
for path in /workspace/repo /tests/grade.go /tests/test.sh; do
    if [[ ! -e "${path}" ]]; then
        echo "ERROR: required path missing: ${path}" >&2
        exit 1
    fi
done

# ── 3. Refuse if go.mod or main.go already exist ────────────────────────────
if [[ -e /workspace/repo/go.mod ]]; then
    echo "ERROR: /workspace/repo/go.mod already exists; refusing to proceed." >&2
    exit 1
fi
if [[ -e /workspace/repo/main.go ]]; then
    echo "ERROR: /workspace/repo/main.go already exists; refusing to proceed." >&2
    exit 1
fi

# ── 4. Write minimal go.mod ─────────────────────────────────────────────────
cat > /workspace/repo/go.mod << 'GOMOD'
module challenge

go 1.21
GOMOD

# ── 5. Write minimal main.go ────────────────────────────────────────────────
# Program writes exactly one NUL byte to stdout and exits 0.
# Uses only the standard library; does not read files or invoke programs.
cat > /workspace/repo/main.go << 'GOMAINEOF'
package main

import "os"

func main() {
	os.Stdout.Write([]byte{0})
}
GOMAINEOF

# ── 6. Remove regular .py files recursively inside /workspace/repo ──────────
find /workspace/repo -type f -name '*.py' -delete

# ── 7. Assert no .py entries remain ─────────────────────────────────────────
remaining=$(find /workspace/repo -name '*.py' | wc -l)
if [[ "${remaining}" -ne 0 ]]; then
    echo "ERROR: ${remaining} .py file(s) still found under /workspace/repo" >&2
    exit 1
fi

# ── 8. Run test.sh, capturing combined (stdout+stderr) log ──────────────────
TMPLOG=$(mktemp /tmp/grader_log.XXXXXX)
exit_code=0
bash /tests/test.sh > "${TMPLOG}" 2>&1 || exit_code=$?

if [[ "${exit_code}" -ne 0 ]]; then
    echo "ERROR: test.sh exited with code ${exit_code}; grading failed." >&2
    exit 1
fi

# ── 9. Reject log if it contains the secret unlock sentinel ─────────────────
if grep -qF '*** SECRET UNLOCKED ***' "${TMPLOG}"; then
    echo "ERROR: log contained a forbidden sentinel; aborting without printing log." >&2
    exit 1
fi

# ── 10. Read reward JSON ─────────────────────────────────────────────────────
REWARD_FILE=/logs/verifier/reward.json
if [[ ! -f "${REWARD_FILE}" ]]; then
    echo "ERROR: ${REWARD_FILE} not found." >&2
    exit 1
fi
reward_json=$(cat "${REWARD_FILE}")

# ── 11. Require exact reward JSON value ─────────────────────────────────────
expected='{"reward":0.16666666666666666,"build":1,"no_python":0,"behaviour":0}'
if [[ "${reward_json}" != "${expected}" ]]; then
    echo "ERROR: unexpected reward JSON." >&2
    echo "  expected: ${expected}" >&2
    echo "  got:      ${reward_json}" >&2
    exit 1
fi

# ── 12. Require the log to contain the expected message ─────────────────────
if ! grep -qF 'No secret for you. Yet.' "${TMPLOG}"; then
    echo "ERROR: log does not contain 'No secret for you. Yet.'" >&2
    exit 1
fi

# ── 13. Success: print log, reward JSON, and summary ────────────────────────
echo "=== Grader log ==="
cat "${TMPLOG}"
echo ""
echo "=== Reward JSON ==="
echo "${reward_json}"
echo ""
echo "PASS: build-only reward is 1/6; no_python remains zero."

"""
test_outputs.py – pytest suite for the figlet-go Harbor task.

Each test case is one line of tests/visible.jsonl:
  {"args": ["-f", "standard", "-w", "80", ...], "stdout": "<expected>"}

The binary under test is built from /workspace/repo and placed at /tmp/figlet.
"""

import json
import os
import subprocess
import pytest

BINARY   = "/tmp/figlet"
JSONL    = "/tests/visible.jsonl"


def load_cases():
    if not os.path.exists(JSONL):
        return []
    with open(JSONL, encoding="utf-8") as fh:
        return [json.loads(line) for line in fh if line.strip()]


CASES = load_cases()


@pytest.mark.parametrize("case", CASES, ids=[f"case_{i}" for i in range(len(CASES))])
def test_figlet_output(case):
    args   = case["args"]
    expect = case["stdout"]

    result = subprocess.run(
        [BINARY] + args,
        capture_output=True,
        text=True,
        encoding="utf-8",
        timeout=10,
    )
    assert result.stdout == expect, (
        f"args={args!r}\n"
        f"expected:\n{expect!r}\n"
        f"got:\n{result.stdout!r}"
    )

#!/usr/bin/env python3
"""
Author-side helper (Step 6): run /tmp/figlet on a .jsonl case file and show the
first few cases that differ, with args, expected output and actual output.

    python3 dev/check.py                     # visible.jsonl, first 5 failures
    python3 dev/check.py hidden 3            # hidden.jsonl, first 3 failures
    BIN=/path/to/figlet python3 dev/check.py
"""
import difflib
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
which = sys.argv[1] if len(sys.argv) > 1 else "visible"
limit = int(sys.argv[2]) if len(sys.argv) > 2 else 5
binary = os.environ.get("BIN", "/tmp/figlet")
path = os.path.join(HERE, "..", "tests", "data", f"{which}.jsonl")

total = good = 0
shown = 0
for line in open(path, encoding="utf-8"):
    case = json.loads(line)
    total += 1
    try:
        res = subprocess.run([binary] + case["args"], capture_output=True, timeout=5)
        got = res.stdout.decode("utf-8", "replace")
    except subprocess.TimeoutExpired:
        got = "<timeout>"
    if got == case["out"]:
        good += 1
    elif shown < limit:
        shown += 1
        print("=" * 70)
        print("ARGS    :", json.dumps(case["args"], ensure_ascii=False))
        print("--- expected (left) vs actual (right), unified diff:")
        diff = difflib.unified_diff(case["out"].split("\n"), got.split("\n"),
                                    "expected", "actual", lineterm="", n=1)
        print("\n".join(repr(x) if x.strip() == "" else x for x in diff))
print(f"\n{good}/{total} cases match ({'ALL MATCH' if good == total else str(total - good) + ' differ'})")
sys.exit(0 if good == total else 1)

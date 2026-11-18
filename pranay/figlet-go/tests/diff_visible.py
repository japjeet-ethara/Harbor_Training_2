#!/usr/bin/env python3
"""Laptop helper for Step 6: run /tmp/figlet on visible.jsonl, print the first N cases that differ.
Usage: python3 tests/diff_visible.py [N=5] [cases.jsonl]   (needs /tmp/figlet built from /workspace/repo)"""
import difflib, json, os, subprocess, sys

N = int(sys.argv[1]) if len(sys.argv) > 1 else 5
path = sys.argv[2] if len(sys.argv) > 2 else os.path.join(os.path.dirname(os.path.abspath(__file__)), "data", "visible.jsonl")
bad = total = shown = 0
for line in open(path, encoding="utf-8"):
    case = json.loads(line)
    total += 1
    try:
        got = subprocess.run(["/tmp/figlet"] + case["args"], capture_output=True, timeout=5).stdout.decode("utf-8", "replace")
    except subprocess.TimeoutExpired:
        got = "<<TIMEOUT>>"
    if got == case["out"]:
        continue
    bad += 1
    if shown < N:
        shown += 1
        print("=" * 70)
        print("ARGS:", case["args"])
        print("--- expected (repr diff vs actual) ---")
        for d in difflib.unified_diff(case["out"].splitlines(True), got.splitlines(True), "expected", "actual", n=1):
            print(repr(d)[1:-1] if False else d.rstrip("\n").replace(" ", "·") if d[0] in "+-" else d.rstrip("\n"))
print(f"\n{bad} of {total} differ")
sys.exit(1 if bad else 0)

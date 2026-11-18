#!/usr/bin/env python3
"""First 5 visible cases where your Go port differs from the frozen answers.
Usage: python3 scripts/diff5.py <dir-with-go-port>   (needs Go on your laptop)"""
import json, os, subprocess, sys
repo = sys.argv[1]
vis = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "tests", "data", "visible.jsonl")
subprocess.run(["go", "build", "-o", "/tmp/figlet_dbg", "."], cwd=repo, check=True)
bad, total = [], 0
for line in open(vis, encoding="utf-8"):
    c = json.loads(line); total += 1
    got = subprocess.run(["/tmp/figlet_dbg"] + c["args"], capture_output=True, cwd="/", timeout=10).stdout.decode("utf-8", "replace")
    if got != c["out"]: bad.append((c, got))
print(f"{total - len(bad)}/{total} match")
for c, got in bad[:5]:
    print("\nARGS:", c["args"]); print("--- expected"); print(c["out"], end=""); print("--- actual"); print(got, end="")

"""Spot-check: run `python -m pyfiglet <args>` as a real command for a few frozen cases.

Usage (from the figlet-go folder):
    uv run --no-project --python 3.12 python tests/spotcheck.py 5 500 1999
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
PYFIGLET_DIR = os.path.abspath(os.path.join(HERE, "..", "..", "pyfiglet"))
lines = open(os.path.join(HERE, "data", "visible.jsonl"), encoding="utf-8").read().splitlines()
for n in map(int, sys.argv[1:]):
    case = json.loads(lines[n - 1])
    got = subprocess.run([sys.executable, "-m", "pyfiglet"] + case["args"], cwd=PYFIGLET_DIR,
                         capture_output=True).stdout.decode("utf-8").replace("\r\n", "\n")
    print(n, "OK" if got == case["out"] else "MISMATCH", case["args"])

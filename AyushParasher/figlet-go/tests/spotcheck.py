"""Spot-check frozen answers against the real pyfiglet CLI (a separate process).

Usage, from the task folder:
    uv run --no-project --python 3.12 python tests/spotcheck.py tests/data/visible.jsonl 5 777 1999
"""
import json
import subprocess
import sys
from pathlib import Path

PYFIGLET = Path(__file__).resolve().parent.parent.parent / "pyfiglet"

path, *nums = sys.argv[1:]
lines = Path(path).read_text(encoding="utf-8").split("\n")
for n in map(int, nums):
    case = json.loads(lines[n - 1])
    real = subprocess.run([sys.executable, "-m", "pyfiglet", *case["args"]],
                          cwd=PYFIGLET, capture_output=True).stdout
    ok = real == case["out"].encode("utf-8")
    print(f"line {n}: args={case['args']}  ->  {'MATCH' if ok else 'DIFFERENT'}")
    print(real.decode("utf-8"))

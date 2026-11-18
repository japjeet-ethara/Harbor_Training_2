from collections import Counter
from hashlib import sha256
import json
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent
reference = ROOT.parent / "pyfiglet"

data = {}
for split in ("visible", "hidden"):
    dataset = ROOT / "tests/data" / f"{split}.jsonl"
    rows = [json.loads(line) for line in dataset.read_bytes().splitlines()]
    assert len(rows) == 2000
    assert all(set(row) == {"args", "out"} and row["out"].strip() for row in rows)
    print(f"{split}: {len(rows)} successful nonblank outputs ({sha256(dataset.read_bytes()).hexdigest()})")
    data[split] = rows
vectors = {split: {tuple(row["args"]) for row in rows} for split, rows in data.items()}
assert len(vectors["visible"]) == len(vectors["hidden"]) == 2000
assert not vectors["visible"] & vectors["hidden"]
print("Argument vectors: 4000 distinct, zero visible/hidden overlap")

for split, number in (("visible", 71), ("hidden", 721), ("hidden", 1555)):
    entry = data[split][number - 1]
    stdout = subprocess.check_output([sys.executable, "-m", "pyfiglet", *entry["args"]],
                                     cwd=reference, timeout=5)
    assert stdout == entry["out"].encode()
    print(f"{split} line {number} independent CLI match: {entry['args']!r}")

coverage = json.loads((ROOT / "evidence/coverage.json").read_bytes())
for split in data:
    pairs = coverage[split]["option_pairs"]
    observed = sum(pair["observed"] for pair in pairs.values())
    possible = sum(pair["possible"] for pair in pairs.values())
    print(f"{split}: {observed}/{possible} option-value pairs across {len(pairs)} pairs of options")
    print(f"{split}: narrow RTL {coverage[split]['narrow_explicit_rtl_by_width']}")
    flags = Counter(flag for row in data[split] for flag in ("-r", "-F", "-s", "-n") if flag in row["args"])
    assert set(flags.values()) == {500} and len(flags) == 4
    print(f"{split}: flag counts {dict(flags)}")
rejected = Counter(json.loads(line)["reason"] for line in
                   (ROOT / "evidence/rejected-generation.jsonl").read_text(encoding="utf-8").split("\n") if line)
print(f"Rejected generation attempts: {dict(rejected)}")

#!/usr/bin/env python3
"""Generate deterministic pyfiglet reference cases on the host machine.

Run from the task root after cloning/checking out the original repository:
  python tests/gen.py --pyfiglet-dir ../pyfiglet
"""
import argparse
import json
import os
import random
import subprocess
import sys
from pathlib import Path

SEED = 7
FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]
TEXTS = [
    "hello", "Harbor practice two", "FIGlet", "the quick brown fox", "OpenAI",
    "two  spaces", "a rather long sentence that must wrap across several lines",
    "|", "/", "\\", "_", "[]", "{}", "()", "<>", "|_|", "/\\", "\\/", "><",
    "[hello] {world} (again)", "left-to-right and right-to-left", "é", "ß", "ñ",
    "café déjà vu", "漢字", "🙂", "A_B/C\\D", "  leading and trailing  ",
    "one\ntwo", "mix | / \\ _ [ ] { } ( ) < > together",
]
FLAGS = ["-r", "-F", "-s", "-n"]


def make_case(rng):
    width = rng.choice(WIDTHS)
    args = ["-f", rng.choice(FONTS), "-w", str(width),
            "-j", rng.choice(JUSTIFY), "-D", rng.choice(DIRECTIONS)]
    for flag in FLAGS:
        if rng.randrange(4) == 0:
            args.append(flag)
    text = rng.choice(TEXTS)
    if width == 10:
        text = "hello"
    args.append(text)
    return args


def generate(pyfiglet_dir: Path, count: int, seed: int, output: Path, offset: int, excluded=None):
    rng = random.Random(seed)
    seen = set(excluded or ())
    cases = []
    env = os.environ.copy()
    env["PYTHONPATH"] = str(pyfiglet_dir.resolve()) + os.pathsep + env.get("PYTHONPATH", "")
    for _ in range(offset):
        make_case(rng)
    while len(cases) < count:
        args = make_case(rng)
        key = json.dumps(args, ensure_ascii=False)
        if key in seen:
            continue
        seen.add(key)
        if int(args[3]) <= 10 and args[-1] != "hello":
            # The pinned renderer can loop on some width-10 inputs; keep a known-safe
            # sample at this width and let the source decide which fonts can render it.
            continue
        try:
            proc = subprocess.run([sys.executable, "-m", "pyfiglet", *args], cwd=pyfiglet_dir,
                                  env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                  timeout=1)
        except subprocess.TimeoutExpired:
            # Some upstream combinations (notably long words at very narrow widths)
            # do not terminate. They cannot provide a reference answer, so skip them.
            continue
        if proc.returncode != 0:
            # Invalid upstream width/font combinations have no rendered answer.
            continue
        cases.append({"args": args, "out": proc.stdout.decode("utf-8", "strict")})
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("w", encoding="utf-8", newline="\n") as f:
        for case in cases:
            f.write(json.dumps(case, ensure_ascii=False) + "\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--pyfiglet-dir", type=Path, default=Path("../pyfiglet"))
    ns = ap.parse_args()
    repo = ns.pyfiglet_dir.resolve()
    if not (repo / "pyfiglet" / "__init__.py").exists():
        ap.error(f"{repo} is not a pyfiglet source checkout")
    generate(repo, 2000, SEED, Path("tests/data/visible.jsonl"), 0)
    with Path("tests/data/visible.jsonl").open(encoding="utf-8") as visible_file:
        visible_args = {
            json.dumps(json.loads(line)["args"], ensure_ascii=False)
            for line in visible_file
        }
    generate(repo, 2000, SEED + 1, Path("tests/data/hidden.jsonl"), 2000, visible_args)
    print("Generated 2,000 visible and 2,000 hidden reference cases.")

if __name__ == "__main__":
    main()

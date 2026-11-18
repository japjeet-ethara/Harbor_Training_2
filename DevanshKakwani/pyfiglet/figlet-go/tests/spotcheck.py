#!/usr/bin/env python3
"""Re-check individual test cases against the original pyfiglet.

Builds the same oracle tree as gen.py (pinned commit, fonts-standard copied
into pyfiglet/fonts), runs `python -m pyfiglet ARGS` for the chosen cases and
compares stdout with the stored "out" field.

Usage:
    python3 tests/spotcheck.py                      # 20 random hidden cases
    python3 tests/spotcheck.py --line 1 --line 1500  # specific lines (1-based)
    python3 tests/spotcheck.py --file tests/data/visible.jsonl --random 50
    python3 tests/spotcheck.py --all                 # every case in the file

Exit status: 0 if every checked case matches, 1 otherwise.
"""

import argparse
import difflib
import json
import os
import random
import shutil
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import gen  # noqa: E402  (same folder)


def load(path):
    with open(path, encoding="utf-8") as f:
        return [json.loads(line) for line in f if line.strip()]


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--file", default=os.path.join(gen.HERE, "data", "hidden.jsonl"))
    ap.add_argument("--line", type=int, action="append", default=[],
                    help="1-based line number to check (repeatable)")
    ap.add_argument("--random", type=int, default=20, help="number of random cases")
    ap.add_argument("--seed", type=int, default=None, help="seed for --random")
    ap.add_argument("--all", action="store_true", help="check every case")
    ap.add_argument("--source", default=gen.DEFAULT_SOURCE)
    ap.add_argument("--jobs", type=int, default=os.cpu_count() or 4)
    opts = ap.parse_args()

    cases = load(opts.file)
    if opts.all:
        picks = list(range(1, len(cases) + 1))
    elif opts.line:
        picks = opts.line
    else:
        rng = random.Random(opts.seed)
        picks = sorted(rng.sample(range(1, len(cases) + 1), min(opts.random, len(cases))))
    for n in picks:
        if not 1 <= n <= len(cases):
            sys.exit("line %d out of range (1..%d)" % (n, len(cases)))

    workdir = tempfile.mkdtemp(prefix="pyfiglet-spotcheck-")
    try:
        src, origin = gen.build_oracle(opts.source, workdir)
        oracle = gen.Oracle(src, workdir)
        print("oracle: pyfiglet %s from %s" % (gen.COMMIT[:7], origin))

        import concurrent.futures
        with concurrent.futures.ThreadPoolExecutor(opts.jobs) as pool:
            results = list(pool.map(lambda n: oracle(cases[n - 1]["args"]), picks))

        bad = 0
        for n, (rc, out, err) in zip(picks, results):
            want = cases[n - 1]["out"]
            got = out.decode("utf-8", "replace")
            if rc == 0 and not err and got == want:
                continue
            bad += 1
            print("MISMATCH line %d: args=%s (exit %s)" % (n, json.dumps(cases[n - 1]["args"]), rc))
            for d in difflib.unified_diff(want.splitlines(True), got.splitlines(True),
                                          "stored", "oracle", n=1):
                sys.stdout.write("    " + d if d.endswith("\n") else "    " + d + "\n")
        print("%d/%d cases match the original (%s)" % (len(picks) - bad, len(picks),
                                                      os.path.relpath(opts.file)))
        return 1 if bad else 0
    finally:
        shutil.rmtree(workdir, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())

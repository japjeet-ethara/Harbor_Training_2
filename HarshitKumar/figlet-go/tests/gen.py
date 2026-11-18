#!/usr/bin/env python3
"""Deterministic test case generator for figlet-go task.

Builds 2000 distinct visible cases into tests/data/visible.jsonl and
2000 distinct hidden cases into tests/data/hidden.jsonl by executing
the original pyfiglet CLI logic in fresh subprocesses. Hidden cases
never reuse a JSON line already written to the visible set.
"""

import io
import json
import os
import random
import subprocess
import sys
from pathlib import Path

repo_root = Path(__file__).resolve().parent.parent
possible_pyfiglet_paths = [
    repo_root.parent / "pyfiglet",
    repo_root / "pyfiglet",
]
pyfiglet_dir = None
for p in possible_pyfiglet_paths:
    if p.exists() and (p / "pyfiglet").exists():
        pyfiglet_dir = p
        break

if pyfiglet_dir is None:
    raise RuntimeError("Could not find pyfiglet directory")

ENV = os.environ.copy()
ENV["PYTHONUTF8"] = "1"
ENV["PYTHONIOENCODING"] = "utf-8"

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFICATIONS = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]

WORDS = [
    "hello", "world", "Harbor", "Go", "test", "SHOUT", "GIANT", "letters",
    "the", "quick", "brown", "fox", "jumps", "over", "lazy", "dog",
    "12345", "42", "2026",
    # Punctuation & smushing triggers
    "|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">",
    "|/\\_", "[{()}]", "<>", "/\\", "\\/", "[_]", "{_}",
    "A|B", "X_Y", "foo/bar", "one\\two", "(a+b)", "[x]", "{y}",
    # Latin-1 & special characters
    "café", "straße", "año", "naïve", "über", "München",
    # Double spaces & wrapping sentences
    "hello  world", "double  space  here",
    "This is a longer line that will definitely need word wrapping across columns",
    "Short", "UPPERCASE", "lowercase", "MixedCase_123"
]


def generate_single_case(rng: random.Random) -> dict:
    while True:
        font = rng.choice(FONTS)
        width = rng.choice(WIDTHS)
        just = rng.choice(JUSTIFICATIONS)
        direction = rng.choice(DIRECTIONS)

        args = ["-f", font, "-w", str(width), "-j", just, "-D", direction]

        if rng.random() < 0.25:
            args.append("-r")
        if rng.random() < 0.25:
            args.append("-F")
        if rng.random() < 0.25:
            args.append("-s")
        elif rng.random() < 0.25:
            args.append("-n")

        # Pick text (1 to 4 words or punctuation fragments)
        num_words = rng.randint(1, 4)
        text = " ".join(rng.choice(WORDS) for _ in range(num_words))
        args.append(text)

        try:
            res = subprocess.run(
                [sys.executable, "-m", "pyfiglet", *args],
                cwd=pyfiglet_dir,
                capture_output=True,
                env=ENV,
                timeout=2,
            )
            if res.returncode != 0:
                continue
            output_str = res.stdout.decode("utf-8").replace("\r\n", "\n")
            return {"args": args, "out": output_str}
        except Exception:
            continue


def generate_dataset(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed)
    cases = []
    for _ in range(count):
        cases.append(generate_single_case(rng))
    return cases


def main():
    out_dir = repo_root / "tests" / "data"
    out_dir.mkdir(parents=True, exist_ok=True)

    random.seed(7)
    rng = random.Random(7)

    # ---- visible.jsonl: 2000 distinct lines ----
    print("Generating 2000 visible cases (seed 7)...", flush=True)
    visible_set: set[str] = set()
    visible_lines: list[str] = []
    while len(visible_lines) < 2000:
        case = generate_single_case(rng)
        line = json.dumps(case, ensure_ascii=False)
        if line in visible_set:
            continue
        visible_set.add(line)
        visible_lines.append(line)
        if len(visible_lines) % 250 == 0:
            print(f"  Visible progress: {len(visible_lines)}/2000", flush=True)

    visible_path = out_dir / "visible.jsonl"
    with open(visible_path, "w", encoding="utf-8") as f:
        for line in visible_lines:
            f.write(line + "\n")
    print(f"Wrote {len(visible_lines)} lines to {visible_path}", flush=True)

    # ---- hidden.jsonl: 2000 distinct lines, none shared with visible ----
    print("Generating 2000 hidden cases (continuing from seed 7 generator)...", flush=True)
    hidden_set: set[str] = set()
    hidden_lines: list[str] = []
    while len(hidden_lines) < 2000:
        case = generate_single_case(rng)
        line = json.dumps(case, ensure_ascii=False)
        if line in visible_set or line in hidden_set:
            continue
        hidden_set.add(line)
        hidden_lines.append(line)
        if len(hidden_lines) % 250 == 0:
            print(f"  Hidden progress: {len(hidden_lines)}/2000", flush=True)

    hidden_path = out_dir / "hidden.jsonl"
    with open(hidden_path, "w", encoding="utf-8") as f:
        for line in hidden_lines:
            f.write(line + "\n")
    print(f"Wrote {len(hidden_lines)} lines to {hidden_path}", flush=True)


if __name__ == "__main__":
    main()

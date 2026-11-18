#!/usr/bin/env python3
"""Generate the visible and hidden cases for the pyfiglet -> Go task.

Runs on the task author's machine, NOT inside Docker. It imports the ORIGINAL
pyfiglet (the clone from Step 1, commit 255827ef..., with pyfiglet/fonts/
populated from fonts-standard/) and records exactly what pyfiglet.main()
writes to stdout. No "out" value is ever written by hand.

Usage:
    python tests/gen.py [--pyfiglet PATH]

PATH defaults to the first of these that exists:
    <task>/../pyfiglet    (clone next to the task folder)
    <task>/pyfiglet       (clone inside the task folder)

Output (UTF-8, LF line endings, one JSON object per line):
    tests/data/visible.jsonl   2000 cases
    tests/data/hidden.jsonl    2000 different cases
Each line: {"args": [...options..., ...text...], "out": "<exact stdout>"}
"""

from __future__ import annotations

import argparse
import collections
import io
import json
import random
import sys
from pathlib import Path

SEED = 7
N_PER_FILE = 2000

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = ["10", "20", "40", "60", "80", "120"]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]
FLAGS = ["-r", "-F", "-s", "-n"]  # each on in 1 of 4 cases

WORDS = [
    "hello", "world", "Harbor", "figlet", "Go", "port", "quick", "brown",
    "fox", "jumps", "over", "lazy", "dog", "ASCII", "art", "smush", "kern",
    "Hello,", "World!", "OK", "abc", "XYZ", "2026", "42", "v1.0", "a+b=c",
    "C:/tmp", "it's", "\"quoted\"", "#tag", "100%", "x*y", "~home", "@me",
]

# Rare cases from NOTES.md, turned into text that triggers them.
RARE = [
    # 1 + 3. Big-X rule: / + \ -> |, \ + / -> Y, > + < -> X, < + > no smush.
    "/\\", "\\/", "><", "<>", "/\\/\\", "\\/\\/", "><><", "<><>",
    # 2. Opposite pairs work in both orders.
    "[]", "][", "{}", "}{", "()", ")(", "[](){}", ")(][}{",
    # 4. Narrow characters never smush.
    "i", "l", "!", "|", ".", "ii", "l.l", "!!", "i|i", "1l1",
    # 5. Trailing spaces on the last row (seen with -s / -n).
    "T", "L", "_", "J.",
    # 7. Flip map: _ -> - but - stays -; brackets unchanged.
    "_-_", "a_b-c", "__--__", "(_)", "[-]",
]
SYMBOLS = ["|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">",
           "|/\\_[]{}()<>", "<|>", "{/}", "(_)", "[\\]", "/_\\", "\\_/"]
DOUBLE_SPACES = ["a  b", "hello  world", "x   y", "  lead", "trail  ",
                 "go  go  go", "/  \\", "[  ]"]
LATIN1 = ["é", "ß", "ñ", "café", "Straße", "niño", "Ä Ö Ü", "äöü", "ÄÖÜß",
          "crème brûlée", "señor", "Grüße"]
MISSING = ["€", "☃", "中文", "→", "☺ hi", "a€b", "日本", "✓ ok", "ü€ß"]
TRICKY = RARE + SYMBOLS + DOUBLE_SPACES + LATIN1 + MISSING


def find_pyfiglet(override: str | None) -> Path:
    task = Path(__file__).resolve().parent.parent
    candidates = [Path(override)] if override else [task.parent / "pyfiglet", task / "pyfiglet"]
    for c in candidates:
        if (c / "pyfiglet" / "__init__.py").is_file():
            if not (c / "pyfiglet" / "fonts" / "standard.flf").is_file():
                sys.exit(f"{c}: pyfiglet/fonts/ is missing. Run the font setup first:\n"
                         "  mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/")
            return c.resolve()
    sys.exit("pyfiglet clone not found; pass --pyfiglet PATH")


def make_text(rng: random.Random) -> list[str]:
    """Return the text part of argv: one or more arguments."""
    kind = rng.choices(["word", "tricky", "mix", "long"], weights=[25, 35, 20, 20])[0]
    if kind == "word":
        parts = rng.sample(WORDS, rng.randint(1, 2))
    elif kind == "tricky":
        parts = rng.sample(TRICKY, rng.randint(1, 2))
    elif kind == "mix":
        parts = [rng.choice(WORDS), rng.choice(TRICKY)]
        rng.shuffle(parts)
    else:  # long sentence that must wrap
        pool = WORDS * 3 + TRICKY
        parts = [rng.choice(pool) for _ in range(rng.randint(6, 14))]
    # Either one argument with spaces, or several arguments (pyfiglet joins with ' ').
    if rng.random() < 0.5:
        return [" ".join(parts)]
    return parts


def make_args(rng: random.Random) -> list[str]:
    args = ["-f", rng.choice(FONTS)]
    if rng.random() < 0.85:
        args += ["-w", rng.choice(WIDTHS)]
    if rng.random() < 0.85:
        args += ["-j", rng.choice(JUSTIFY)]
    if rng.random() < 0.85:
        args += ["-D", rng.choice(DIRECTION)]
    for flag in FLAGS:
        if rng.random() < 0.25:
            args.append(flag)
    return args + make_text(rng)


# pyfiglet loops forever on some inputs (e.g. -f slant -w 10 Harbor: the
# wrapper in FigletBuilder.handleNewLine never makes progress). We stop a run
# after a fixed number of executed Python lines. Counting lines, not seconds,
# keeps the skip decision identical on every machine and every run.
STEP_BUDGET = 3_000_000


class StepBudgetExceeded(Exception):
    pass


def _budget_tracer():
    steps = 0

    def tracer(frame, event, arg):
        nonlocal steps
        if event == "line":
            steps += 1
            if steps > STEP_BUDGET:
                raise StepBudgetExceeded
        return tracer

    return tracer


def run_pyfiglet(pyfiglet_mod, args: list[str]) -> str | None:
    """Run the original pyfiglet.main() and capture stdout bytes exactly.

    Returns None when pyfiglet does not produce a normal result (an exception
    such as CharNotPrinted, a non-zero return code, or an endless loop). Those
    inputs are not valid cases and are skipped.
    """
    buf = io.BytesIO()
    old_argv, old_stdout = sys.argv, sys.stdout
    sys.argv = ["pyfiglet"] + args
    # main() calls sys.stdout.detach() and writes UTF-8 bytes to the result.
    sys.stdout = io.TextIOWrapper(buf, encoding="utf-8", newline="")
    sys.settrace(_budget_tracer())
    try:
        rc = pyfiglet_mod.main()
    except BaseException:  # noqa: BLE001 - any failure means "not a valid case"
        rc = -1
    finally:
        sys.settrace(None)
        sys.argv, sys.stdout = old_argv, old_stdout
    if rc != 0:
        return None
    return buf.getvalue().decode("utf-8")


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--pyfiglet", help="path to the pyfiglet clone")
    opts = ap.parse_args()

    root = find_pyfiglet(opts.pyfiglet)
    sys.path.insert(0, str(root))
    import pyfiglet  # the original, from the clone

    if not str(Path(pyfiglet.__file__).resolve()).startswith(str(root)):
        sys.exit(f"imported pyfiglet from {pyfiglet.__file__}, expected {root}")

    rng = random.Random(SEED)
    random.seed(SEED)  # pyfiglet itself uses no randomness; seeded for safety

    out_dir = Path(__file__).resolve().parent / "data"
    out_dir.mkdir(parents=True, exist_ok=True)

    seen: set[str] = set()
    skipped = blank = 0
    for name in ("visible", "hidden"):
        cases = []
        while len(cases) < N_PER_FILE:
            args = make_args(rng)
            key = json.dumps(args, ensure_ascii=False)
            if key in seen:
                continue
            seen.add(key)  # never reuse args, inside a file or across files
            out = run_pyfiglet(pyfiglet, args)
            if out is None:
                skipped += 1
                continue
            if out.strip() == "":
                # Text the font cannot draw (e.g. "é" or "€" alone) gives a
                # blank drawing. A cheat printing "\n" would match those for
                # free, so they are not valid cases.
                blank += 1
                continue
            cases.append({"args": args, "out": out})

        path = out_dir / f"{name}.jsonl"
        with open(path, "w", encoding="utf-8", newline="\n") as fh:
            for c in cases:
                fh.write(json.dumps(c, ensure_ascii=False) + "\n")
        print(f"wrote {len(cases)} cases to {path}")
        report(cases)
    print(f"skipped {skipped} inputs that pyfiglet rejects (e.g. CharNotPrinted)")
    print(f"skipped {blank} inputs that draw nothing (blank output)")


def report(cases: list[dict]) -> None:
    def opt(args, flag, default):
        return args[args.index(flag) + 1] if flag in args else default

    c = collections.Counter()
    for case in cases:
        a = case["args"]
        c["font=" + opt(a, "-f", "standard")] += 1
        c["width=" + opt(a, "-w", "80")] += 1
        c["justify=" + opt(a, "-j", "auto")] += 1
        c["dir=" + opt(a, "-D", "auto")] += 1
        for f in FLAGS:
            c[f] += f in a
        c["rtl_and_narrow(<=20)"] += (opt(a, "-D", "auto") == "right-to-left"
                                      and int(opt(a, "-w", "80")) <= 20)
    for k in sorted(c):
        print(f"  {k:24} {c[k]}")


if __name__ == "__main__":
    main()

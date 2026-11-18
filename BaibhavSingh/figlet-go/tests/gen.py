#!/usr/bin/env python3
"""Generate the frozen answer files for the figlet-go task.

Runs on the task author's machine (NOT in Docker), with the pyfiglet clone from
Step 1 next to the task folder:

    work/
      pyfiglet/      <- git clone, commit 255827ef..., fonts set up (pyfiglet/fonts/)
      figlet-go/
        tests/gen.py <- this file

Usage:  python3 tests/gen.py            (from the figlet-go folder)
        PYFIGLET_DIR=/path/to/pyfiglet python3 tests/gen.py

Every "out" is produced by running the ORIGINAL pyfiglet main() in a fresh
subprocess and capturing its stdout. Nothing is written by hand. Inputs on which
pyfiglet crashes (non-zero exit) or hangs (timeout) are dropped, because they
have no defined answer.
"""
import json
import os
import random
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor

HERE = os.path.dirname(os.path.abspath(__file__))
TASK = os.path.dirname(HERE)
PYFIGLET_DIR = os.path.abspath(
    os.environ.get("PYFIGLET_DIR", os.path.join(TASK, "..", "pyfiglet")))
DATA = os.path.join(HERE, "data")
N_PER_FILE = 2000
TIMEOUT = 10  # seconds; real renders take milliseconds, hangs never finish

# Runs the original main() exactly as `python -m pyfiglet` would.
RUNNER = (
    "import sys; sys.path.insert(0, sys.argv[1]); "
    "from pyfiglet import main; "
    "sys.argv = ['pyfiglet'] + sys.argv[2:]; "
    "sys.exit(main())"
)

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script",
         # ivrit: the only standard font whose header says right-to-left,
         # so it is the only way to exercise "-D auto" choosing RTL.
         "ivrit"]
WIDTHS = ["10", "20", "40", "60", "80", "120"]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = ("hello world figlet go port smush kern wrap shout giant letters harbor "
         "test quick brown fox jumps over lazy dog Hello WORLD Zebra mixed CaSe "
         "x y i l I 1 0 42 2026 ok no").split()

# Rare cases from NOTES.md + the edge characters smushing rules care about.
TRICKY = [
    "|_|", "||", "__", "_|_", "/\\", "\\/", "><", "<>", "[]", "][", "{}", "}{",
    "()", ")(", "|/", "/|", "_/", "\\_", "[{(<>)}]", "$", "$$", "a$b",
    "| / \\ _ [ ] { } ( ) < >", "|||", "///", "\\\\\\", "_ _", "/_\\", "\\_/",
    "mm", "WW", "MW", "vv", "^^", "~", "`", "'\"", "#@%&*", "!?", ".,;:",
    "il1|", "o_O", ">_<", "<|>", "{|}", "(|)", "[|]",
]
LATIN = ["é", "ß", "ñ", "Ä", "Ö", "Ü", "ä", "ö", "ü", "Straße", "Ñandú",
         "café", "Grüße", "ÄÖÜäöüß", "naïve façade"]
MISSING = ["€", "漢字", "☃", "→", "😀", "\u00a0", "\t", "\x7f", "Ω"]
LONG = [
    "the quick brown fox jumps over the lazy dog",
    "this sentence is long enough that it has to wrap onto more lines",
    "supercalifragilisticexpialidocious",
    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "one two three four five six seven eight nine ten",
    "| | | | | | | | | | | | | | | | | | | |",
    "__init__ __main__ __str__ __repr__",
]


def make_text(rng):
    kind = rng.random()
    if kind < 0.20:
        return " ".join(rng.choice(WORDS) for _ in range(rng.randint(1, 3)))
    if kind < 0.45:
        return rng.choice(TRICKY) if rng.random() < 0.6 else \
            " ".join(rng.choice(TRICKY) for _ in range(rng.randint(2, 3)))
    if kind < 0.55:
        return rng.choice(LATIN) + (" " + rng.choice(WORDS) if rng.random() < 0.5 else "")
    if kind < 0.62:
        return rng.choice(WORDS) + rng.choice(MISSING) + rng.choice(WORDS)
    if kind < 0.72:
        a, b = rng.choice(WORDS), rng.choice(WORDS + TRICKY)
        return a + " " * rng.randint(2, 4) + b
    if kind < 0.87:
        return rng.choice(LONG)
    if kind < 0.93:
        return rng.choice(WORDS) + "\n" + rng.choice(WORDS + TRICKY)
    if kind < 0.96:
        return " " + rng.choice(WORDS) + " "  # leading/trailing blanks
    return "".join(rng.choice("|/\\_[]{}()<>abcXYZ $") for _ in range(rng.randint(1, 8)))


def make_args(rng):
    args = []
    if rng.random() < 0.9:
        args += ["-f", rng.choice(FONTS)]
    if rng.random() < 0.85:
        # narrow widths more often: that is where wrapping and justification break
        args += ["-w", rng.choice(WIDTHS + ["10", "20", "40"])]
    if rng.random() < 0.7:
        args += ["-j", rng.choice(JUSTIFY)]
    if rng.random() < 0.7:
        args += ["-D", rng.choice(DIRECTION)]
    for flag in ["-r", "-F", "-s", "-n"]:
        if rng.random() < 0.25:
            args.append(flag)
    text = make_text(rng)
    # a text argument must never look like an option
    if text.startswith("-"):
        text = "x" + text
    if " " in text and rng.random() < 0.25:
        parts = text.split(" ")  # several args, joined back with one space by main()
        parts = ["x" + p if p.startswith("-") else p for p in parts]
        args += parts
    else:
        args.append(text)
    return args


def run_original(args):
    try:
        p = subprocess.run([sys.executable, "-I", "-c", RUNNER, PYFIGLET_DIR] + args,
                           capture_output=True, timeout=TIMEOUT)
    except subprocess.TimeoutExpired:
        return None, "timeout"
    if p.returncode != 0:
        return None, "crash"
    return p.stdout.decode("utf-8"), "ok"


def main():
    if not os.path.isdir(os.path.join(PYFIGLET_DIR, "pyfiglet", "fonts")):
        sys.exit(f"{PYFIGLET_DIR}/pyfiglet/fonts missing: do the Makefile font step first")
    random.seed(7)
    rng = random  # all randomness comes from the module generator seeded above

    seen = set()
    visible, hidden = [], []
    stats = {"ok": 0, "crash": 0, "timeout": 0, "duplicate": 0}
    with ThreadPoolExecutor(max_workers=os.cpu_count() or 4) as pool:
        while len(visible) < N_PER_FILE or len(hidden) < N_PER_FILE:
            batch = [make_args(rng) for _ in range(1000)]
            results = list(pool.map(run_original, batch))  # order preserved
            for args, (out, status) in zip(batch, results):
                if status != "ok":
                    stats[status] += 1
                    continue
                key = json.dumps(args)
                if key in seen:  # no case may appear twice, nor in both files
                    stats["duplicate"] += 1
                    continue
                seen.add(key)
                stats["ok"] += 1
                case = {"args": args, "out": out}
                if len(visible) < N_PER_FILE:
                    visible.append(case)
                elif len(hidden) < N_PER_FILE:
                    hidden.append(case)

    os.makedirs(DATA, exist_ok=True)
    for name, cases in (("visible", visible), ("hidden", hidden)):
        with open(os.path.join(DATA, name + ".jsonl"), "w", encoding="utf-8", newline="\n") as f:
            for c in cases:
                f.write(json.dumps(c, ensure_ascii=False) + "\n")
    print("dropped / kept:", stats, file=sys.stderr)


if __name__ == "__main__":
    main()

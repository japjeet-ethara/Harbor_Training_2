#!/usr/bin/env python3
"""
Freeze pyfiglet's answers into tests/data/{visible,hidden}.jsonl.

Runs on the task author's laptop, NOT in Docker. Needs the pyfiglet clone from
Step 1 next to the task folder (../../pyfiglet from this file), at commit
255827efbc09d662875dad50fa0edc1e1263bf45, with the font setup done
(pyfiglet/fonts/ populated from pyfiglet/fonts-standard/).

Every "out" is the exact stdout of the ORIGINAL pyfiglet main(), run in a
fresh subprocess. Nothing is written by hand.

Cases are dropped (not frozen) when pyfiglet
  - crashes (non-zero exit, e.g. CharNotPrinted) -> stdout would be empty,
  - prints only whitespace (e.g. "\\n" when the font has none of the glyphs),
  - hangs (e.g. -f slant -w 10 H loops forever)   -> killed after TIMEOUT s.
Keeping such cases would let a "print nothing / print a newline" program
match some cases and score points.

Usage:  python tests/gen.py          (takes ~1-3 minutes)
"""

import json
import os
import random
import subprocess
import sys
from collections import Counter
from concurrent.futures import ThreadPoolExecutor

HERE = os.path.dirname(os.path.abspath(__file__))
PYFIGLET_DIR = os.environ.get(
    "PYFIGLET_DIR", os.path.normpath(os.path.join(HERE, "..", "..", "pyfiglet")))
DATA_DIR = os.path.join(HERE, "data")
PINNED_COMMIT = "255827efbc09d662875dad50fa0edc1e1263bf45"

N_VISIBLE = 2000
N_HIDDEN = 2000
TIMEOUT = 10  # seconds; real cases finish in well under 1 s
BATCH = 400

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]
FLAGS = ["-r", "-F", "-s", "-n"]

WORDS = [
    "hello", "world", "Go", "pyfiglet", "FIGlet", "smush", "Harbor", "twin",
    "Shout", "giant", "letters", "Quick", "brown", "fox", "jumps", "over",
    "the", "lazy", "dog", "HELLO", "WORLD", "Kerning", "wrap", "center",
    "iii", "lll", "Hi!", "OK?", "1234567890", "3.14", "a+b=c", "x*y", "50%",
    "#tag", "@home", "$5", "~tilde~", "'quote'", '"dq"', "semi;colon",
    "co:lon", "comma,", "back`tick", "caret^", "and&", "eq=", "q?",
]

# Rare cases from NOTES.md, plus every smush-rule character.
SMUSHY = [
    "|_|", "/\\", "\\/", "><", "<>", "[]", "][", "{}", "}{", "()", ")(",
    "|/", "/|", "_|_", "__", "||", "|||", "_/", "\\_", "[|]", "{|}", "(|)",
    "<|>", "/_\\", "\\_/", "W", "H", "M", "WW", "HH", "MW", "WWW", "AV", "VA",
    "TT", "LT", "ij", "j", "l1|", "Ty", "bP", "vw", "^v",
    "|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">",
]
TRICKY = SMUSHY + [
    # Latin-1 / German (some fonts have them, most lack é ñ)
    "ÄÖÜ", "äöüß", "é", "ñ", "ß", "café", "naïve", "Straße", "señor", "Ünïcödé",
    # characters no task font defines (silently skipped)
    "€", "☃", "日本", "a\tb", "x€y",
]

PUNCT = list("|/\\_[]{}()<>!?.,:;'\"`^~*+=#$%&@")
LETTERS = list("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
LATIN1 = list("ÄÖÜäöüßéñçàøÅ€")


def tok(rng):
    return rng.choice(WORDS) if rng.random() < 0.55 else rng.choice(TRICKY)


def make_text(rng):
    k = rng.random()
    if k < 0.20:                                   # one token
        t = tok(rng)
    elif k < 0.45:                                 # a few tokens, single/double spaces
        t = "".join(
            tok(rng) + rng.choice([" ", " ", " ", "  "]) for _ in range(rng.randint(2, 4))
        ).rstrip(" ")
    elif k < 0.60:                                 # long sentence that must wrap
        t = " ".join(rng.choice(WORDS) for _ in range(rng.randint(6, 14)))
    elif k < 0.70:                                 # contains newlines
        t = "\n".join(tok(rng) for _ in range(rng.randint(2, 3)))
    elif k < 0.85:                                 # random character soup
        pool = PUNCT * 3 + LETTERS + LATIN1 + [" "] * 4
        t = "".join(rng.choice(pool) for _ in range(rng.randint(1, 10))).strip(" ")
    else:                                          # smush-heavy: tricky tokens glued
        t = "".join(rng.choice(SMUSHY) for _ in range(rng.randint(2, 5)))
    if not t.strip():
        t = rng.choice(WORDS)
    return t


def split_args(rng, text):
    """Usually one argument; sometimes split on single spaces into several
    (pyfiglet joins them back with ' ', so the text is unchanged)."""
    if " " in text and rng.random() < 0.3:
        return text.split(" ")   # "a  b" -> ["a", "", "b"]: empty args are legal
    return [text]


def make_args(rng):
    opts = []
    if rng.random() < 0.9:
        opts.append(["-f", rng.choice(FONTS)])
    if rng.random() < 0.85:
        opts.append(["-w", str(rng.choice(WIDTHS))])
    if rng.random() < 0.7:
        opts.append(["-j", rng.choice(JUSTIFY)])
    if rng.random() < 0.7:
        opts.append(["-D", rng.choice(DIRECTION)])
    for flag in FLAGS:
        if rng.random() < 0.25:
            opts.append([flag])
    rng.shuffle(opts)
    args = [a for o in opts for a in o]
    text_args = split_args(rng, make_text(rng))
    for a in text_args:  # instruction.md promise: text never starts with '-'
        assert not a.startswith("-"), a
    return args + text_args


RUNNER = (
    "import sys; sys.path.insert(0, sys.argv.pop(1)); "
    "import pyfiglet; sys.exit(pyfiglet.main())"
)


def run_original(args):
    """Return pyfiglet's stdout as str, or None if it crashed or hung."""
    try:
        p = subprocess.run(
            [sys.executable, "-c", RUNNER, PYFIGLET_DIR] + args,
            capture_output=True, timeout=TIMEOUT,
            env={**os.environ, "PYTHONIOENCODING": "utf-8", "PYTHONUTF8": "1"},
        )
    except subprocess.TimeoutExpired:
        return "HANG"
    # Also drop blank-only output (e.g. "\n" when no glyph exists): a trivial
    # program that always prints "\n" must not be able to match any case.
    if p.returncode != 0 or not p.stdout.strip():
        return None
    return p.stdout.decode("utf-8")


def check_setup():
    fonts = os.path.join(PYFIGLET_DIR, "pyfiglet", "fonts")
    if not os.path.isdir(fonts) or not os.path.isfile(os.path.join(fonts, "standard.flf")):
        sys.exit(f"pyfiglet font setup missing: run\n  mkdir -p {fonts} && "
                 f"cp {fonts}-standard/* {fonts}/")
    head = os.path.join(PYFIGLET_DIR, ".git", "HEAD")
    if os.path.isfile(head):
        try:
            sha = subprocess.run(["git", "-C", PYFIGLET_DIR, "rev-parse", "HEAD"],
                                 capture_output=True, text=True).stdout.strip()
            if sha != PINNED_COMMIT:
                sys.exit(f"pyfiglet is at {sha}, expected {PINNED_COMMIT}")
        except FileNotFoundError:
            pass


def main():
    check_setup()
    random.seed(7)
    rng = random  # the seeded module-level generator
    need = N_VISIBLE + N_HIDDEN
    kept, seen = [], set()
    dropped = Counter()
    workers = min(8, os.cpu_count() or 4)

    with ThreadPoolExecutor(max_workers=workers) as pool:
        while len(kept) < need:
            batch = []
            while len(batch) < BATCH:
                args = make_args(rng)
                key = tuple(args)
                if key in seen:           # visible and hidden never share a case
                    dropped["duplicate"] += 1
                    continue
                seen.add(key)
                batch.append(args)
            # map() keeps input order -> deterministic regardless of timing
            for args, out in zip(batch, pool.map(run_original, batch)):
                if out == "HANG":
                    dropped["hang"] += 1
                elif out is None:
                    dropped["crash/blank"] += 1
                elif len(kept) < need:
                    kept.append({"args": args, "out": out})
            print(f"  kept {len(kept)}/{need}", file=sys.stderr)

    os.makedirs(DATA_DIR, exist_ok=True)
    sets = {"visible": kept[:N_VISIBLE], "hidden": kept[N_VISIBLE:need]}
    for name, cases in sets.items():
        path = os.path.join(DATA_DIR, name + ".jsonl")
        with open(path, "w", encoding="utf-8", newline="\n") as f:
            for c in cases:
                f.write(json.dumps(c, ensure_ascii=False) + "\n")

    # --- coverage report: are the hard rules actually exercised? ---
    print(f"dropped: {dict(dropped)}")
    for name, cases in sets.items():
        c = Counter()
        for case in cases:
            a = case["args"]
            opt = {a[i]: a[i + 1] for i in range(len(a) - 1) if a[i] in ("-f", "-w", "-j", "-D")}
            c["font=" + opt.get("-f", "(default)")] += 1
            c["w=" + opt.get("-w", "(default)")] += 1
            c["j=" + opt.get("-j", "(default)")] += 1
            c["D=" + opt.get("-D", "(default)")] += 1
            for fl in FLAGS:
                c[fl] += fl in a
            narrow = opt.get("-w") in ("10", "20")
            rtl = opt.get("-D") == "right-to-left"
            c["RTL+narrow(<=20)"] += rtl and narrow
            text = " ".join(x for x in a if x not in FLAGS)
            c["has newline"] += "\n" in text
            c["multi-line output (wrapped)"] += case["out"].count("\n") > 12
        print(f"\n{name}: {len(cases)} cases")
        for k in sorted(c):
            print(f"  {k:30} {c[k]}")


if __name__ == "__main__":
    main()

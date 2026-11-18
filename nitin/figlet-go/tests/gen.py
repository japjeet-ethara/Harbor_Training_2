#!/usr/bin/env python3
"""Generate the visible and hidden test cases for the figlet-go task.

Runs on the AUTHOR's laptop (never in Docker). It needs a pyfiglet checkout at
commit 255827efbc09d662875dad50fa0edc1e1263bf45, with the Makefile font step done:

    mkdir -p pyfiglet/pyfiglet/fonts && cp pyfiglet/pyfiglet/fonts-standard/* pyfiglet/pyfiglet/fonts/

Every "out" comes from running the ORIGINAL pyfiglet main() with the case's args and
capturing stdout byte for byte. Nothing is ever written by hand.

    python3 tests/gen.py [--pyfiglet DIR]      (default DIR: ../pyfiglet next to the task folder)
"""
import argparse
import io
import json
import os
import random
import signal
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
OUT_DIR = os.path.join(HERE, "data")
N_PER_FILE = 2000
TIMEOUT_SEC = 0.7  # the original loops forever when a character is exactly as wide as -w

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = ["hello", "world", "figlet", "go", "Harbor", "slash", "smush", "kern", "Python", "gopher",
         "WAVE", "MWVA", "Quick", "brown", "fox", "jumps", "over", "lazy", "dog", "pixel",
         "Zebra", "xylophone", "OK", "42", "2026", "a", "I", "Hi", "yo", "mix", "TeSt", "ship"]

# The "rare cases" list from NOTES.md: places where smushing / wrapping / justification bite.
RARE = ["|_|", "_|_", "|_|_|", "/\\/\\", "\\/\\/", "/\\", "\\/", "[]", "][", "{}", "}{", "()", ")(",
        "<>", "><", "||", "__", "___", "_|", "|_", "/_/", "\\_\\", "A V", "VAVA", "MWMW", "~~", "^_^",
        "`'", "'`", "-_-", "=_=", "(_)", "[_]", "{_}", "<_>", "|-|", "\\|/", "/|\\", "!!", "..",
        "ii", "lll", "ll", "I I", "i.i", "j j", "ij", "WW", "mm", "oo", "00", "x x", "o_o"]

PUNCT = list("|/\\_[]{}()<>")
LATIN1 = ["é", "ß", "ñ", "ü", "Ä", "ö", "à", "ç", "ø", "å", "Éclair", "Straße", "niño", "café", "über"]
MISSING = ["€", "☃", "日本", "→", "…", "\u00a0", "\t", "\u200b", "λ", "😀", "x€y", "ab☃cd"]
SENTENCES = [
    "the quick brown fox jumps over the lazy dog",
    "pack my box with five dozen liquor jugs",
    "smushing squeezes giant letters together",
    "a long sentence that must wrap onto several lines",
    "wrap me at every possible width please",
    "Go ports must match pyfiglet byte for byte",
    "sphinx of black quartz judge my vow",
    "how vexingly quick daft zebras jump",
]


def rnd_word(r):
    return r.choice(WORDS)


def make_text(r):
    kind = r.random()
    if kind < 0.28:
        t = rnd_word(r) if r.random() < 0.6 else " ".join(rnd_word(r) for _ in range(r.randint(2, 3)))
    elif kind < 0.50:
        t = r.choice(RARE)
        if r.random() < 0.5:
            t = t + r.choice(["", " ", "x", "o"]) + r.choice(RARE)
    elif kind < 0.58:
        t = "".join(r.choice(PUNCT) for _ in range(r.randint(1, 7)))
    elif kind < 0.66:
        a, b = rnd_word(r), rnd_word(r)
        t = a + r.choice(["  ", "   ", "    "]) + b
    elif kind < 0.78:
        t = r.choice(SENTENCES)
        if r.random() < 0.3:
            t += " " + r.choice(SENTENCES)
    elif kind < 0.86:
        t = r.choice(LATIN1)
        if r.random() < 0.5:
            t = rnd_word(r) + " " + t
    elif kind < 0.92:
        t = r.choice(MISSING)
        if r.random() < 0.7:
            t = rnd_word(r) + t + rnd_word(r)
    elif kind < 0.95:
        t = r.choice(["", " ", "  ", "   a", "a   ", " a b "])
    else:
        t = "".join(r.choice(PUNCT + list("abc ABC")) for _ in range(r.randint(4, 14)))
    if t.startswith("-"):  # would be parsed as an option
        t = "x" + t
    return t


def make_args(r):
    args = []
    font = r.choice(FONTS)
    if font != "standard" or r.random() < 0.4:
        args += ["-f", font]
    if r.random() < 0.85:
        args += ["-w", str(r.choice(WIDTHS))]
    if r.random() < 0.7:
        args += ["-j", r.choice(JUSTIFY)]
    if r.random() < 0.7:
        args += ["-D", r.choice(DIRECTION)]
    for flag in ("-r", "-F", "-s", "-n"):
        if r.random() < 0.25:
            args.append(flag)
    # option order does not matter to pyfiglet: shuffle the option groups
    groups, i = [], 0
    while i < len(args):
        if args[i] in ("-f", "-w", "-j", "-D"):
            groups.append(args[i:i + 2])
            i += 2
        else:
            groups.append([args[i]])
            i += 1
    r.shuffle(groups)
    flat = [x for g in groups for x in g]
    return flat + [make_text(r)]


class Timeout(Exception):
    pass


def _alarm(signum, frame):
    raise Timeout()


class _Cap:
    """Stands in for sys.stdout: main() does sys.stdout.detach() and writes bytes to it."""
    def __init__(self):
        self.buf = io.BytesIO()

    def detach(self):
        return self.buf

    def write(self, s):  # text written before detach (not expected)
        self.buf.write(s.encode("utf-8"))

    def flush(self):
        pass


def run_original(pyfiglet_main, args):
    """Run the original pyfiglet main() and return its stdout as str, or None if it fails/hangs."""
    cap = _Cap()
    real_out, real_argv = sys.stdout, sys.argv
    sys.stdout, sys.argv = cap, ["pyfiglet"] + args
    signal.signal(signal.SIGALRM, _alarm)
    signal.setitimer(signal.ITIMER_REAL, TIMEOUT_SEC)
    try:
        code = pyfiglet_main()
    except BaseException:
        return None
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        sys.stdout, sys.argv = real_out, real_argv
    if code not in (0, None):
        return None
    try:
        return cap.buf.getvalue().decode("utf-8")
    except UnicodeDecodeError:
        return None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--pyfiglet", default=os.path.join(HERE, "..", "..", "pyfiglet"))
    ns = ap.parse_args()
    root = os.path.abspath(ns.pyfiglet)
    if not os.path.isdir(os.path.join(root, "pyfiglet", "fonts")):
        sys.exit("pyfiglet/fonts is missing: run the Makefile font step first (see the docstring).")
    sys.path.insert(0, root)
    import pyfiglet  # the ORIGINAL

    r = random.Random(7)  # == random.seed(7), but isolated
    random.seed(7)
    seen = set()
    sets = {"visible": [], "hidden": []}
    skipped = 0
    for name in ("visible", "hidden"):
        while len(sets[name]) < N_PER_FILE:
            args = make_args(r)
            key = tuple(args)
            if key in seen:
                continue
            seen.add(key)
            out = run_original(pyfiglet.main, args)
            if out is None:
                skipped += 1
                continue
            sets[name].append({"args": args, "out": out})

    os.makedirs(OUT_DIR, exist_ok=True)
    for name, cases in sets.items():
        with open(os.path.join(OUT_DIR, name + ".jsonl"), "w", encoding="utf-8", newline="\n") as f:
            for c in cases:
                f.write(json.dumps(c, ensure_ascii=False) + "\n")

    for name, cases in sets.items():
        n = len(cases)
        rtl = sum("right-to-left" in c["args"] for c in cases)
        narrow = sum(any(a in ("10", "20") for a in c["args"][:-1]) and "-w" in c["args"] for c in cases)
        both = sum("right-to-left" in c["args"] and "-w" in c["args"] and
                   c["args"][c["args"].index("-w") + 1] in ("10", "20") for c in cases)
        flags = {f: sum(f in c["args"] for c in cases) for f in ("-r", "-F", "-s", "-n")}
        print(f"{name}: {n} cases | -D right-to-left: {rtl} | width 10/20: {narrow} | rtl+narrow: {both} | {flags}")
    print(f"skipped (original raised, hung or exited non-zero): {skipped}")


if __name__ == "__main__":
    main()

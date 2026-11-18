#!/usr/bin/env python3
"""Generate the visible and hidden cases for the figlet-go task.

Runs on the task author's laptop, NOT inside Docker. Needs the original
pyfiglet checkout next to the task folder, with the font setup step done:

    <parent>/pyfiglet/pyfiglet/fonts/   (copy of fonts-standard)
    <parent>/figlet-go/tests/gen.py     (this file)

Every "out" is produced by running the ORIGINAL pyfiglet main() and capturing
its stdout bytes. Nothing is ever written by hand.

Usage:  python3 tests/gen.py
"""

import io
import json
import os
import random
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
PYFIGLET_REPO = os.path.abspath(os.path.join(HERE, "..", "..", "pyfiglet"))
DATA_DIR = os.path.join(HERE, "data")

N_CASES = 2000
SEED = 7

if not os.path.isdir(os.path.join(PYFIGLET_REPO, "pyfiglet", "fonts")):
    sys.exit(
        "pyfiglet checkout with fonts not found at %s\n"
        "Do Step 1: git clone, git checkout 255827ef..., then\n"
        "  mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/"
        % PYFIGLET_REPO
    )

sys.path.insert(0, PYFIGLET_REPO)
import pyfiglet  # noqa: E402  (the ORIGINAL)

# ---------------------------------------------------------------------------
# Choices
# ---------------------------------------------------------------------------

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = [
    "hello", "world", "Harbor", "FIGlet", "pyfiglet", "Go", "port", "agent",
    "smush", "banner", "Quick", "brown", "fox", "jumps", "over", "lazy", "dog",
    "ABC", "xyz", "Test", "0123", "a", "I", "Hi!", "ok?", "x=y+z", "a.b,c;d",
    "MiXeD", "WIDE", "tall", "gjpqy", "Wow", "mm", "ii", "ll", "oo",
]

# Rare-cases list from NOTES.md
SYMBOL_TEXTS = [
    "|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">",
    "|_|", "_|_", "/\\", "\\/", "/\\/\\", "[]", "][", "{}", "}{", "()", ")(",
    "<>", "><", "<<>>", "||", "//", "\\\\", "__", "[_]", "(_)", "{_}", "<_>",
    "|/|\\|", "|-|", "[|]", "{|}", "(|)", "<|>", "/_\\", "\\_/", "/|\\",
    "[](){}<>", "|/\\[]{}()<>", "<>()[]{}\\/|_", "_|/\\[]{}()<>",
]
LATIN1_TEXTS = ["é", "ß", "ñ", "café", "Straße", "señor", "über", "Ärger", "naïve"]
MISSING_TEXTS = ["€", "日本", "😀", "Ω", "a€b", "x😀y", "日本語 text", "Ωmega", "☃ snow"]
SPACE_TEXTS = [
    " ", "  ", "   ", " a", "a ", " a ", "a  b", "a   b", "  hello", "hello  ",
    "  hi  there  ", "a b  c   d",
]
NEWLINE_TEXTS = ["a\nb", "hi\nthere", "\nx", "x\n", "a\n\nb", "one\ntwo\nthree"]
SENTENCES = [
    "the quick brown fox jumps over the lazy dog",
    "hello world from the harbor",
    "pack my box with five dozen liquor jugs",
    "a quick test of the wrapping logic here",
    "supercalifragilisticexpialidocious is long",
    "one two three four five six seven eight",
    "wrap me at a space or in a word",
    "Go port of pyfiglet must match exactly",
    "ab cd ef gh ij kl mn op qr st uv wx yz",
]
CHARS_POOL = list("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") + list(
    "|/\\_[]{}()<>-+=*#@!?.,:;'\" "
)


def make_text(rng):
    """Pick one text. Never starts with '-' (optparse would read it as an option)."""
    kind = rng.random()
    if kind < 0.22:
        text = rng.choice(WORDS)
    elif kind < 0.34:
        text = " ".join(rng.choice(WORDS) for _ in range(rng.randint(2, 4)))
    elif kind < 0.52:
        text = rng.choice(SYMBOL_TEXTS)
    elif kind < 0.60:
        text = "".join(rng.choice(["|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">"])
                       for _ in range(rng.randint(2, 8)))
    elif kind < 0.66:
        text = rng.choice(LATIN1_TEXTS)
    elif kind < 0.72:
        text = rng.choice(MISSING_TEXTS)
    elif kind < 0.78:
        text = rng.choice(SPACE_TEXTS)
    elif kind < 0.82:
        text = rng.choice(NEWLINE_TEXTS)
    elif kind < 0.92:
        text = rng.choice(SENTENCES)
    elif kind < 0.97:
        text = "".join(rng.choice(CHARS_POOL) for _ in range(rng.randint(1, 14)))
    elif kind < 0.99:
        # mixed: a word + a tricky piece
        text = rng.choice(WORDS) + " " + rng.choice(SYMBOL_TEXTS + LATIN1_TEXTS + MISSING_TEXTS)
    else:
        text = ""  # empty text
    if text.startswith("-"):
        text = "x" + text
    return text


def make_args(rng):
    args = []
    args += ["-f", rng.choice(FONTS)]
    args += ["-w", str(rng.choice(WIDTHS))]
    args += ["-j", rng.choice(JUSTIFY)]
    args += ["-D", rng.choice(DIRECTION)]
    for flag in ("-r", "-F", "-s", "-n"):
        if rng.random() < 0.25:
            args.append(flag)
    args.append(make_text(rng))
    return args


# ---------------------------------------------------------------------------
# Run the ORIGINAL
# ---------------------------------------------------------------------------


class RunawayLoop(Exception):
    pass


_orig_append = pyfiglet.FigletProduct.append


def _guarded_append(self, buffer):
    # A char whose width equals -w makes the original loop for ever. A real run
    # has at most a few dozen lines; stop deterministically (no timers).
    if len(self.queue) > 1000:
        raise RunawayLoop()
    _orig_append(self, buffer)


pyfiglet.FigletProduct.append = _guarded_append


def run_original(args):
    """Return stdout bytes of pyfiglet.main() for args, or None if it fails."""
    old_argv, old_stdout = sys.argv, sys.stdout
    raw = io.BytesIO()
    sys.argv = ["pyfiglet"] + args
    sys.stdout = io.TextIOWrapper(raw, encoding="utf-8")
    try:
        try:
            rc = pyfiglet.main()
        except SystemExit as e:
            rc = e.code
    except BaseException:  # CharNotPrinted, RunawayLoop, IndexError, ...
        return None
    finally:
        sys.argv, sys.stdout = old_argv, old_stdout
    if rc not in (0, None):
        return None
    return raw.getvalue()


def build_cases(rng, n, forbidden):
    cases = []
    seen = set()
    tries = 0
    while len(cases) < n:
        tries += 1
        args = make_args(rng)
        key = json.dumps(args, ensure_ascii=False)
        if key in seen or key in forbidden:
            continue
        out = run_original(args)
        if out is None:
            continue  # original errors or hangs: not a usable case
        try:
            text = out.decode("utf-8")
        except UnicodeDecodeError:
            continue
        seen.add(key)
        cases.append({"args": args, "out": text})
    return cases, seen, tries


def write_jsonl(path, cases):
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        for case in cases:
            f.write(json.dumps(case, ensure_ascii=False) + "\n")


def report(name, cases):
    rtl = sum(1 for c in cases if "right-to-left" in c["args"])
    narrow = sum(1 for c in cases if c["args"][c["args"].index("-w") + 1] in ("10", "20"))
    both = sum(
        1 for c in cases
        if "right-to-left" in c["args"] and c["args"][c["args"].index("-w") + 1] in ("10", "20")
    )
    print("%-8s %d cases | right-to-left: %d | width<=20: %d | both: %d"
          % (name, len(cases), rtl, narrow, both))
    return rtl, narrow, both


def main():
    random.seed(SEED)
    rng = random  # the global, seeded generator
    os.makedirs(DATA_DIR, exist_ok=True)

    visible, vkeys, t1 = build_cases(rng, N_CASES, set())
    hidden, _hkeys, t2 = build_cases(rng, N_CASES, vkeys)  # disjoint from visible

    write_jsonl(os.path.join(DATA_DIR, "visible.jsonl"), visible)
    write_jsonl(os.path.join(DATA_DIR, "hidden.jsonl"), hidden)

    print("tries: visible %d, hidden %d" % (t1, t2))
    for name, cases in (("visible", visible), ("hidden", hidden)):
        rtl, narrow, both = report(name, cases)
        # the hard rules must really be tested
        assert rtl > N_CASES * 0.2, "too few right-to-left cases"
        assert narrow > N_CASES * 0.2, "too few narrow-width cases"
        assert both > N_CASES * 0.05, "too few narrow right-to-left cases"
    print("wrote", DATA_DIR)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""
Generate the exam cases by running the ORIGINAL pyfiglet main().

Runs on the author's laptop (not in Docker). Layout expected:

    <somewhere>/pyfiglet/     <- Step 1 clone, at the pinned commit, with pyfiglet/fonts set up
    <somewhere>/figlet-go/    <- this task folder (this file is figlet-go/tests/gen.py)

Output:  tests/data/visible.jsonl  (2000 cases)
         tests/data/hidden.jsonl   (a different 2000 cases)
One JSON object per line:  {"args": [...], "out": "..."}

No "out" is ever written by hand: it is the exact stdout of pyfiglet.main().
Cases on which the original crashes, loops forever, or exits non-zero are dropped
(error behaviour is not part of the exam).
"""
import io
import json
import os
import random
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
TASK = os.path.dirname(HERE)
REPO = os.environ.get("PYFIGLET_DIR", os.path.join(os.path.dirname(TASK), "pyfiglet"))
sys.path.insert(0, REPO)

import pyfiglet  # noqa: E402  (the original, from the Step 1 clone)

random.seed(7)

N_PER_FILE = 2000

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
# Coverage fonts (also from fonts-standard). The eight fonts above never draw with [ ] { } or with the letters
# that the -F flip table swaps (A M P R W b m v w ^), and none has a right-to-left header, so a twin could get
# -r / -F / "-D auto" wrong and still pass. These fonts close those holes:
#   ivrit    = the only font whose header says right-to-left (auto direction + auto justify)
#   bubble   = draws with [ ] { } ^ and many letters       mnemonic/digital/term = letter-based art, other layouts
EXTRA_FONTS = ["ivrit", "bubble", "mnemonic", "digital", "term"]
EXTRA_FONT_SHARE = 0.20
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = ("hello world figlet python go giant letters shout chat smush kern slash "
         "underscore harbor agent task exam twin copy rule narrow window wrap "
         "Alpha Beta Gamma Delta ZEBRA Quick brown fox jumps over lazy dog "
         "a I x 42 2026 foo bar baz").split()

# The "rare cases" from NOTES.md: places where smushing rules collide.
TRICKY = [
    "|_|", "|/\\|", "/\\/\\", "\\/ \\/", "/\\", "\\/", "><", "<>", "_-_", "__", "||", "|||",
    "[]", "][", "{}", "}{", "()", ")(", "[ ]", "( )", "{ }", "///", "\\\\\\", "[]{}()<>",
    "<<>>", "_|_", "|_|_|", "/_/", "\\_\\", "(_)", "{_}", "[_]", "-_-", "T_T", "o_O", "^_^",
    "|/ \\|", "a|b", "a/b", "a\\b", "a_b", "[a]", "{a}", "(a)", "<a>", "i||i", "l|l",
    "x", "Q", "ee", "ll", "mm", "oo", "ii", "H|H", "AV", "VA", "WW", "TT", "LT", "j", "y",
    "Hello, World!", "a.b,c;d:e", "!?!", "...", "#$%&*+=", "@@@", "~~~", "'quoted'", '"dq"',
]
# Inputs whose art contains the characters that -r / -F swap (used mostly with the letter-art fonts)
FLIP_TEXTS = ["AMPRW", "bmvw", "ARM", "WAVE", "MP3", "Rob", "Meow", "vw", "^v^", "[a]{b}", "PRAWN", "Mmm", "wavy", "Bob",
              "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "abcdefghijklmnopqrstuvwxyz", "0123456789", "[]{}()<>^_~", "Pb Rb", "Vv Ww Mm"]
SYMS = list("|/\\_[]{}()<>")
LATIN1 = ["é", "ß", "ñ", "ü", "Ä", "Ö", "caf\u00e9", "ni\u00f1o", "Stra\u00dfe", "\u00e9\u00e0\u00e8", "\u00c5ngstr\u00f6m"]
MISSING = ["\u20ac", "\u65e5\u672c", "\U0001F600", "a\u20acb", "\u03bb", "x\u2603y", "\u0416"]
SENTENCES = [
    "the quick brown fox jumps over the lazy dog",
    "shout in giant letters inside a very small window",
    "two giant letters stand next to each other and get squeezed",
    "this sentence must wrap onto several lines without losing words",
    "a b c d e f g h i j k l m n o p",
    "ab cd ef gh ij kl mn op qr st uv wx yz",
    "pneumonoultramicroscopicsilicovolcanoconiosis is a very long word",
]

# ---------------------------------------------------------------- running the ORIGINAL

_appends = 0
_orig_append = pyfiglet.FigletProduct.append


def _guarded_append(self, buffer):
    # pyfiglet loops forever when a glyph is exactly as wide as -w; cut that off deterministically.
    global _appends
    _appends += 1
    if _appends > 5000:
        raise RuntimeError("runaway")
    _orig_append(self, buffer)


pyfiglet.FigletProduct.append = _guarded_append


def run_original(args):
    """Return stdout (str) of pyfiglet.main() for args, or None if it fails in any way."""
    global _appends
    _appends = 0
    real_out, real_err, real_argv = sys.stdout, sys.stderr, sys.argv
    buf = io.BytesIO()
    sys.stdout = io.TextIOWrapper(buf, encoding="utf-8", newline="")
    sys.stderr = io.StringIO()
    sys.argv = ["pyfiglet"] + list(args)
    try:
        rc = pyfiglet.main()   # main() detaches sys.stdout and writes bytes into buf
        data = buf.getvalue()
    except BaseException:
        return None
    finally:
        sys.stdout, sys.stderr, sys.argv = real_out, real_err, real_argv
    if rc != 0:
        return None
    return data.decode("utf-8")


# ---------------------------------------------------------------- building cases


def make_text(font=None):
    r = random.random()
    if font in EXTRA_FONTS and random.random() < 0.35:
        return random.choice(FLIP_TEXTS)
    if r < 0.30:
        return " ".join(random.choice(WORDS) for _ in range(random.randint(1, 3)))
    if r < 0.52:
        return random.choice(TRICKY)
    if r < 0.64:
        n = random.randint(2, 8)
        pool = SYMS + list("ab ilT")
        return "".join(random.choice(pool) for _ in range(n)).strip() or "|_|"
    if r < 0.76:
        return random.choice(SENTENCES)
    if r < 0.82:
        base = " ".join(random.choice(WORDS) for _ in range(random.randint(2, 4)))
        return random.choice(["  " + base, base + "  ", base.replace(" ", "  ", 1), base.replace(" ", "   ", 1),
                              " ".join(random.choice(TRICKY) for _ in range(3)) + "  x"])
    if r < 0.90:
        return random.choice(LATIN1) + random.choice(["", " " + random.choice(WORDS), random.choice(TRICKY)])
    if r < 0.96:
        return random.choice(MISSING) if random.random() < 0.3 else random.choice(WORDS) + random.choice(MISSING)
    return random.choice(WORDS) + "\n" + random.choice(WORDS)


def make_args():
    opts = []
    font = None
    if random.random() < 0.9:
        font = random.choice(EXTRA_FONTS) if random.random() < EXTRA_FONT_SHARE else random.choice(FONTS)
        opts.append(["-f", font])
    if random.random() < 0.8:
        opts.append(["-w", str(random.choice(WIDTHS))])
    if random.random() < 0.6:
        opts.append(["-j", random.choice(JUSTIFY)])
    if random.random() < 0.6:
        opts.append(["-D", random.choice(DIRECTION)])
    for flag in ("-r", "-F", "-s", "-n"):
        if random.random() < 0.25:
            opts.append([flag])
    random.shuffle(opts)
    args = [a for o in opts for a in o]
    text = make_text(font)
    if text.startswith("-"):
        text = "x" + text
    if " " in text and "  " not in text and not text.startswith(" ") and not text.endswith(" ") \
            and "\n" not in text and random.random() < 0.15:
        args.extend(text.split(" "))        # several text arguments -> joined with one space
    else:
        args.append(text)
    return args


def build(total):
    seen, cases = set(), []
    attempts = 0
    while len(cases) < total:
        attempts += 1
        args = make_args()
        key = tuple(args)
        if key in seen:
            continue
        out = run_original(args)
        if out is None:
            continue
        seen.add(key)
        cases.append({"args": args, "out": out})
    return cases, attempts


def stats(name, cases):
    def has(c, flag, val=None):
        a = c["args"]
        if flag not in a:
            return False
        return val is None or a[a.index(flag) + 1] == val

    def width(c):
        a = c["args"]
        return int(a[a.index("-w") + 1]) if "-w" in a else 80

    rtl = [c for c in cases if has(c, "-D", "right-to-left")]
    print(f"{name}: {len(cases)} cases | rtl={len(rtl)} | rtl&narrow(<=40)={sum(width(c) <= 40 for c in rtl)}"
          f" | -j right={sum(has(c, '-j', 'right') for c in cases)} -j center={sum(has(c, '-j', 'center') for c in cases)}"
          f" | -r={sum(has(c, '-r') for c in cases)} -F={sum(has(c, '-F') for c in cases)}"
          f" -s={sum(has(c, '-s') for c in cases)} -n={sum(has(c, '-n') for c in cases)}"
          f" | width<=20={sum(width(c) <= 20 for c in cases)}")
    return len(rtl)


def main():
    cases, attempts = build(2 * N_PER_FILE)
    visible, hidden = cases[:N_PER_FILE], cases[N_PER_FILE:]
    os.makedirs(os.path.join(HERE, "data"), exist_ok=True)
    for name, part in (("visible", visible), ("hidden", hidden)):
        with open(os.path.join(HERE, "data", name + ".jsonl"), "w", encoding="utf-8", newline="\n") as f:
            for c in part:
                f.write(json.dumps(c, ensure_ascii=True) + "\n")
        n_rtl = stats(name, part)
        assert n_rtl >= 300, "too few right-to-left cases: the hard rules would not be tested"
    print(f"(kept {len(cases)} of {attempts} attempts; the rest crashed/looped in the original and were dropped)")


if __name__ == "__main__":
    main()

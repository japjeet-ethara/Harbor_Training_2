#!/usr/bin/env python3
"""Generate the Harbor exam for figlet-go.  Runs on the task author's laptop.

Needs the pyfiglet checkout from Step 1 (commit 255827ef...) in a folder called
`pyfiglet` next to this task folder (or set PYFIGLET_DIR).  Every expected
output ("out") is produced by running the ORIGINAL pyfiglet.main() with the
very same arguments and capturing its stdout bytes.  Nothing is written by hand.

    python3 tests/gen.py

Writes tests/data/visible.jsonl and tests/data/hidden.jsonl (2000 cases each).
The two files never share a case (same args), so a program that only memorised
visible.jsonl scores zero on hidden.jsonl.
"""
import io
import json
import os
import random
import shutil
import signal
import sys
from collections import Counter
from pathlib import Path

HERE = Path(__file__).resolve().parent
TASK = HERE.parent
PYFIGLET_DIR = Path(os.environ.get("PYFIGLET_DIR", TASK.parent / "pyfiglet")).resolve()
N = 2000
TIMEOUT = 3  # seconds; the original loops forever when -w equals a glyph's width

# --- make sure the original is runnable (Makefile "minimal" font step) -------
sys.dont_write_bytecode = True
fonts_dir = PYFIGLET_DIR / "pyfiglet" / "fonts"
if not fonts_dir.is_dir():
    fonts_dir.mkdir(parents=True)
    for f in (PYFIGLET_DIR / "pyfiglet" / "fonts-standard").iterdir():
        shutil.copy(f, fonts_dir)
sys.path.insert(0, str(PYFIGLET_DIR))
import pyfiglet  # noqa: E402  (the ORIGINAL, from the folder above)


# --- the original, as a function ---------------------------------------------
class Timeout(Exception):
    pass


def _alarm(signum, frame):
    raise Timeout()


signal.signal(signal.SIGALRM, _alarm)


def run_original(args):
    """Run pyfiglet.main() with `args`; return its stdout as str, or None if it
    crashed, exited non-zero, or hung (those inputs are not part of the exam)."""
    raw = io.BytesIO()
    real_out, real_err, real_argv = sys.stdout, sys.stderr, sys.argv
    sys.stdout = io.TextIOWrapper(raw, encoding="utf-8")  # main() detach()es it
    sys.stderr = io.StringIO()
    sys.argv = ["pyfiglet"] + list(args)
    signal.alarm(TIMEOUT)
    try:
        code = pyfiglet.main()
    except BaseException:  # errors, SystemExit from optparse, Timeout
        return None
    finally:
        signal.alarm(0)
        sys.stdout, sys.stderr, sys.argv = real_out, real_err, real_argv
    if code not in (0, None):
        return None
    try:
        return raw.getvalue().decode("utf-8")
    except UnicodeDecodeError:
        return None


# --- the case recipe ----------------------------------------------------------
FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = ["hello", "world", "Harbor", "figlet", "go", "Python", "slash", "x", "HI",
         "abc", "ABC", "xyz", "Test", "123", "a", "zebra", "Quick", "mix", "MAX",
         "pyfiglet", "Rust", "gopher", "waves", "PRMVW", "bug", "ship", "dock", "42"]

# the "rare cases" list from NOTES.md (every one of these is a trap for a port)
RARE = ["|_|", "/\\", "\\/", "/ \\", "\\ /", "[]", "][", "{}", "}{", "()", ")(", "<>", "><",
        "_|_", "|||", "[ ]", "{ }", "(  )", "<<>>", "__", "-_-", "|/\\|", "[{()}]",
        "_/\\_", "^v^", "PRMVW", "AMWV", "mwvb", "/\\/\\/\\", "||__||",
        "  double  spaces  ", "a  b   c", " lead", "trail ", " ", "!", ".", "'", "i", "l",
        "é ß ñ", "Ärger über Öl", "ä ö ü", "日本 go", "ß", "\u00a0nbsp"]
SYMBOLS = list("|/\\_[]{}()<>")
SENTENCES = ["the quick brown fox jumps over the lazy dog",
             "pack my box with five dozen liquor jugs",
             "wrap this long sentence please",
             "a b c d e f g h i j k l m n o p",
             "Harbor tasks are fun to build",
             "hello hello hello hello hello",
             "short words wrap at spaces only",
             "Supercalifragilisticexpialidocious is long"]
NEWLINE_TEXTS = ["hi\nthere", "a\n\nb", "line one\nline two", "\nlead", "trail\n", "x\ny\nz"]


def make_text():
    r = random.random()
    if r < 0.22:
        t = random.choice(WORDS)
    elif r < 0.32:
        t = " ".join(random.choice(WORDS) for _ in range(random.randint(2, 4)))
    elif r < 0.52:
        t = random.choice(RARE)
    elif r < 0.64:
        t = "".join(random.choice(SYMBOLS) for _ in range(random.randint(2, 8)))
    elif r < 0.72:
        t = "".join(random.choice(SYMBOLS + list("abAB _ ")) for _ in range(random.randint(3, 9)))
    elif r < 0.88:
        t = random.choice(SENTENCES)
    elif r < 0.94:
        t = " ".join(random.choice(WORDS) for _ in range(random.randint(5, 9)))
    else:
        t = random.choice(NEWLINE_TEXTS)
    if t.startswith("-"):  # optparse would read it as an option
        t = "_" + t[1:]
    return t


def make_args():
    opts = ["-f", random.choice(FONTS)]
    extra = []
    if random.random() < 0.8:
        extra.append(["-w", str(random.choice(WIDTHS))])
    if random.random() < 0.6:
        extra.append(["-j", random.choice(JUSTIFY)])
    if random.random() < 0.7:
        extra.append(["-D", random.choice(DIRECTION)])
    for flag in ("-r", "-F", "-s", "-n"):
        if random.random() < 0.25:
            extra.append([flag])
    random.shuffle(extra)
    for e in extra:
        opts += e
    text = make_text()
    if " " in text.strip() and random.random() < 0.25:
        return opts + text.split(" ")  # several words as several arguments
    return opts + [text]


def make_cases(n, forbidden):
    cases, seen = [], set(forbidden)
    rejected = Counter()
    while len(cases) < n:
        args = make_args()
        key = tuple(args)
        if key in seen:
            rejected["duplicate"] += 1
            continue
        seen.add(key)
        out = run_original(args)
        if out is None:
            rejected["original failed or hung"] += 1
            continue
        cases.append({"args": args, "out": out})
    return cases, rejected


def write(path, cases):
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        for c in cases:
            f.write(json.dumps(c, ensure_ascii=True) + "\n")


def stats(name, cases, rejected):
    c = Counter()
    for case in cases:
        a = case["args"]
        c["right-to-left"] += "right-to-left" in a
        c["narrow (-w 10/20)"] += any(x == "-w" and a[i + 1] in ("10", "20") for i, x in enumerate(a))
        for fl in ("-r", "-F", "-s", "-n"):
            c[fl] += fl in a
        c["with newline"] += any("\n" in x for x in a)
        c["blank output"] += case["out"].strip() == ""
    print(f"{name}: {len(cases)} cases; rejected: {dict(rejected)}")
    print("   ", dict(c))


if __name__ == "__main__":
    random.seed(7)
    data = TASK / "tests" / "data"
    data.mkdir(parents=True, exist_ok=True)
    visible, rej_v = make_cases(N, forbidden=[])
    hidden, rej_h = make_cases(N, forbidden=[tuple(c["args"]) for c in visible])
    assert not {tuple(c["args"]) for c in visible} & {tuple(c["args"]) for c in hidden}
    write(data / "visible.jsonl", visible)
    write(data / "hidden.jsonl", hidden)
    stats("visible", visible, rej_v)
    stats("hidden ", hidden, rej_h)

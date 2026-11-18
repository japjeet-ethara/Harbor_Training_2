#!/usr/bin/env python3
"""
Generate the visible and hidden test cases for the figlet-go Harbor task.

Runs on the AUTHOR'S laptop (never in Docker). It needs the original pyfiglet
checkout (commit 255827efbc09d662875dad50fa0edc1e1263bf45) in a folder called
"pyfiglet" next to the task folder, or in the folder named by $PYFIGLET_DIR.

Every "out" is produced by calling the ORIGINAL pyfiglet.main() with the same
arguments and capturing its stdout bytes. Nothing is ever written by hand.

    python3 tests/gen.py            # 2000 + 2000 cases, seed 7
    python3 tests/gen.py --n 200    # smaller run for a quick look
"""
import argparse
import io
import json
import os
import random
import shutil
import signal
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
TASK = os.path.dirname(HERE)
SEED = 7
TIMEOUT_SECONDS = 1  # the original loops forever on a few width/char combinations


def find_pyfiglet():
    for cand in (os.environ.get("PYFIGLET_DIR"), os.path.join(TASK, "..", "pyfiglet")):
        if cand and os.path.isdir(os.path.join(cand, "pyfiglet")):
            return os.path.abspath(cand)
    sys.exit('Cannot find the pyfiglet clone. Put it next to the task folder '
             'or set PYFIGLET_DIR=/path/to/pyfiglet')


ROOT = find_pyfiglet()
sys.path.insert(0, ROOT)

# Step 1's font setup (what the project's Makefile "minimal" target does).
std = os.path.join(ROOT, "pyfiglet", "fonts-standard")
fonts = os.path.join(ROOT, "pyfiglet", "fonts")
if not os.path.isdir(fonts) or not os.listdir(fonts):
    print("fonts/ is missing: copying fonts-standard -> fonts (Makefile 'minimal' step)")
    os.makedirs(fonts, exist_ok=True)
    for name in os.listdir(std):
        shutil.copy(os.path.join(std, name), fonts)

import pyfiglet  # noqa: E402  (must come after sys.path is set)

# ---------------------------------------------------------------- ingredients
FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = ["hello", "world", "figlet", "go", "Harbor", "task", "ABC", "xyz", "Python",
         "smush", "kern", "Zebra", "quick", "brown", "fox", "jumps", "over", "lazy", "dog",
         "0123456789", "Hello, World!", "a", "I", "mm", "WW", "ij", "Hi there", "go touch",
         "Test 42", "foo.bar", "x-y", "a+b=c", "100%", "#1", "q?", "end."]

# The rare-cases list (see NOTES.md "Rare cases"): places where a port is easiest to get wrong.
RARE = [
    "|_|", "_|_", "||", "|/\\|", "/\\", "\\/", "/\\/\\", "><", "<>", "[]", "{}", "()", "][", "}{", ")(",
    "[|]", "{|}", "(|)", "__", "_|", "|_", "//", "\\\\", "/|\\", "<|>", "-_-", "(_)", "[_]", "{_}",
    "a\nb", "\n", "a\n\nb", "abc\n", "\nabc", "a\n b\nc", "hello\nworld",
    "  ", " a ", "a  b", "a   b   c", "    x", "x    ", "a b  c   d",
    "", " ",
    "$", "$$$", "a$b", "@", "@@", "%%%", "#", "~",
    "é", "ß", "ñ", "Ärger", "über", "Straße", "ÄÖÜäöü", "café", "señor", "Ünï",
    "日本語", "€", "ab€cd", "naïve — dash", "\t", "tab\tinside",
    "i", "l", "I", "1", "!", ".", ",", "'", "`", ":", ";",
    "supercalifragilisticexpialidocious", "Pneumonoultramicroscopic",
]
SYMBOLS = "|/\\_[]{}()<>"

SENTENCES = [
    "the quick brown fox jumps over the lazy dog",
    "Pack my box with five dozen liquor jugs",
    "Sphinx of black quartz, judge my vow",
    "hello world this is a long sentence that must wrap around",
    "a b c d e f g h i j k l m n o p q r s t u v w x y z",
    "wrap me please, wrap me again, and once more with feeling",
]


def make_text(rng):
    kind = rng.random()
    if kind < 0.30:
        return rng.choice(WORDS)
    if kind < 0.50:
        return rng.choice(RARE)
    if kind < 0.65:
        n = rng.randint(2, 9)
        return "".join(rng.choice(SYMBOLS) for _ in range(n))
    if kind < 0.75:
        # symbols mixed with letters and spaces
        pool = SYMBOLS + "  ab"
        return "".join(rng.choice(pool) for _ in range(rng.randint(3, 10))).strip("-")
    if kind < 0.85:
        return rng.choice(SENTENCES)
    if kind < 0.93:
        words = [rng.choice(WORDS) for _ in range(rng.randint(2, 6))]
        sep = rng.choice([" ", "  ", " "])
        return sep.join(words)
    # random mix of printable ASCII + a few Latin-1 letters
    pool = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 .,;:!?éßñÄ"
    return "".join(rng.choice(pool) for _ in range(rng.randint(1, 14)))


def make_args(rng):
    args = []
    # each option is present in about 85% of the cases; the rest use the defaults
    if rng.random() < 0.85:
        args += ["-f", rng.choice(FONTS)]
    if rng.random() < 0.85:
        args += ["-w", str(rng.choice(WIDTHS))]
    if rng.random() < 0.85:
        args += ["-j", rng.choice(JUSTIFY)]
    if rng.random() < 0.85:
        args += ["-D", rng.choice(DIRECTION)]
    flags = ["-r", "-F", "-s", "-n"]
    rng.shuffle(flags)
    for flag in flags:
        if rng.random() < 0.25:
            args.append(flag)
    text = make_text(rng)
    if text.startswith("-"):
        text = "x" + text  # a leading "-" would be read as an option
    if rng.random() < 0.08 and " " in text.strip(" "):
        # the same words passed as separate arguments (pyfiglet joins them with one space)
        args += text.split(" ")
    else:
        args.append(text)
    return args


# ------------------------------------------------------- run the ORIGINAL
class Timeout(Exception):
    pass


def _on_alarm(signum, frame):
    raise Timeout()


def run_original(args):
    """Return stdout (str) of the original pyfiglet.main(), or None if it fails or hangs."""
    old_argv, old_stdout = sys.argv, sys.stdout
    sys.argv = ["pyfiglet"] + args
    sys.stdout = io.TextIOWrapper(io.BytesIO(), encoding="utf-8")
    signal.signal(signal.SIGALRM, _on_alarm)
    signal.setitimer(signal.ITIMER_REAL, TIMEOUT_SECONDS)
    try:
        code = pyfiglet.main()
        signal.setitimer(signal.ITIMER_REAL, 0)
        if code not in (0, None):
            return None
        # main() detaches stdout, so sys.stdout is now the underlying BytesIO
        return sys.stdout.getvalue().decode("utf-8")
    except BaseException:  # error, SystemExit from optparse, Timeout ...
        return None
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        sys.argv, sys.stdout = old_argv, old_stdout


def make_cases(rng, count, seen):
    cases = []
    tried = rejected = 0
    while len(cases) < count:
        args = make_args(rng)
        tried += 1
        key = json.dumps(args)
        if key in seen:
            continue
        out = run_original(args)
        if out is None:
            rejected += 1  # pyfiglet itself failed/hung on this input: not gradable
            continue
        seen.add(key)
        cases.append({"args": args, "out": out})
    return cases, tried, rejected


def stats(name, cases):
    def has(case, flag):
        return flag in case["args"]

    def val(case, flag):
        a = case["args"]
        return a[a.index(flag) + 1] if flag in a else None

    rtl = sum(1 for c in cases if val(c, "-D") == "right-to-left")
    narrow = sum(1 for c in cases if val(c, "-w") in ("10", "20"))
    both = sum(1 for c in cases if val(c, "-D") == "right-to-left" and val(c, "-w") in ("10", "20", "40"))
    wrapped = sum(1 for c in cases if c["out"].count("\n") > 12)
    print(f"{name}: {len(cases)} cases | right-to-left {rtl} | width<=20 {narrow} | "
          f"rtl with width<=40 {both} | multi-line output {wrapped} | "
          f"-r {sum(has(c,'-r') for c in cases)} -F {sum(has(c,'-F') for c in cases)} "
          f"-s {sum(has(c,'-s') for c in cases)} -n {sum(has(c,'-n') for c in cases)}")


def write(path, cases):
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        for case in cases:
            f.write(json.dumps(case, ensure_ascii=False) + "\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--n", type=int, default=2000, help="cases per file (default 2000)")
    opts = ap.parse_args()

    random.seed(SEED)       # the task asks for random.seed(7)
    rng = random            # every random choice below goes through the seeded module
    seen = set()            # hidden cases never repeat a visible case (and vice versa)

    visible, t1, r1 = make_cases(rng, opts.n, seen)
    hidden, t2, r2 = make_cases(rng, opts.n, seen)

    os.makedirs(os.path.join(HERE, "data"), exist_ok=True)
    write(os.path.join(HERE, "data", "visible.jsonl"), visible)
    write(os.path.join(HERE, "data", "hidden.jsonl"), hidden)
    stats("visible", visible)
    stats("hidden ", hidden)
    print(f"(skipped {r1 + r2} inputs on which the original itself fails or hangs)")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Generate tests/data/{visible,hidden}.jsonl by running the ORIGINAL pyfiglet.

Runs on the task author's machine, not in Docker. Expects the pyfiglet checkout
(commit 255827ef) next to the task folder:  <parent>/pyfiglet and <parent>/figlet-go.
Every "out" is captured from pyfiglet.main(); nothing is written by hand.
"""
import io
import json
import os
import random
import sys
import threading
import time

HERE = os.path.dirname(os.path.abspath(__file__))
PYFIGLET_DIR = os.path.normpath(os.path.join(HERE, "..", "..", "pyfiglet"))
sys.path.insert(0, PYFIGLET_DIR)
import pyfiglet  # noqa: E402
from pyfiglet import FigletFont  # noqa: E402

OUT_DIR = os.path.join(HERE, "data")
N_PER_FILE = 2000

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]
FLAGS = ["-r", "-F", "-s", "-n"]

WORDS = [
    "hello", "world", "figlet", "go", "python", "harbor", "banner", "shout",
    "Gunjit", "Hello World", "PORT", "Twin", "smush", "kern", "wrap", "x",
    "I", "l", "1", "0", "42", "2026", "3.14", "a.b", "Z", "MW", "mw", "VvAa",
    "pbqd", "PR", "AM", "quick", "brown", "fox", "jumps", "over", "lazy", "dog",
    "The", "end", "of", "the", "line", "is", "near", "and", "it", "wraps",
]
TRICKY = [
    "||", "//", "_|", "|_", "[]", "{}", "()", "<>", "><", "/\\", "\\/", "|_|",
    "[{(<", ">)}]", "_/_", "|/|", "__", "_", "|", "/", "\\", "[", "]", "{", "}",
    "(", ")", "<", ">", "a  b", "ab  cd  ef", "hello ", " hello", "", "   ",
    "i|l", ".", ",", "'", "\"", "`", "~", "^", "@#$%&*", "+=-", "?!", ":;",
    "ÄÖÜäöüß", "é ß ñ", "café", "niño", "straße", "日本", "€100", "a→b",
    "tab\tsep", "a b", "|_|_|_|", "/\\/\\/\\", "()()", "[[]]", "{{}}",
    "<<>>", "_____", "|||||", "/////", "\\\\\\\\\\", "o_O", "^_^", ">_<", "(_)",
]
SENTENCES = [
    "the quick brown fox jumps over the lazy dog",
    "Pack my box with five dozen liquor jugs",
    "a line that is long enough to wrap more than once in a narrow window",
    "one two three four five six seven eight nine ten eleven twelve",
    "|_| |_| |_| |_| |_| |_| |_| |_|",
    "[] {} () <> [] {} () <> [] {} () <>",
    "ab  cd  ef  gh  ij  kl  mn  op",
    "A B C D E F G H I J K L M N O P",
]


_current = [None, 0.0]


def _watchdog():
    while True:
        time.sleep(5)
        args, started = _current
        if args is not None and time.time() - started > 10:
            sys.stderr.write("HANG in original pyfiglet on %r\n" % (args,))
            os._exit(3)


def run_original(args):
    """Run pyfiglet.main() with argv=args in-process; return stdout bytes or None."""
    _current[0], _current[1] = args, time.time()
    saved_stdout, saved_argv = sys.stdout, sys.argv
    buf = io.BytesIO()
    # main() calls sys.stdout.detach() and writes bytes to the underlying buffer.
    sys.stdout = io.TextIOWrapper(buf, encoding="utf-8")
    sys.argv = ["pyfiglet"] + list(args)
    try:
        rc = pyfiglet.main()
    except Exception:
        rc = None
    finally:
        sys.stdout, sys.argv = saved_stdout, saved_argv
        _current[0] = None
    if rc != 0:
        return None
    return buf.getvalue()


_font_cache = {}


def font_info(name):
    if name not in _font_cache:
        f = FigletFont(name)
        lead = {c: min(len(r) - len(r.lstrip(" ")) for r in rows) for c, rows in f.chars.items()}
        trail = {c: min(len(r) - len(r.rstrip(" ")) for r in rows) for c, rows in f.chars.items()}
        _font_cache[name] = (f.width, lead, trail)
    return _font_cache[name]


def would_hang(font, width, text, rtl):
    """pyfiglet loops forever on a glyph exactly `width` wide whose first-glyph trim
    removes nothing: no leading blank column (left-to-right) or no trailing one (right-to-left)."""
    widths, lead, trail = font_info(font)
    edge = trail if rtl else lead
    return any(widths.get(ord(ch)) == width and edge[ord(ch)] == 0 for ch in text)


def make_text(rng):
    kind = rng.random()
    if kind < 0.25:
        return rng.choice(WORDS)
    if kind < 0.55:
        return rng.choice(TRICKY)
    if kind < 0.70:
        return rng.choice(SENTENCES)
    n = rng.randint(2, 4)
    parts = [rng.choice(WORDS + TRICKY) for _ in range(n)]
    return (rng.choice([" ", " ", "  "])).join(parts)


def make_args(rng):
    font = rng.choice(FONTS)
    args = ["-f", font]
    width = 80
    if rng.random() < 0.8:
        width = rng.choice(WIDTHS)
        args += ["-w", str(width)]
    if rng.random() < 0.6:
        args += ["-j", rng.choice(JUSTIFY)]
    rtl = False
    if rng.random() < 0.6:
        direction = rng.choice(DIRECTION)
        args += ["-D", direction]
        rtl = direction == "right-to-left"  # all eight fonts print left-to-right on "auto"
    for flag in FLAGS:
        if rng.random() < 0.25:
            args.append(flag)
    text = make_text(rng)
    if would_hang(font, width, text, rtl):
        return None
    # the text goes in as one argument, or split on spaces into several (main joins them back)
    if " " in text and rng.random() < 0.3:
        text_args = text.split(" ")
    else:
        text_args = [text]
    if any(t.startswith("-") for t in text_args):
        return None  # optparse would read it as an option
    return args + text_args


def main():
    threading.Thread(target=_watchdog, daemon=True).start()
    rng = random.Random(7)
    seen = set()
    cases = []
    attempts = 0
    while len(cases) < 2 * N_PER_FILE:
        attempts += 1
        args = make_args(rng)
        if args is None:
            continue
        key = tuple(args)
        if key in seen:
            continue
        out = run_original(args)
        if out is None:
            continue  # CharNotPrinted: no answer exists for this width
        seen.add(key)
        cases.append({"args": args, "out": out.decode("utf-8")})

    os.makedirs(OUT_DIR, exist_ok=True)
    for name, chunk in (("visible", cases[:N_PER_FILE]), ("hidden", cases[N_PER_FILE:])):
        path = os.path.join(OUT_DIR, name + ".jsonl")
        with open(path, "w", encoding="utf-8", newline="\n") as fh:
            for case in chunk:
                fh.write(json.dumps(case) + "\n")
        rtl = sum(1 for c in chunk if "right-to-left" in c["args"])
        narrow = sum(1 for c in chunk if "-w" in c["args"] and c["args"][c["args"].index("-w") + 1] in ("10", "20"))
        print(f"{path}: {len(chunk)} cases, {rtl} right-to-left, {narrow} with width 10/20")
    print(f"{attempts} attempts for {len(cases)} cases")


if __name__ == "__main__":
    main()

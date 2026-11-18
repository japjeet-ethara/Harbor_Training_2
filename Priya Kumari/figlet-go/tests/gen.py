#!/usr/bin/env python3
"""
Generate visible and hidden test-case JSONL files for the FIGlet-Go Harbor task.

This script runs on the developer's Windows laptop.  It invokes the *original*
pyfiglet (located at C:\\Users\\PriyaKumari\\pyfiglet) via its Python API to
capture byte-exact expected output.  Every expected output is produced by
pyfiglet itself — nothing is written by hand.

Usage:
    cd C:\\Users\\PriyaKumari\\figlet-go
    python tests/gen.py

Outputs:
    tests/data/visible.jsonl   (2 000 cases)
    tests/data/hidden.jsonl    (2 000 cases)
"""

import concurrent.futures
import json
import os
import random
import sys
from pathlib import Path

# ---------------------------------------------------------------------------
# Bootstrap: make the *original* pyfiglet importable
# ---------------------------------------------------------------------------

PYFIGLET_DIR = str(Path(r"C:\Users\PriyaKumari\pyfiglet"))
if PYFIGLET_DIR not in sys.path:
    sys.path.insert(0, PYFIGLET_DIR)

from pyfiglet import Figlet, FigletString  # noqa: E402

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

REPO_DIR = Path(r"C:\Users\PriyaKumari\figlet-go")
DATA_DIR = REPO_DIR / "tests" / "data"

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFICATIONS = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]

# Minimum width that is safe for each font (avoids CharNotPrinted errors
# for characters wider than the column limit).  Determined empirically.
# Fonts with very wide glyphs need more columns.
FONT_MIN_WIDTH = {
    "standard": 20,
    "slant": 20,
    "small": 20,
    "big": 20,
    "mini": 10,
    "banner": 20,
    "shadow": 20,
    "script": 20,
}

# ---------------------------------------------------------------------------
# Input texts
# ---------------------------------------------------------------------------

NORMAL_WORDS = [
    "Hello",
    "World",
    "FIGlet",
    "Go",
    "Harbor",
    "Test",
    "ASCII",
    "Art",
    "Python",
    "Render",
    "Font",
    "Banner",
    "Shadow",
    "Slant",
    "Mini",
    "Script",
    "Quick brown fox",
    "Lazy dog",
    "ABCDEFGHIJ",
    "klmnopqrst",
    "uvwxyz",
    "0123456789",
    "Hello World",
    "Foo Bar Baz",
    "OpenAI",
    "Antigravity",
    "Smushing",
    "Kerning",
    "Overlap",
    "Justified",
]

TRICKY_INPUTS = [
    # Individual smushing-relevant characters
    "|",
    "/",
    "\\",
    "_",
    "[",
    "]",
    "{",
    "}",
    "(",
    ")",
    "<",
    ">",
    # Pairs that trigger specific smush rules
    "||",
    "/\\",
    "\\/",
    "><",
    "<>",
    "[]",
    "{}",
    "()",
    "[}",
    "{]",
    "(<",
    ">)",
    # Mixed smushing characters in words
    "|/\\_[]{}()<>",
    "a|b/c\\d",
    "[Hello]",
    "{World}",
    "(Test)",
    "<FIGlet>",
    "a_b_c",
    "X|Y|Z",
    # Double spaces
    "Hello  World",
    "A  B  C",
    "double  space  test",
    # Latin-1 characters
    "caf\u00e9",
    "na\u00efve",
    "r\u00e9sum\u00e9",
    "\u00fcber",
    "stra\u00dfe",
    "jalape\u00f1o",
    "se\u00f1or",
    "\u00c4rger",
    "\u00d6lpreis",
    "\u00d1and\u00fa",
    "cr\u00e8me br\u00fbl\u00e9e",
    "\u00e0 la carte",
    "expos\u00e9",
    "pi\u00f1ata",
    "t\u00eate-\u00e0-t\u00eate",
    # Characters unsupported by most fonts (high Unicode)
    "\u03b1\u03b2\u03b3",
    "\u65e5\u672c\u8a9e",
    "\U0001f389",
    "\u2192\u2190\u2191\u2193",
    "\u2605\u2713\u2717",
    # Mixed normal + tricky
    "Hello | World",
    "left < right >",
    "a/b\\c",
    "open(close)",
    "begin{end}",
    "start[stop]",
    "one_two_three",
    # Single characters
    "A",
    "Z",
    "a",
    "z",
    "0",
    "9",
    "!",
    "@",
    "#",
    "$",
    "%",
    "^",
    "&",
    "*",
    "~",
    " ",
    # Whitespace-ish
    "   ",
]

# Long sentences — only used with wider widths to avoid hangs / errors.
LONG_INPUTS = [
    "The quick brown fox jumps over the lazy dog",
    "Pack my box with five dozen liquor jugs",
    "How valiantly the quick brown fox jumped over the lazy sleeping dog",
    "A very long sentence that should definitely wrap at smaller column widths to test wrapping behavior properly",
    "Sphinx of black quartz judge my vow and pack my box with five dozen liquor jugs",
    "The five boxing wizards jump quickly over the lazy brown dog sleeping in the sun",
]

SHORT_INPUTS = NORMAL_WORDS + TRICKY_INPUTS

# ---------------------------------------------------------------------------
# Rendering helper
# ---------------------------------------------------------------------------

# A thread-pool executor used to enforce timeouts (Windows lacks SIGALRM).
_executor = concurrent.futures.ThreadPoolExecutor(max_workers=1)

RENDER_TIMEOUT = 10  # seconds


def _render_in_thread(font, width, direction, justify, text,
                      reverse, flip, strip_nl, normalize_nl):
    """Render one case using pyfiglet's Python API.  Runs inside a thread
    so we can enforce a timeout on Windows."""
    f = Figlet(font=font, direction=direction, justify=justify, width=width)
    r = f.renderText(text)

    if reverse:
        r = r.reverse()
    if flip:
        r = r.flip()
    if strip_nl:
        r = r.strip_surrounding_newlines()
    elif normalize_nl:
        r = r.normalize_surrounding_newlines()

    # pyfiglet's CLI writes: r.encode('UTF-8') + b'\n'
    return r + "\n"


def render_case(font, width, direction, justify, text,
                reverse, flip, strip_nl, normalize_nl):
    """Render with a timeout.  Returns the output string or None on failure."""
    future = _executor.submit(
        _render_in_thread,
        font, width, direction, justify, text,
        reverse, flip, strip_nl, normalize_nl,
    )
    try:
        return future.result(timeout=RENDER_TIMEOUT)
    except Exception:
        future.cancel()
        return None


# ---------------------------------------------------------------------------
# Case generation
# ---------------------------------------------------------------------------

def make_case(rng: random.Random) -> dict | None:
    """Build one random test case and return {"args": [...], "out": "..."},
    or None if pyfiglet failed / timed out on this input."""
    font = rng.choice(FONTS)
    justify = rng.choice(JUSTIFICATIONS)
    direction = rng.choice(DIRECTIONS)

    use_reverse = rng.random() < 0.25
    use_flip = rng.random() < 0.25
    use_strip = rng.random() < 0.25
    use_normalize = rng.random() < 0.25

    # Decide whether to use a long input (only with wider widths).
    use_long = rng.random() < 0.15
    if use_long:
        text = rng.choice(LONG_INPUTS)
        # Long inputs need wider columns to avoid hangs.
        safe_widths = [w for w in WIDTHS if w >= 40]
        width = rng.choice(safe_widths)
    else:
        text = rng.choice(SHORT_INPUTS)
        min_w = FONT_MIN_WIDTH.get(font, 20)
        safe_widths = [w for w in WIDTHS if w >= min_w]
        width = rng.choice(safe_widths)

    # Build the argument list (mirrors the CLI interface).
    args = ["-f", font, "-w", str(width), "-j", justify, "-D", direction]
    if use_reverse:
        args.append("-r")
    if use_flip:
        args.append("-F")
    # -s takes priority over -n (matches pyfiglet behavior).
    if use_strip:
        args.append("-s")
    elif use_normalize:
        args.append("-n")
    args.append(text)

    out = render_case(font, width, direction, justify, text,
                      use_reverse, use_flip, use_strip,
                      use_normalize if not use_strip else False)
    if out is None:
        return None

    return {"args": args, "out": out}


def generate_cases(rng: random.Random, count: int, label: str) -> list[dict]:
    """Generate exactly `count` valid test cases."""
    cases: list[dict] = []
    skipped = 0
    while len(cases) < count:
        case = make_case(rng)
        if case is not None:
            cases.append(case)
            if len(cases) % 200 == 0:
                print(f"  [{label}] {len(cases)}/{count} generated"
                      f" ({skipped} skipped)")
        else:
            skipped += 1
    print(f"  [{label}] {count}/{count} done ({skipped} total skipped)")
    return cases


def write_jsonl(path: Path, cases: list[dict]) -> None:
    """Write cases as one JSON object per line."""
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        for case in cases:
            f.write(json.dumps(case, ensure_ascii=False) + "\n")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    # Deterministic seed
    rng = random.Random(7)

    # Ensure output directory exists
    DATA_DIR.mkdir(parents=True, exist_ok=True)

    print("Generating 2000 visible cases ...")
    visible = generate_cases(rng, 2000, "visible")
    visible_path = DATA_DIR / "visible.jsonl"
    write_jsonl(visible_path, visible)
    print(f"  Written to {visible_path}")

    print("Generating 2000 hidden cases ...")
    hidden = generate_cases(rng, 2000, "hidden")
    hidden_path = DATA_DIR / "hidden.jsonl"
    write_jsonl(hidden_path, hidden)
    print(f"  Written to {hidden_path}")

    _executor.shutdown(wait=False)
    print("Done.")


if __name__ == "__main__":
    main()

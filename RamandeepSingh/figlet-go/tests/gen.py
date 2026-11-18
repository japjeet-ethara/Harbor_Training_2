#!/usr/bin/env python3
"""
gen.py – deterministic test-case generator for the figlet-go Harbor task.

Generates two JSONL files:
  tests/visible.jsonl  – 2,000 cases (used for feedback during development)
  tests/hidden.jsonl   – 2,000 cases (used for final grading)

Each line is a JSON object:
  {
    "args": ["<flag>", "<value>", ...],   # list of CLI args passed to the binary
    "stdout": "<expected output string>"  # exact bytes the binary must emit
  }

The expected output is produced by calling pyfiglet's own main() function
through its public Python API so that the result matches the CLI byte-for-byte.

Run from the repository root (the directory that contains pyfiglet/):
  python tests/gen.py
"""

import json
import os
import random
import sys
import io

# ---------------------------------------------------------------------------
# Locate repository root (the directory that contains pyfiglet/)
# ---------------------------------------------------------------------------
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))   # …/tests/
REPO_ROOT   = os.path.dirname(SCRIPT_DIR)                 # …/figlet-go/ parent
# pyfiglet lives one level above figlet-go in the workspace layout
PYFIGLET_ROOT = os.path.dirname(REPO_ROOT)                # …/pyfiglet/

# Add pyfiglet source to sys.path so we can import it without installing.
for candidate in [
    PYFIGLET_ROOT,
    REPO_ROOT,
    os.path.join(REPO_ROOT, "pyfiglet"),
    r"C:\Users\RamandeepSingh\.gemini\antigravity\brain\90a4cdf7-7b96-4b28-946b-43315f9f576c\scratch",
]:
    if candidate and os.path.isdir(candidate) and candidate not in sys.path:
        sys.path.insert(0, candidate)

# ---------------------------------------------------------------------------
# Import pyfiglet internals (after path fixup)
# ---------------------------------------------------------------------------
import pyfiglet as _pf
from pyfiglet import Figlet, FigletString, FontNotFound, CharNotPrinted

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

# All .flf fonts present in pyfiglet/fonts-standard/
ALL_FONTS = [
    "5x7", "5x8", "6x9", "banner", "big", "block", "blocky_25", "bubble",
    "chartr", "chartri", "clb6x10", "clb8x10", "clb8x8", "cli8x8",
    "clr4x6", "clr5x10", "clr5x6", "clr5x8", "clr6x10", "clr6x6",
    "clr6x8", "clr7x10", "clr7x8", "clr8x10", "clr8x8",
    "cour", "courb", "courbi", "couri",
    "digital", "helv", "helvb", "helvbi", "helvi",
    "ivrit", "lean", "mini", "mnemonic",
    "sbook", "sbookb", "sbookbi", "sbooki",
    "script", "shadow", "slant", "small", "smscript", "smshadow", "smslant",
    "standard", "term", "times",
    "xchartr", "xchartri", "xcour", "xcourb", "xcourbi", "xcouri",
    "xhelv", "xhelvb", "xhelvbi", "xhelvi",
    "xsbook", "xsbookb", "xsbookbi", "xsbooki", "xtimes",
]

# Fonts explicitly required by the task spec (always included in pool)
REQUIRED_FONTS = [
    "standard", "slant", "small", "big", "mini",
    "banner", "shadow", "script",
]

WIDTHS       = [10, 20, 40, 60, 80, 120]
JUSTIFIES    = ["auto", "left", "center", "right"]
DIRECTIONS   = ["auto", "left-to-right", "right-to-left"]

# Input string pools
ORDINARY_WORDS  = [
    "hello", "world", "foo", "bar", "test", "Go", "FIGlet", "Harbor",
    "agent", "build", "output", "font", "render", "ascii", "art",
]
NUMBERS         = ["0", "42", "1234", "007", "3.14", "2026", "100"]
MIXED_CASE      = ["Hello", "WORLD", "CamelCase", "mixED", "GoLang"]
PUNCTUATION     = ["Hi!", "What?", "A+B", "1+1=2", "Yes.", "No..."]
SPECIAL_CHARS   = ["#1", "@home", "$100", "%done", "C++", "A&B", "x<y"]
REPEATED_SPACE  = ["a b", "foo  bar", "one   two", "x  y  z"]
WRAPPING_CASES  = [
    "The quick brown fox",
    "Pack my box with five dozen liquor jugs",
    "How vexingly quick daft zebras jump",
    "Sphinx of black quartz judge my vow",
]
LATIN1_CHARS    = ["caf\u00e9", "na\u00efve", "r\u00e9sum\u00e9", "\u00e9t\u00e9", "M\u00fcnchen"]
UNSUPPORTED     = ["\u03b1\u03b2\u03b3", "\u4e2d\u6587", "\u00df\u00e4\u00f6\u00fc"]

ALL_INPUTS = (
    ORDINARY_WORDS * 6
    + NUMBERS * 3
    + MIXED_CASE * 4
    + PUNCTUATION * 4
    + SPECIAL_CHARS * 3
    + REPEATED_SPACE * 3
    + WRAPPING_CASES * 5
    + LATIN1_CHARS * 2
    + UNSUPPORTED * 1
)

# ---------------------------------------------------------------------------
# Output generator – mirrors pyfiglet main() exactly
# ---------------------------------------------------------------------------

def render(args: list) -> "str | None":
    """
    Reproduce the byte-for-byte output of:
        python -m pyfiglet <args>

    Parses the same flags that main() parses and applies the same
    post-render transforms in the same order.
    Returns the UTF-8 string that pyfiglet would write to stdout.
    """
    # Parse args manually to match pyfiglet's main() exactly
    font      = "standard"
    width     = 80
    justify   = "auto"
    direction = "auto"
    reverse   = False
    flip      = False
    strip_nl  = False
    norm_nl   = False
    text_parts = []

    i = 0
    while i < len(args):
        a = args[i]
        if a == "-f" and i + 1 < len(args):
            font = args[i + 1]; i += 2
        elif a == "-w" and i + 1 < len(args):
            width = int(args[i + 1]); i += 2
        elif a == "-j" and i + 1 < len(args):
            justify = args[i + 1]; i += 2
        elif a == "-D" and i + 1 < len(args):
            direction = args[i + 1]; i += 2
        elif a == "-r":
            reverse = True; i += 1
        elif a == "-F":
            flip = True; i += 1
        elif a == "-s":
            strip_nl = True; i += 1
        elif a == "-n":
            norm_nl = True; i += 1
        else:
            text_parts.append(a); i += 1

    text = " ".join(text_parts)

    try:
        f = Figlet(font=font, direction=direction, justify=justify, width=width)
    except FontNotFound:
        return ""

    # Guard against pyfiglet infinite loop bug:
    # If a character's width strictly exceeds width, pyfiglet raises CharNotPrinted.
    # But if character's width is EQUAL to width, self.width < self.getCurWidth()
    # is False, yet addCharToProduct wraps when currentTotalWidth >= width.
    # On an empty buffer, this leads to an infinite decrement/increment loop.
    for ch in text:
        c = ord(ch)
        if c in f.Font.width and f.Font.width[c] >= width:
            return None

    try:
        r = f.renderText(text)
    except CharNotPrinted:
        # Width too narrow for at least one character in this font.
        # Signal the caller to retry with a different combination.
        return None

    if reverse:
        r = r.reverse()
    if flip:
        r = r.flip()
    if strip_nl:
        r = r.strip_surrounding_newlines()
    elif norm_nl:
        r = r.normalize_surrounding_newlines()

    # pyfiglet main() writes r.encode('UTF-8') then b'\n'
    # so the full stdout is r + '\n'
    return r + "\n"


# ---------------------------------------------------------------------------
# Test-case builder
# ---------------------------------------------------------------------------

def build_case(rng: random.Random) -> dict:
    """
    Build one randomised test case. Returns {"args": [...], "stdout": "..."}.

    Retries up to MAX_RETRIES times when pyfiglet raises CharNotPrinted
    (width too narrow for a character in the chosen font).  After all
    retries are exhausted the function falls back to a configuration that
    is guaranteed to succeed: font=standard, width=80, plain ASCII text.
    """
    MAX_RETRIES = 20

    for attempt in range(MAX_RETRIES + 1):
        if attempt < MAX_RETRIES:
            # Normal random selection
            if rng.random() < 0.40:
                font = rng.choice(REQUIRED_FONTS)
            else:
                font = rng.choice(ALL_FONTS)

            width   = rng.choice(WIDTHS)
            justify = rng.choice(JUSTIFIES)
            direc   = rng.choice(DIRECTIONS)

            use_r = rng.random() < 0.25
            use_F = rng.random() < 0.25
            use_s = rng.random() < 0.25
            use_n = rng.random() < 0.25

            text = rng.choice(ALL_INPUTS)
        else:
            # Guaranteed-safe fallback: standard font, wide width, short ASCII
            font    = "standard"
            width   = 80
            justify = "left"
            direc   = "auto"
            use_r   = False
            use_F   = False
            use_s   = False
            use_n   = False
            text    = rng.choice(ORDINARY_WORDS)   # plain ASCII, always fits

        args = ["-f", font, "-w", str(width), "-j", justify, "-D", direc]
        if use_r:
            args.append("-r")
        if use_F:
            args.append("-F")
        if use_s:
            args.append("-s")
        if use_n:
            args.append("-n")
        args.append(text)

        stdout = render(args)

        # render() returns None when CharNotPrinted was raised → retry.
        # render() returns "" when font was not found → retry.
        # render() returns a non-empty string → valid case.
        if stdout is not None and stdout.strip():
            return {"args": args, "stdout": stdout}

    # Should never reach here because the fallback always succeeds,
    # but guard defensively.
    raise RuntimeError("build_case: could not generate a valid test case")


# ---------------------------------------------------------------------------
# Main generation loop
# ---------------------------------------------------------------------------

def generate(seed: int, n: int, out_path: str) -> None:
    rng = random.Random(seed)
    os.makedirs(os.path.dirname(out_path), exist_ok=True)
    filename = os.path.basename(out_path)
    print(f"Generating {n} cases for {filename}...")
    written = 0
    with open(out_path, "w", encoding="utf-8") as fh:
        while written < n:
            # build_case() always returns a valid, non-empty case.
            # It retries internally on CharNotPrinted / FontNotFound / empty
            # output, and uses a guaranteed-safe fallback if retries run out.
            case = build_case(rng)
            fh.write(json.dumps(case, ensure_ascii=False) + "\n")
            written += 1
            if written % 500 == 0 or written == n:
                print(f"  [{filename}] {written}/{n} cases generated", flush=True)
    print(f"Wrote {written} cases → {out_path}", flush=True)


if __name__ == "__main__":
    BASE_SEED = 7

    # Visible dataset  – seed 7
    generate(
        seed=BASE_SEED,
        n=2000,
        out_path=os.path.join(SCRIPT_DIR, "visible.jsonl"),
    )

    # Hidden dataset – seed derived from BASE_SEED to keep it deterministic
    # but independent from the visible set.
    generate(
        seed=BASE_SEED + 1_000_003,   # prime offset → no correlation
        n=2000,
        out_path=os.path.join(SCRIPT_DIR, "hidden.jsonl"),
    )

    print("Done.")

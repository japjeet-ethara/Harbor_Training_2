#!/usr/bin/env python3
"""
gen.py – Generate visible.jsonl and hidden.jsonl for the figlet-go Harbor task.

Run from INSIDE the figlet-go/ folder, with the pyfiglet/ folder next to it:
    python tests/gen.py

Requires: Python 3, pyfiglet already set up (fonts-standard copied to fonts/).
"""

import sys
import os
import json
import random
import io

# ── Make sure pyfiglet is importable ─────────────────────────────────────────
# Try multiple candidate locations for the pyfiglet clone
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
TASK_DIR   = os.path.dirname(SCRIPT_DIR)

candidates = [
    os.path.join(TASK_DIR, "pyfiglet"),          # task2-devansh/pyfiglet   (sibling of figlet-go)
    os.path.join(os.path.dirname(TASK_DIR), "pyfiglet"),  # one level up
    os.getcwd(),                                  # running from inside pyfiglet clone
]
for c in candidates:
    if os.path.isdir(os.path.join(c, "pyfiglet")):
        if c not in sys.path:
            sys.path.insert(0, c)
        break

from pyfiglet import Figlet, FigletString   # noqa: E402

# ── Configuration ─────────────────────────────────────────────────────────────
SEED       = 7
N_VISIBLE  = 2000
N_HIDDEN   = 2000

FONTS      = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS     = [10, 20, 40, 60, 80, 120]
JUSTIFIES  = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]

# Normal words
NORMAL_WORDS = [
    "hello", "world", "foo", "bar", "baz", "test", "Harbor",
    "Go", "figlet", "python", "ascii", "art", "agent",
    "A", "Z", "a", "z", "0", "9",
]

# Tricky / rare-case texts (from NOTES.md rare-cases list)
TRICKY_TEXTS = [
    # smushing edge cases
    "||", "||A", "|_|", "/_\\", "[_]", "{_}", "(_)",
    "/\\", "\\/", ">/", "><", "/>",
    "[]", "{}", "()", "<>",
    # special characters
    "|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">",
    # double spaces
    "hello  world", "a  b  c",
    # long text that must wrap
    "the quick brown fox jumps over the lazy dog",
    "superlongwordthatwillwrapinnarrowwidth",
    "Harbor is an AI evaluation framework for testing agents",
    # Latin-1 letters (some fonts have them)
    "cafe", "naive",    # ASCII fallbacks
    # Characters the font might not have (should be silently skipped)
    "\x01", "\x02", "\x7f",
    # Mixed
    "Hi!", "Yes?", "No.", "OK!", "1+1=2", "#1",
    "a|b", "c/d", "e\\f",
]

# Combined pool for text generation
TEXT_POOL = NORMAL_WORDS + TRICKY_TEXTS


def run_pyfiglet(args):
    """
    Run pyfiglet's main() with the given args list and capture stdout exactly.
    Returns the output string, or None on error.
    """
    import pyfiglet as pf_module

    # Parse args the same way the CLI does
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
    if not text:
        return None

    try:
        f = Figlet(font=font, direction=direction, justify=justify, width=width)
        # Avoid pyfiglet infinite loop when a single character does not fit in width
        for ch in text:
            code = ord(ch)
            if code in f.Font.width and f.Font.width[code] >= width:
                return None
    except Exception:
        return None

    try:
        r = f.renderText(text)
    except Exception:
        return None

    if reverse:
        r = r.reverse()
    if flip:
        r = r.flip()
    if strip_nl:
        r = r.strip_surrounding_newlines()
    elif norm_nl:
        r = r.normalize_surrounding_newlines()

    # Encode to bytes and back to get exact output (matches CLI's UTF-8 write + \n)
    raw = r.encode("UTF-8") + b"\n"
    return raw.decode("UTF-8", errors="replace")


def build_case(rng):
    """Build one random test case. Returns {"args": [...], "out": "..."} or None."""
    font      = rng.choice(FONTS)
    width     = rng.choice(WIDTHS)
    justify   = rng.choice(JUSTIFIES)
    direction = rng.choice(DIRECTIONS)
    reverse   = rng.random() < 0.25
    flip      = rng.random() < 0.25
    strip_nl  = rng.random() < 0.25
    # -n is applied only if -s is not set
    norm_nl   = (not strip_nl) and (rng.random() < 0.25)

    text = rng.choice(TEXT_POOL)

    args = ["-f", font, "-w", str(width), "-j", justify, "-D", direction]
    if reverse:
        args.append("-r")
    if flip:
        args.append("-F")
    if strip_nl:
        args.append("-s")
    elif norm_nl:
        args.append("-n")
    args.append(text)

    out = run_pyfiglet(args)
    if out is None:
        return None
    return {"args": args, "out": out}


def generate(path, n, rng, exclude_args=None):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    written = 0
    attempts = 0
    seen = set()
    with open(path, "w", encoding="utf-8") as fh:
        while written < n:
            attempts += 1
            if attempts > n * 20:
                print(f"WARNING: only wrote {written}/{n} cases after {attempts} attempts", file=sys.stderr)
                break
            case = build_case(rng)
            if case is None:
                continue
            key = tuple(case["args"])
            if key in seen:
                continue
            if exclude_args and key in exclude_args:
                continue
            seen.add(key)
            fh.write(json.dumps(case, ensure_ascii=False) + "\n")
            written += 1
            if written % 500 == 0:
                print(f"  {written}/{n} written...", flush=True)
    print(f"Wrote {written} cases to {path}", flush=True)
    return seen


def main():
    rng = random.Random(SEED)

    out_dir = os.path.join(SCRIPT_DIR, "data")

    print("Generating visible.jsonl ...")
    visible_keys = generate(os.path.join(out_dir, "visible.jsonl"), N_VISIBLE, rng)

    print("Generating hidden.jsonl ...")
    generate(os.path.join(out_dir, "hidden.jsonl"), N_HIDDEN, rng, exclude_args=visible_keys)

    print("Done.")


if __name__ == "__main__":
    main()

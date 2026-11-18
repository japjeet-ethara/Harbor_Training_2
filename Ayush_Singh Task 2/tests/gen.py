#!/usr/bin/env python3
"""
gen.py — Generate visible.jsonl and hidden.jsonl for the figlet-go Harbor task.

Run from the figlet-practice/ directory (parent of both pyfiglet/ and figlet-go/):
    python figlet-go/tests/gen.py

Requirements: pyfiglet cloned and fonts set up at ../pyfiglet relative to this script's dir.
"""

import json
import os
import random
import sys

# ---------------------------------------------------------------------------
# Path setup — make sure we import the pinned pyfiglet, not any installed copy
# ---------------------------------------------------------------------------
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
TASK_DIR = os.path.dirname(SCRIPT_DIR)             # figlet-go/
PRACTICE_DIR = os.path.dirname(TASK_DIR)           # figlet-practice/
PYFIGLET_DIR = os.path.join(PRACTICE_DIR, "pyfiglet")

if not os.path.isdir(PYFIGLET_DIR):
    sys.exit(f"ERROR: pyfiglet not found at {PYFIGLET_DIR}\n"
             "Clone it and set up fonts first (see Step 1).")

sys.path.insert(0, PYFIGLET_DIR)

from pyfiglet import FigletFont, Figlet, FigletString, CharNotPrinted  # noqa: E402

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------
FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFIES = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]

# Preload fonts for fast validation and rendering
CACHED_FONTS = {name: FigletFont(name) for name in FONTS}

# Normal words
NORMAL_WORDS = [
    "hello", "world", "test", "go", "python", "figlet", "harbor",
    "foo", "bar", "baz", "cat", "dog", "hi", "ok", "yes", "no",
    "agent", "code", "build", "run", "port",
]

# Tricky inputs that stress smushing and edge cases (from NOTES.md rare-cases list)
TRICKY = [
    # Smushing edge cases
    "|_|", "/>\\", "><", "[]", "{}", "()", "[]{}", "([{}])",
    # Individual rule stressors
    "||", "__", "//", "\\\\", ">><<",
    # Bracket pairs
    "[]{}()",
    # Mixed special
    "/\\|_",
    # Latin-1 / umlauts (pyfiglet specially loads ÄÖÜäöüß after char 127)
    "é", "ß", "ñ", "ä", "ö", "ü", "Ä", "Ö", "Ü",
    # Double spaces
    "hello  world", "a  b  c",
    # Long sentences that must wrap at narrow widths
    "the quick brown fox jumps over the lazy dog",
    "pack my box with five dozen liquor jugs",
    "how razorback jumping frogs can level six piqued gymnasts",
    # Characters not in most fonts (silently skipped)
    "→", "★", "€",
    # Hardblank collision candidates
    "i i", "l l", "! !",
]

LONG_TEXTS = [
    "the quick brown fox",
    "hello beautiful world",
    "agent builds a perfect twin",
    "smush all the chars together now",
]


def random_text(rng: random.Random) -> str:
    choice = rng.random()
    if choice < 0.30:
        return rng.choice(TRICKY)
    elif choice < 0.55:
        return rng.choice(NORMAL_WORDS)
    elif choice < 0.70:
        return rng.choice(NORMAL_WORDS) + " " + rng.choice(NORMAL_WORDS)
    elif choice < 0.80:
        return rng.choice(LONG_TEXTS)
    else:
        specials = list("|/\\_ []{}()<>")
        chars = [rng.choice(specials) for _ in range(rng.randint(1, 4))]
        return "".join(chars) + rng.choice(NORMAL_WORDS)


def build_args(rng: random.Random) -> list:
    """Build a pyfiglet-compatible argument list."""
    args = []

    font = rng.choice(FONTS)
    args += ["-f", font]

    width = rng.choice(WIDTHS)
    args += ["-w", str(width)]

    justify = rng.choice(JUSTIFIES)
    args += ["-j", justify]

    direction = rng.choice(DIRECTIONS)
    args += ["-D", direction]

    if rng.random() < 0.25:
        args.append("-r")
    if rng.random() < 0.25:
        args.append("-F")
    if rng.random() < 0.25:
        args.append("-s")
    if rng.random() < 0.25:
        args.append("-n")

    text = random_text(rng)
    args.append(text)

    return args


def render(args: list) -> str:
    """
    Reproduce exactly what pyfiglet's main() does for the given arg list.
    Returns the output string (UTF-8 text, ending with \\n).
    """
    font = "standard"
    width = 80
    justify = "auto"
    direction = "auto"
    do_reverse = False
    do_flip = False
    do_strip = False
    do_norm = False
    text_parts = []

    i = 0
    while i < len(args):
        a = args[i]
        if a in ("-f", "--font"):
            i += 1; font = args[i]
        elif a in ("-w", "--width"):
            i += 1; width = int(args[i])
        elif a in ("-j", "--justify"):
            i += 1; justify = args[i]
        elif a in ("-D", "--direction"):
            i += 1; direction = args[i]
        elif a in ("-r", "--reverse"):
            do_reverse = True
        elif a in ("-F", "--flip"):
            do_flip = True
        elif a in ("-s", "--strip-surrounding-newlines"):
            do_strip = True
        elif a in ("-n", "--normalize-surrounding-newlines"):
            do_norm = True
        else:
            text_parts.append(a)
        i += 1

    text = " ".join(text_parts)

    try:
        f = Figlet(font=font, direction=direction, justify=justify, width=width)
        # Use cached font to avoid re-reading disk
        f.Font = CACHED_FONTS[font]
        r = f.renderText(text)
    except Exception:
        return ""

    if do_reverse:
        r = r.reverse()
    if do_flip:
        r = r.flip()
    if do_strip:
        r = r.strip_surrounding_newlines()
    elif do_norm:
        r = r.normalize_surrounding_newlines()

    return r + "\n"


def generate(n: int, rng: random.Random) -> list:
    """Generate n cases using the provided seeded RNG."""
    cases = []
    while len(cases) < n:
        args = build_args(rng)
        # Safety check: if any character's width in the selected font >= terminal width,
        # pyfiglet either raises CharNotPrinted or loops infinitely in handleNewLine.
        font_name = args[1]
        width = int(args[3])
        text = args[-1]
        ff = CACHED_FONTS[font_name]
        if any(ff.width.get(ord(c), 0) >= width for c in text):
            continue

        out = render(args)
        if out:
            cases.append({"args": args, "out": out})
    return cases


def write_jsonl(path: str, cases: list) -> None:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        for case in cases:
            f.write(json.dumps(case, ensure_ascii=False) + "\n")
    print(f"Wrote {len(cases)} cases to {path}")


def main():
    # Use random.seed(7) as strictly specified in the instructions
    rng = random.Random(7)

    out_dir = os.path.join(SCRIPT_DIR, "data")

    # Generate 2000 visible cases and then 2000 hidden cases from the same seeded stream
    print("Generating 2000 visible cases...")
    visible = generate(2000, rng)
    write_jsonl(os.path.join(out_dir, "visible.jsonl"), visible)

    print("Generating 2000 hidden cases...")
    hidden = generate(2000, rng)
    write_jsonl(os.path.join(out_dir, "hidden.jsonl"), hidden)

    print("Done generating test cases.")


if __name__ == "__main__":
    main()

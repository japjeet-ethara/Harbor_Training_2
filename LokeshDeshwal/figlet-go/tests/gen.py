#!/usr/bin/env python3
"""
Generate test cases for figlet-go task.
Runs locally (not in Docker) using the pyfiglet repo next to the task folder.
"""

import io
import json
import os
import random
import sys
from pathlib import Path

# Locate pyfiglet repository next to figlet-go
CURRENT_DIR = Path(__file__).resolve().parent
TASK_DIR = CURRENT_DIR.parent
PYFIGLET_DIR = TASK_DIR.parent / "pyfiglet"

if not PYFIGLET_DIR.is_dir():
    raise RuntimeError(f"pyfiglet directory not found at {PYFIGLET_DIR}")

sys.path.insert(0, str(PYFIGLET_DIR))
import pyfiglet

FONTS = [
    "standard",
    "slant",
    "small",
    "big",
    "mini",
    "banner",
    "shadow",
    "script",
]

# Preload fonts for fast lookup and validation
FONT_OBJS = {f: pyfiglet.FigletFont(f) for f in FONTS}

WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

NORMAL_TEXTS = [
    "hello",
    "world",
    "pyfiglet",
    "Harbor",
    "Go",
    "Python",
    "test",
    "banner",
    "42",
    "OK",
    "rule 42",
    "12345",
    "Code & Data",
    "Smushing works!",
    "FIGlet twin",
    "quick check",
    "terminal art",
    "zero one",
    "abc XYZ",
    "hello world",
]

TRICKY_TEXTS = [
    # Equal character (Rule 1)
    "XX",
    "##",
    "++",
    "**",
    "aa",
    # Underscore / lowline (Rule 2)
    "_/",
    "}_",
    "_|",
    "|_",
    "_\\",
    "\\_",
    "_[",
    "]_",
    "_<_",
    # Hierarchy (Rule 3)
    "|(",
    ">]",
    "|/",
    "|\\",
    "[/]",
    "}(",
    "<{",
    # Opposite pairs (Rule 4)
    "[]",
    "}{",
    "()",
    "][",
    "{}",
    ")(",
    "[[]]",
    "{{}}",
    "(())",
    # Big X (Rule 5)
    "/\\",
    "\\/",
    "><",
    "/\\/\\",
    "\\/\\/",
    # Hardblank (Rule 6) & tricky symbols
    "$$",
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
    "| / \\ _ [ ] { } ( ) < >",
    "|_|",
    "\\_/",
    "/_\\",
    "[|]",
    "{|}",
    "(|)",
    # Double spaces & spacing
    "hello  world",
    "foo   bar",
    "  spaced  out  ",
    "a  b  c",
    "one   two    three",
    # Long sentences that must wrap
    "The quick brown fox jumps over the lazy dog.",
    "FIGlet is a computer program that generates text banners, in which letters are synthesized from smaller ASCII characters.",
    "Pack my box with five dozen liquor jugs.",
    "To be, or not to be, that is the question: whether tis nobler in the mind to suffer the slings and arrows.",
    # Latin-1 letters
    "é ß ñ",
    "café",
    "Straße",
    "año",
    "Ä Ö Ü ä ö ü",
    "Grüße",
    "München",
    # Characters font might not have
    "hello ☺ world",
    "test 🚀 launch",
    "price € 100",
    "漢字",
    "中文",
    "crab 🦀",
]

ALL_TEXTS = NORMAL_TEXTS + TRICKY_TEXTS


def is_valid_combination(font: str, width: int, text: str) -> bool:
    """
    Ensure width > any character width in the text for the given font.
    If width <= char_width, pyfiglet either raises CharNotPrinted
    or hangs in an infinite loop due to upstream bug in handleNewLine.
    """
    ff = FONT_OBJS[font]
    for ch in text:
        code = ord(ch)
        if code in ff.chars and ff.width.get(code, 0) >= width:
            return False
    return True


def run_pyfiglet(args: list[str]) -> str:
    """Run pyfiglet.main() with args and capture stdout exactly."""
    old_stdout = sys.stdout
    old_argv = sys.argv
    buf = io.BytesIO()
    wrapper = io.TextIOWrapper(buf, encoding="utf-8")
    sys.stdout = wrapper
    sys.argv = ["pyfiglet"] + args
    try:
        pyfiglet.main()
    finally:
        sys.stdout = old_stdout
        sys.argv = old_argv
    return buf.getvalue().decode("utf-8", errors="replace")


def generate_case() -> tuple[list[str], str]:
    """Generate a single random test case and return (args, out)."""
    while True:
        font = random.choice(FONTS)
        width = random.choice(WIDTHS)
        justify = random.choice(JUSTIFY)
        direction = random.choice(DIRECTION)
        text = random.choice(ALL_TEXTS)

        if not is_valid_combination(font, width, text):
            continue

        args = [
            "-f",
            font,
            "-w",
            str(width),
            "-j",
            justify,
            "-D",
            direction,
        ]

        if random.random() < 0.25:
            args.append("-r")
        if random.random() < 0.25:
            args.append("-F")
        if random.random() < 0.25:
            args.append("-s")
        if random.random() < 0.25:
            args.append("-n")

        args.append(text)

        try:
            out = run_pyfiglet(args)
            return args, out
        except Exception:
            continue


def build_dataset(filepath: Path, count: int, exclude_keys: set = None) -> set:
    filepath.parent.mkdir(parents=True, exist_ok=True)
    seen_keys = set()
    with filepath.open("w", encoding="utf-8") as f:
        i = 0
        while i < count:
            args, out = generate_case()
            key = json.dumps(args)
            if key in seen_keys:
                continue
            if exclude_keys is not None and key in exclude_keys:
                continue

            seen_keys.add(key)
            entry = {"args": args, "out": out}
            f.write(json.dumps(entry, ensure_ascii=False) + "\n")
            i += 1
            if i % 500 == 0:
                print(f"  {filepath.name}: generated {i}/{count} cases", flush=True)
    return seen_keys


def main():
    random.seed(7)
    data_dir = CURRENT_DIR / "data"
    visible_file = data_dir / "visible.jsonl"
    hidden_file = data_dir / "hidden.jsonl"

    print("Generating visible cases (2000)...", flush=True)
    visible_keys = build_dataset(visible_file, 2000)

    print("Generating hidden cases (a strictly different 2000)...", flush=True)
    build_dataset(hidden_file, 2000, exclude_keys=visible_keys)

    print("Done generating test cases.", flush=True)


if __name__ == "__main__":
    main()

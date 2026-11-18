#!/usr/bin/env python3
"""Generate deterministic pyfiglet behavior fixtures on the task author's host."""

import argparse
import io
import json
import random
import sys
from pathlib import Path

FONTS = ("standard", "slant", "small", "big", "mini", "banner", "shadow", "script")
WIDTHS = (10, 20, 40, 60, 80, 120)
JUSTIFICATIONS = ("auto", "left", "center", "right")
DIRECTIONS = ("auto", "left-to-right", "right-to-left")
TEXTS = (
    "hello",
    "FIGlet is a serious way to shout in a chat",
    "smush rules",
    "vertical flip",
    "right to left",
    "two  spaces",
    "a long sentence that must wrap across several lines at narrow widths",
    "||",
    "|_|",
    "/\\",
    "\\/",
    "_/|[]{}()<>",
    "[ ] { } ( ) < >",
    "[] {} ()",
    "$$ hardblanks",
    "é ß ñ",
    "café naïve façade",
    "snowman ☃ missing glyph",
    "A\nB",
    "leading and trailing spaces  ",
    "X" * 50,
    "12345 !?.,",
    "wide Ω characters",
)
FONT_CACHE = {}


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--pyfiglet",
        type=Path,
        default=Path(__file__).resolve().parents[4] / "pyfiglet",
        help="path to the pinned pyfiglet checkout",
    )
    parser.add_argument("--count", type=int, default=2000)
    return parser.parse_args()


def make_case(rng, text):
    args = [
        "-f",
        rng.choice(FONTS),
        "-w",
        str(rng.choice(WIDTHS)),
        "-j",
        rng.choice(JUSTIFICATIONS),
        "-D",
        rng.choice(DIRECTIONS),
    ]
    for flag in ("-r", "-F", "-s", "-n"):
        if rng.randrange(4) == 0:
            args.append(flag)
    args.append(text)
    return args


def oracle(checkout, args):
    if str(checkout) not in sys.path:
        sys.path.insert(0, str(checkout))
    import pyfiglet

    font_name = args[args.index("-f") + 1]
    width = int(args[args.index("-w") + 1])
    face = FONT_CACHE.get(font_name)
    if face is None:
        face = pyfiglet.FigletFont(font_name)
        FONT_CACHE[font_name] = face
    if width <= 0 or any(face.width.get(ord(char), 0) >= width for char in args[-1]):
        return None

    original_argv = sys.argv
    original_stdout = sys.stdout
    captured = io.BytesIO()
    text_stdout = io.TextIOWrapper(captured, encoding="utf-8")
    sys.argv = ["pyfiglet", *args]
    sys.stdout = text_stdout
    try:
        try:
            status = pyfiglet.main()
        except pyfiglet.CharNotPrinted:
            return None
        if status:
            raise RuntimeError(f"pyfiglet exited with status {status} for {args!r}")
        return captured.getvalue().decode("utf-8")
    finally:
        sys.argv = original_argv
        sys.stdout = original_stdout


def write_cases(path, cases):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as output:
        for record in cases:
            output.write(json.dumps(record, ensure_ascii=False) + "\n")


def main():
    options = parse_args()
    checkout = options.pyfiglet.resolve()
    if not (checkout / "pyfiglet" / "__main__.py").is_file():
        raise SystemExit(f"not a pyfiglet checkout: {checkout}")
    fonts = checkout / "pyfiglet" / "fonts"
    if not fonts.is_dir():
        raise SystemExit(
            "pyfiglet/fonts is missing; copy fonts-standard there before generating"
        )

    random.seed(7)
    rng = random
    task = Path(__file__).resolve().parents[1]
    visible = []
    hidden = []
    seen = set()
    while len(visible) < options.count or len(hidden) < options.count:
        args = make_case(rng, rng.choice(TEXTS))
        key = tuple(args)
        if key in seen:
            continue
        seen.add(key)
        expected = oracle(checkout, args)
        if expected is None:
            continue
        record = {"args": args, "out": expected}
        if len(visible) < options.count:
            visible.append(record)
        else:
            hidden.append(record)

    write_cases(task / "tests" / "data" / "visible.jsonl", visible)
    write_cases(task / "tests" / "data" / "hidden.jsonl", hidden)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Generate deterministic pyfiglet reference cases for Task-2."""

import json
import io
import random
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
REFERENCE = ROOT.parent / "pyfiglet"
FONT_NAMES = ("standard", "slant", "small", "big", "mini", "banner", "shadow", "script")
WIDTHS = (10, 20, 40, 60, 80, 120)
JUSTIFICATIONS = ("auto", "left", "center", "right")
DIRECTIONS = ("auto", "left-to-right", "right-to-left")
FLAGS = ("-r", "-F", "-s", "-n")
TEXTS = (
    "hello",
    "Harbor FIGlet",
    "portable Go",
    "wrap these words around",
    "the quick brown fox jumps over the lazy dog",
    "|",
    "/",
    "\\",
    "_",
    "[ ] { } ( )",
    "< >",
    "|_|",
    "[]{}()",
    "<><>",
    "/\\",
    "___",
    "__/",
    "~",
    "two  spaces",
    "é ß ñ",
    "🙂 ☃",
    "A long sentence with many ordinary words that must wrap at the selected narrow width.",
    "left-to-right and right-to-left",
)
NARROW_TEXTS = (
    "|",
    "/",
    "\\",
    "_",
    "~",
    "i",
    "1",
    ".",
    "x",
    "b",
    "c",
    "n",
    "(",
    ")",
    "[",
    "]",
    "{",
    "}",
    "<",
    ">",
    "A",
    "e",
    "o",
)


def run_original(args, pyfiglet_main):
    original_stdout = sys.stdout
    original_argv = sys.argv
    captured = io.BytesIO()

    class Capture:
        def detach(self):
            return captured

    sys.stdout = Capture()
    sys.argv = ["pyfiglet", *args]
    try:
        status = pyfiglet_main()
    finally:
        sys.stdout = original_stdout
        sys.argv = original_argv
    if status != 0:
        raise RuntimeError(f"pyfiglet exited with status {status} for args {args!r}")
    return captured.getvalue()


def build_flag_cases():
    rng = random.Random(7)
    selected = {flag: set() for flag in FLAGS}
    for start in (0, 2000):
        indexes = range(start, start + 2000)
        for flag in FLAGS:
            selected[flag].update(rng.sample(indexes, 500))
    return selected


def build_args(index, font_cache, flag_cases):
    font_name = FONT_NAMES[index % len(FONT_NAMES)]
    width = WIDTHS[(index // len(FONT_NAMES)) % len(WIDTHS)]
    if font_name not in font_cache:
        sys.path.insert(0, str(REFERENCE))
        from pyfiglet import FigletFont

        font_cache[font_name] = FigletFont(font_name)
    font = font_cache[font_name]
    if width == 10:
        unknown_codepoint = 0xF0000 + index
        while unknown_codepoint in font.chars:
            unknown_codepoint += 1
        unique_suffix = chr(unknown_codepoint)
    else:
        unique_suffix = f" i{index:04d}"
    candidates = NARROW_TEXTS if width == 10 else TEXTS
    safe_texts = [
        text
        for text in candidates
        if all(
            font.width.get(ord(character), 0) < width
            for character in text + unique_suffix
        )
    ]
    args = [
        "-f",
        font_name,
        "-w",
        str(width),
        "-j",
        JUSTIFICATIONS[(index // 3) % len(JUSTIFICATIONS)],
        "-D",
        DIRECTIONS[(index // 2) % len(DIRECTIONS)],
    ]
    for option in FLAGS:
        if index in flag_cases[option]:
            args.append(option)
    args.append(random.choice(safe_texts or ("i",)) + unique_suffix)
    return args


def cache_loaded_fonts(font_cache):
    from pyfiglet import Figlet

    def set_font(self, **kwargs):
        if "font" in kwargs:
            self.font = kwargs["font"]
        self.Font = font_cache[self.font]

    Figlet.setFont = set_font


def main():
    random.seed(7)
    if not (REFERENCE / "pyfiglet" / "__main__.py").is_file():
        raise SystemExit(
            "Expected the pyfiglet checkout beside Task-2 at "
            f"{REFERENCE}; clone the pinned repository and set up its standard fonts first."
        )
    if not list((REFERENCE / "pyfiglet" / "fonts").glob("*.flf")):
        raise SystemExit(
            "No standard fonts found. Copy pyfiglet/fonts-standard/* into pyfiglet/fonts/ first."
        )
    sys.path.insert(0, str(REFERENCE))
    from pyfiglet import main as pyfiglet_main

    seen = set()
    cases = []
    font_cache = {}
    flag_cases = build_flag_cases()
    cache_loaded_fonts(font_cache)
    while len(cases) < 4000:
        args = build_args(len(cases), font_cache, flag_cases)
        key = json.dumps(args, ensure_ascii=False, separators=(",", ":"))
        if key in seen:
            continue
        output = run_original(args, pyfiglet_main)
        seen.add(key)
        cases.append({"args": args, "out": output.decode("utf-8")})

    data_dir = ROOT / "tests" / "data"
    data_dir.mkdir(parents=True, exist_ok=True)
    for name, subset in (
        ("visible.jsonl", cases[:2000]),
        ("hidden.jsonl", cases[2000:]),
    ):
        with (data_dir / name).open("w", encoding="utf-8", newline="\n") as output:
            for case in subset:
                output.write(json.dumps(case, ensure_ascii=False) + "\n")
    print("Wrote 2000 visible and 2000 hidden cases.")


if __name__ == "__main__":
    main()

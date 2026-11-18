#!/usr/bin/env python3
"""
Generate visible and hidden FIGlet cases by running the original pyfiglet.

Runs on the host (not in Docker) with the pyfiglet folder from Step 1 next to
the task folder (../.. / "pyfiglet" relative to this file, i.e. sibling of the
figlet-go task folder).

Writes:
  tests/data/visible.jsonl  (2000 cases)
  tests/data/hidden.jsonl   (2000 different cases)

Each line: {"args": [...], "out": "..."}
"out" is captured exactly from original pyfiglet stdout (never handwritten).
"""

from __future__ import annotations

import io
import json
import os
import random
import shutil
import sys
from pathlib import Path


SEED = 7
COUNT_PER_SET = 2000

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFIES = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]

# Texts: mix normal words with tricky ones (edge cases, smushing chars, spacing,
# wrapping, Latin-1, and characters not present in these fonts).
TEXTS = [
    # empty / whitespace edge cases
    "",
    " ",
    "  ",
    "\t",
    "\n",
    " \n ",
    "hello ",
    "  leading",
    "trailing  ",
    # normal words
    "hello",
    "Hello",
    "HELLO",
    "World",
    "hello world",
    "FIGlet",
    "PyFiglet",
    "Go",
    "Test",
    "The Quick Brown Fox",
    "lorem ipsum dolor sit amet",
    "Pack my box with five dozen liquor jugs",
    "Sphinx of black quartz, judge my vow",
    # digits / punctuation / mixed
    "1234567890",
    "Hello, World!",
    "v1.2.3",
    "C:\\path\\to\\file",
    "user@example.com",
    "100% ready?",
    "A&B",
    "rock'n'roll",
    "end.",
    # tricky/smushing characters
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
    "|/\\_[]{}()<>",
    "|_|",
    "/\\",
    "\\/",
    "><",
    "<>",
    "[]",
    "{}",
    "()",
    "_/\\_",
    "|/|\\|",
    "<o_o>",
    # double spaces
    "a  b",
    "foo  bar  baz",
    "  a  b  ",
    # long sentences that must wrap
    "This is a fairly long sentence that should wrap when rendered at narrow widths.",
    "Supercalifragilisticexpialidocious is a very long word for wrapping behavior.",
    "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen",
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789",
    # newlines inside text
    "hello\nworld",
    "line1\nline2\nline3",
    "a\n\nb",
    # Latin-1 letters
    "é",
    "ß",
    "ñ",
    "café",
    "niño",
    "Straße",
    "àèìòù",
    "ÄÖÜäöü",
    "Æ Ø Å",
    # characters the font does not have
    "☺",
    "★",
    "你",
    "こんにちは",
    "Привет",
    "مرحبا",
    "€£¥",
    "\u2603\u2764\u2714",
]


def locate_pyfiglet_repo() -> Path:
    # Task folder is .../figlet-go, pyfiglet folder is next to it (sibling).
    here = Path(__file__).resolve()
    task_dir = here.parents[1]  # .../figlet-go
    candidate = task_dir.parent / "pyfiglet"
    if not (candidate / "pyfiglet" / "__init__.py").is_file():
        raise SystemExit(f"pyfiglet source not found at: {candidate}")
    return candidate


def ensure_fonts(pkg_dir: Path) -> Path | None:
    """Ensure pyfiglet/fonts exists; return path to remove if we created it."""
    fonts_dir = pkg_dir / "fonts"
    if fonts_dir.is_dir():
        return None
    standard_dir = pkg_dir / "fonts-standard"
    if not standard_dir.is_dir():
        raise SystemExit(f"fonts-standard not found at: {standard_dir}")
    fonts_dir.mkdir()
    for src in standard_dir.iterdir():
        if src.is_file():
            shutil.copy2(src, fonts_dir / src.name)
    return fonts_dir


def run_pyfiglet(args: list[str]) -> str | None:
    """Run original pyfiglet main() with args and capture stdout exactly.

    Returns None if pyfiglet fails (e.g. CharNotPrinted) or produces no stdout;
    such cases are skipped rather than written.
    """
    import pyfiglet as _pyfiglet  # imported after sys.path/fonts setup

    old_argv = sys.argv
    old_stdout = sys.stdout
    old_stderr = sys.stderr
    buf = io.BytesIO()
    # main() (py3) does sys.stdout = sys.stdout.detach() and writes bytes.
    # Give it a TextIOWrapper over our BytesIO so detach() yields our buffer.
    sys.argv = ["pyfiglet"] + list(args)
    sys.stdout = io.TextIOWrapper(buf, encoding="utf-8", write_through=True)
    sys.stderr = io.StringIO()
    try:
        try:
            _pyfiglet.main()
        except SystemExit:
            pass
        except Exception:
            return None
        # Flush the text wrapper if main did not detach it.
        try:
            if hasattr(sys.stdout, "flush"):
                sys.stdout.flush()
        except Exception:
            pass
        # If main detached, sys.stdout is the BytesIO; flush it too.
        try:
            sys.stdout.flush()
        except Exception:
            pass
    finally:
        sys.argv = old_argv
        sys.stdout = old_stdout
        sys.stderr = old_stderr
    data = buf.getvalue()
    if not data:
        return None
    return data.decode("utf-8", errors="surrogateescape")


def make_args(rng: random.Random) -> list[str]:
    """Pick one valid case. Avoid known pyfiglet infinite-loop combinations.

    pyfiglet 1.0.4 can loop forever in FigletBuilder when the requested width
    is too small for the glyphs, but only for a few configurations: the slant
    font at width 10, and width 10 with right-to-left direction. Widths of 20
    and above render all of our texts in every font, so width 10 is limited to
    short, left-to-right (or auto) cases.
    """
    while True:
        font = rng.choice(FONTS)
        width = rng.choice(WIDTHS)
        direction = rng.choice(DIRECTIONS)
        text = rng.choice(TEXTS)
        if width == 10:
            if font == "slant":
                continue
            if direction == "right-to-left":
                continue
            if len(text) > 8:
                continue
        break

    args: list[str] = []
    args += ["-f", font]
    args += ["-w", str(width)]
    args += ["-j", rng.choice(JUSTIFIES)]
    args += ["-D", direction]
    # Each on in 1 of 4 cases.
    if rng.randrange(4) == 0:
        args.append("-r")
    if rng.randrange(4) == 0:
        args.append("-F")
    if rng.randrange(4) == 0:
        args.append("-s")
    if rng.randrange(4) == 0:
        args.append("-n")
    args.append(text)
    return args


def args_key(args: list[str]) -> tuple[str, ...]:
    return tuple(args)


def generate_cases(rng: random.Random, count: int, seen: set[tuple[str, ...]]) -> list[dict]:
    cases: list[dict] = []
    while len(cases) < count:
        args = make_args(rng)
        key = args_key(args)
        if key in seen:
            continue
        out = run_pyfiglet(args)
        if out is None:
            # pyfiglet failed for this input (e.g. CharNotPrinted); skip it.
            continue
        seen.add(key)
        cases.append({"args": args, "out": out})
    return cases


def write_jsonl(path: Path, cases: list[dict]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as f:
        for case in cases:
            f.write(json.dumps(case, ensure_ascii=False, separators=(",", ":")) + "\n")


def main() -> int:
    random.seed(SEED)
    sys.dont_write_bytecode = True
    pyfiglet_repo = locate_pyfiglet_repo()
    pkg_dir = pyfiglet_repo / "pyfiglet"

    # Ensure we import the original source from Step 1.
    sys.path.insert(0, str(pyfiglet_repo))
    os.environ["PYTHONDONTWRITEBYTECODE"] = "1"

    created_fonts = ensure_fonts(pkg_dir)
    try:
        # Import after fonts are in place so pyfiglet.fonts is resolvable.
        import pyfiglet  # noqa: F401

        seen: set[tuple[str, ...]] = set()
        visible = generate_cases(random, COUNT_PER_SET, seen)
        hidden = generate_cases(random, COUNT_PER_SET, seen)

        out_dir = Path(__file__).resolve().parent / "data"
        write_jsonl(out_dir / "visible.jsonl", visible)
        write_jsonl(out_dir / "hidden.jsonl", hidden)
        print(f"Wrote {len(visible)} visible and {len(hidden)} hidden cases to {out_dir}")
        return 0
    finally:
        # Clean up fonts dir only if this script created it.
        if created_fonts is not None and created_fonts.is_dir():
            shutil.rmtree(created_fonts)
        # Remove imported module so reruns in same process pick up changes if any.
        for mod in [m for m in list(sys.modules) if m == "pyfiglet" or m.startswith("pyfiglet.")]:
            sys.modules.pop(mod, None)


if __name__ == "__main__":
    raise SystemExit(main())

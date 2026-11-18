import io
import json
import random
import sys
from pathlib import Path


random.seed(7)

TASK_DIR = Path(__file__).resolve().parents[1]
PYFIGLET_DIR = next(
    path
    for path in (TASK_DIR / "pyfiglet", TASK_DIR.parent / "pyfiglet")
    if (path / "pyfiglet" / "__init__.py").is_file()
)
sys.path.insert(0, str(PYFIGLET_DIR))

import pyfiglet


pyfiglet.FONT_DIRECTORIES = (
    str(PYFIGLET_DIR / "pyfiglet" / "fonts-standard"),
    str(PYFIGLET_DIR / "pyfiglet" / "fonts-contrib"),
)

FONTS = ("standard", "slant", "small", "big", "mini", "banner", "shadow", "script")
WIDTHS = (10, 20, 40, 60, 80, 120)
JUSTIFICATIONS = ("auto", "left", "center", "right")
DIRECTIONS = ("auto", "left-to-right", "right-to-left")
FONT_CHAR_WIDTHS = {
    font: pyfiglet.FigletFont(font).width for font in FONTS
}
NORMAL_TEXTS = (
    "hello world",
    "the quick brown fox jumps over the lazy dog",
    "figlet fonts make text look different",
    "Go uses a small standard library",
    "write clear programs and keep useful notes",
)
RARE_CASES = (
    "A A",
    "_ |",
    "| /",
    "[ ]",
    "/ \\",
    "\\ /",
    "> <",
    "< >",
    "$ $",
    "# @",
)
TEXT_FRAGMENTS = (
    "| / \\ _ [ ] { } ( ) < >",
    "two  spaces stay together",
    "é ß ñ",
    "characters absent from a font: ☃ Ω 漢 🙂",
    (
        "A long sentence with several ordinary words should wrap cleanly at "
        "narrow widths while preserving all of its spaces and punctuation."
    ),
)


class _BinaryStdout:
    def __init__(self):
        self.buffer = io.BytesIO()

    def detach(self):
        return self.buffer


def render(args):
    previous_argv = sys.argv
    previous_stdout = sys.stdout
    capture = _BinaryStdout()
    try:
        sys.argv = ["pyfiglet", *args]
        sys.stdout = capture
        pyfiglet.main()
        return capture.buffer.getvalue().decode("utf-8")
    finally:
        sys.argv = previous_argv
        sys.stdout = previous_stdout


def make_case():
    while True:
        font = random.choice(FONTS)
        width = random.choice(WIDTHS)
        justify = random.choice(JUSTIFICATIONS)
        direction = random.choice(DIRECTIONS)
        text = " ".join(
            (
                random.choice(NORMAL_TEXTS),
                random.choice(RARE_CASES),
                random.choice(TEXT_FRAGMENTS),
            )
        )
        # pyfiglet's wrap-at-width logic cannot render a glyph exactly that wide.
        if not any(
            width <= FONT_CHAR_WIDTHS[font][ord(char)]
            for char in text
            if ord(char) in FONT_CHAR_WIDTHS[font]
        ):
            break
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
    for flag in ("-r", "-F", "-s", "-n"):
        if random.randrange(4) == 0:
            args.append(flag)
    args.append(text)
    return {"args": args, "out": render(args)}


def write_cases(path, count, seen):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as output:
        written = 0
        while written < count:
            case = make_case()
            signature = tuple(case["args"])
            if signature in seen:
                continue
            seen.add(signature)
            output.write(json.dumps(case, ensure_ascii=False) + "\n")
            written += 1


def main():
    seen = set()
    data_dir = TASK_DIR / "tests" / "data"
    write_cases(data_dir / "visible.jsonl", 2000, seen)
    write_cases(data_dir / "hidden.jsonl", 2000, seen)


if __name__ == "__main__":
    main()

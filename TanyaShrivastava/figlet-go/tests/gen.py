import io
import json
import random
import sys
from pathlib import Path


random.seed(7)

TASK_ROOT = Path(__file__).resolve().parents[1]
PYFIGLET_ROOT = TASK_ROOT.parent / "pyfiglet"
if not (PYFIGLET_ROOT / "pyfiglet" / "__init__.py").is_file():
    raise FileNotFoundError(f"pyfiglet source not found at {PYFIGLET_ROOT}")

sys.path.insert(0, str(PYFIGLET_ROOT))
from pyfiglet import CharNotPrinted, FigletFont, main as pyfiglet_main


FONTS = ("standard", "slant", "small", "big", "mini", "banner", "shadow", "script")
WIDTHS = (10, 20, 40, 60, 80, 120)
JUSTIFICATIONS = ("auto", "left", "center", "right")
DIRECTIONS = ("auto", "left-to-right", "right-to-left")
FLAGS = ("-r", "-F", "-s", "-n")
NORMAL_TEXTS = (
    "hello",
    "world",
    "pyfiglet",
    "Go compatibility",
    "Harbor task",
    "Good morning",
    "The quick brown fox",
    "simple words",
)
SHORT_NORMAL_TEXTS = ("A", "I", "Go", "Hi", "OK")
SPECIAL_CHARACTERS = r"| / \ _ [ ] { } ( ) < >"
SHORT_TRICKY_TEXTS = (
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
    "é",
    "ß",
    "ñ",
    "😀",
    "漢",
    "  ",
)
LONG_SENTENCES = (
    "A deliberately long compatibility sentence exercises automatic wrapping "
    "at narrow widths while preserving words punctuation and spacing across "
    "several rendered rows for comparison.",
    "The renderer should wrap this extended sentence naturally when the "
    "selected width is small and keep its character layout consistent from "
    "one output line to the next throughout the complete phrase.",
)
FONT_WIDTHS = {font: FigletFont(font).width for font in FONTS}
TRICKY_TEXTS = (
    SPECIAL_CHARACTERS,
    f"symbols {SPECIAL_CHARACTERS} together",
    "two  spaces stay doubled",
    "Latin-1 letters: é ß ñ, café jalapeño",
    "characters absent from fonts: 😀 漢",
    *LONG_SENTENCES,
)


def random_text():
    if random.randrange(4) == 0:
        return random.choice(NORMAL_TEXTS)
    if random.randrange(3) == 0:
        return random.choice(LONG_SENTENCES)
    if random.randrange(2) == 0:
        return f"{random.choice(NORMAL_TEXTS)} {random.choice(TRICKY_TEXTS)}"
    return random.choice(TRICKY_TEXTS)


def short_text(font, width):
    widths = FONT_WIDTHS[font]

    def fits(text):
        return sum(widths.get(ord(char), 0) for char in text) < width

    normal_texts = [text for text in SHORT_NORMAL_TEXTS if fits(text)]
    tricky_texts = [text for text in SHORT_TRICKY_TEXTS if fits(text)]
    choices = random.choice((normal_texts, tricky_texts))
    return random.choice(choices or normal_texts or tricky_texts)


def render_with_original_main(args):
    output = io.BytesIO()
    text_stdout = io.TextIOWrapper(output, encoding="utf-8")
    previous_stdout = sys.stdout
    previous_argv = sys.argv
    try:
        sys.stdout = text_stdout
        sys.argv = ["pyfiglet", *args]
        status = pyfiglet_main()
        if status not in (None, 0):
            raise RuntimeError(f"pyfiglet main returned {status} for {args!r}")
        return output.getvalue().decode("utf-8")
    finally:
        sys.stdout = previous_stdout
        sys.argv = previous_argv


def make_case(seen_args):
    while True:
        font = random.choice(FONTS)
        width = random.choice(WIDTHS)
        text = random_text()
        if width < 40:
            text = short_text(font, width)
        args = [
            "-f",
            font,
            "-w",
            str(width),
            "-j",
            random.choice(JUSTIFICATIONS),
            "-D",
            random.choice(DIRECTIONS),
        ]
        args.extend(flag for flag in FLAGS if random.randrange(4) == 0)
        args.append(text)

        key = tuple(args)
        if key in seen_args:
            continue
        try:
            output = render_with_original_main(args)
        except CharNotPrinted:
            continue
        seen_args.add(key)
        return {"args": args, "out": output}


def write_cases(path, count, seen_args):
    with path.open("w", encoding="utf-8", newline="\n") as data_file:
        for _ in range(count):
            case = make_case(seen_args)
            data_file.write(json.dumps(case, ensure_ascii=False, separators=(",", ":")))
            data_file.write("\n")


def main():
    data_dir = TASK_ROOT / "tests" / "data"
    data_dir.mkdir(parents=True, exist_ok=True)
    seen_args = set()
    write_cases(data_dir / "visible.jsonl", 2000, seen_args)
    write_cases(data_dir / "hidden.jsonl", 2000, seen_args)


if __name__ == "__main__":
    main()

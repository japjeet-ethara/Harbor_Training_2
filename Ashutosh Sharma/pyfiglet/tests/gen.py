#!/usr/bin/env python3

import json
import io
from pathlib import Path
import random
import sys


FONTS = ("standard", "slant", "small", "big", "mini", "banner", "shadow", "script")
WIDTHS = (10, 20, 40, 60, 80, 120)
JUSTIFICATIONS = ("auto", "left", "center", "right")
DIRECTIONS = ("auto", "left-to-right", "right-to-left")

NORMAL_TEXTS = (
    "hello world",
    "pyfiglet",
    "ASCII art",
    "FIGlet fonts",
    "quick brown fox",
    "simple text",
    "gopher",
    "good morning",
)
NORMAL_WORDS = ("I", "a", "hi", "go", "ok", "we", "are", "good", "hello", "world")

# These are character combinations described in NOTES.md's smushing section.
NOTES_CASES = (
    "A A",
    "_ |",
    "| /",
    "[ ]",
    "{ }",
    "( )",
    "/ \\",
    "\\ /",
    "> <",
    "$ $",
    "##  ##",
)

OPERATORS = "| / \\ _ [ ] { } ( ) < >"
LONG_SENTENCE = (
    "A long line of text will wrap at a word and keep each art row clear. "
) * 5


def source_root(task_root: Path) -> Path:
    candidates = (task_root.parent / "pyfiglet", task_root)
    for candidate in candidates:
        if (candidate / "pyfiglet" / "__main__.py").is_file():
            return candidate
    raise FileNotFoundError(
        "Could not locate the original pyfiglet checkout next to the task folder "
        "or inside it."
    )


def load_original(source: Path):
    sys.path.insert(0, str(source))
    from pyfiglet import CharNotPrinted, main

    return main, CharNotPrinted


def make_text(index: int, width: int) -> str:
    if width < 40:
        return f"{random.choice(('I', 'A', 'x'))} {OPERATORS}"

    max_word_length = (width - 1) // 15
    normal = random.choice(
        tuple(word for word in NORMAL_WORDS if len(word) <= max_word_length)
    )
    kind = index % 6

    if kind == 0:
        return f"{normal} {random.choice(NOTES_CASES)}"
    if kind == 1:
        return f"{normal} {OPERATORS}"
    if kind == 2:
        return f"{normal}  {random.choice(NORMAL_TEXTS)}"
    if kind == 3:
        return f"{normal} {LONG_SENTENCE}"
    if kind == 4:
        return f"{normal} é ß ñ"
    return f"{normal} 漢 ☃ 🙂"


def make_args(index: int) -> list[str]:
    width = 120 if index % 6 == 3 else random.choice(WIDTHS)
    font = "mini" if width == 10 else random.choice(FONTS)
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
    for option in ("-r", "-F", "-s", "-n"):
        if random.randrange(4) == 0:
            args.append(option)
    args.append(make_text(index, width))
    return args


def render(main, char_not_printed, args: list[str]) -> str | None:
    output = io.BytesIO()
    original_argv = sys.argv
    original_stdout = sys.stdout
    sys.argv = ["pyfiglet", *args]

    class CapturedStdout:
        def detach(self):
            return output

    sys.stdout = CapturedStdout()
    try:
        status = main()
    except char_not_printed:
        return None
    finally:
        sys.argv = original_argv
        sys.stdout = original_stdout

    if status != 0:
        raise RuntimeError(f"Original pyfiglet returned {status} for args {args!r}")
    return output.getvalue().decode("utf-8")


def write_dataset(
    path: Path,
    count: int,
    index_offset: int,
    main,
    char_not_printed,
    seen_args: set[tuple[str, ...]],
) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as output:
        written = 0
        attempts = 0
        while written < count:
            attempts += 1
            if attempts > count * 100:
                raise RuntimeError(f"Could not generate {count} cases for {path}")

            args = make_args(index_offset + written)
            key = tuple(args)
            if key in seen_args:
                continue

            rendered = render(main, char_not_printed, args)
            if rendered is None:
                continue

            seen_args.add(key)
            record = {"args": args, "out": rendered}
            output.write(json.dumps(record, ensure_ascii=False) + "\n")
            output.flush()
            written += 1


def main() -> None:
    random.seed(7)
    task_root = Path(__file__).resolve().parents[1]
    source = source_root(task_root)
    pyfiglet_main, char_not_printed = load_original(source)

    seen_args: set[tuple[str, ...]] = set()
    data_dir = task_root / "tests" / "data"
    write_dataset(
        data_dir / "visible.jsonl",
        2000,
        0,
        pyfiglet_main,
        char_not_printed,
        seen_args,
    )
    write_dataset(
        data_dir / "hidden.jsonl",
        2000,
        2000,
        pyfiglet_main,
        char_not_printed,
        seen_args,
    )


if __name__ == "__main__":
    main()

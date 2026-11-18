import argparse
import itertools
import json
import random
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT.parent / "pyfiglet"))

import pyfiglet
from pyfiglet import Figlet

def safe_render(self, text):
    builder = pyfiglet.FigletBuilder(
        text, self.base.Font, self.base.direction,
        self.base.width, self.base.justify
    )
    seen = set()
    while builder.isNotFinished():
        state = (builder.iterator, tuple(builder.buffer), builder.currentTotalWidth)
        if state in seen:
            raise ValueError("Renderer wrapping loop detected")
        seen.add(state)
        builder.addCharToProduct()
        builder.goToNextChar()
    return builder.returnProduct()

pyfiglet.FigletRenderingEngine.render = safe_render

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFICATIONS = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]
FLAGS = ["-r", "-F", "-s", "-n"]

TEXTS = [
    "hello", "FIGlet", "Go", "12345", "a b", "two words",
    "A", "W", "MW", "[]{}()", "/\\><", "_|", "###",
    "cafe", "café", "naïve", "façade", "über", "ß ñ é",
    "Ω", "€", "日本", "漢字", "hello world",
    "The quick brown fox", "1234567890", "AaBbCc",
    "abcdefghij", "WWWWWWWW", "iiiiiiii", "____",
    "((()))", "{{[]}}", "/\\/\\", "A B C D",
    "long text for wrapping", "   spaces   ",
    "x" * 30, "Go!" * 12, "a\tb", "Mixed CASE 123!"
]

CACHE = {}


def get_figlet(font, width, justify, direction):
    key = (font, width, justify, direction)
    if key not in CACHE:
        CACHE[key] = Figlet(
            font=font,
            width=width,
            justify=justify,
            direction=direction
        )
    return CACHE[key]


def render(font, width, justify, direction, text, flags):
    fig = get_figlet(font, width, justify, direction)

    # Prevent pyfiglet's exact-width single-character wrapping loop.
    for char in text:
        char_width = fig.Font.width.get(ord(char), 0)
        if char_width >= width:
            raise ValueError("Character cannot safely fit this width")

    result = fig.renderText(text)

    if "-r" in flags:
        result = result.reverse()
    if "-F" in flags:
        result = result.flip()
    if "-s" in flags:
        result = result.strip_surrounding_newlines()
    elif "-n" in flags:
        result = result.normalize_surrounding_newlines()

    return str(result)


def make_case(index, combinations):
    font, width, justify, direction = combinations[index % len(combinations)]

    for _ in range(100):
        text = random.choice(TEXTS)
        flags = [flag for flag in FLAGS if random.randrange(4) == 0]

        try:
            output = render(font, width, justify, direction, text, flags)
        except Exception:
            continue

        args = [
            "-f", font, "-w", str(width),
            "-j", justify, "-D", direction
        ]
        args.extend(flags)
        args.append(text)

        return {"args": args, "out": output}

    raise RuntimeError(f"Could not generate case {index}")


def write_cases(path, count, combinations):
    with path.open("w", encoding="utf-8", newline="\n") as file:
        for i in range(count):
            case = make_case(i, combinations)
            file.write(json.dumps(case, ensure_ascii=False) + "\n")

    print(f"Generated {count} cases: {path}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--visible-count", type=int, default=2000)
    parser.add_argument("--hidden-count", type=int, default=2000)
    args = parser.parse_args()

    if args.visible_count < 1 or args.hidden_count < 1:
        raise SystemExit("Case counts must be positive.")

    random.seed(7)

    combinations = list(itertools.product(
        FONTS, WIDTHS, JUSTIFICATIONS, DIRECTIONS
    ))
    random.shuffle(combinations)

    data_dir = ROOT / "tests" / "data"
    data_dir.mkdir(parents=True, exist_ok=True)

    write_cases(data_dir / "visible.jsonl", args.visible_count, combinations)
    write_cases(data_dir / "hidden.jsonl", args.hidden_count, combinations)

    print("Generation complete.")


if __name__ == "__main__":
    main()


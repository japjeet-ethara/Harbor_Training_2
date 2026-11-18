import json
import os
import random
import sys

REPO_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "pyfiglet"))
sys.path.insert(0, REPO_DIR)

from pyfiglet import CharNotPrinted, Figlet

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
NARROW_FONTS = ["small", "mini", "standard", "slant", "script"]
WIDTHS = [30, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

RARE_TEXTS = [
    "||", "_/", "|/", "[]", "{}", "()", "<>", "/\\", "\\/", "><",
    "|_|", "___", "///\\\\\\", "[({})]", "<|>",
    "hello  world", "  leading and trailing  ",
    "The quick brown fox jumps over the lazy dog 1234567890!",
    "Narrow wrap test with multiple long words",
    "café naïve résumé ñ ß ü ö ä",
    "unknown chars: ~`!@#$%^&*-+=;:'\",.?€★",
    "A", "AB", "XYZ", "0123456789",
    "smush | / \\ _ [ ] { } ( ) < > test"
]

WORDS = [
    "hello", "world", "figlet", "golang", "harbor", "docker", "alpine",
    "testing", "banner", "giant", "letters", "smush", "kern", "width",
    "center", "right", "left", "mirror", "flip", "code", "build", "2026"
]

FONT_CACHE = {}

def get_figlet(font, width, justify, direction):
    key = (font, width, justify, direction)
    if key not in FONT_CACHE:
        FONT_CACHE[key] = Figlet(font=font, width=width, justify=justify, direction=direction)
    return FONT_CACHE[key]

def generate_dataset(rng, count, label):
    cases = []
    while len(cases) < count:
        width = rng.choice(WIDTHS)
        font = rng.choice(NARROW_FONTS if width == 10 else FONTS)
        args = ["-f", font, "-w", str(width)]

        justify = "auto"
        if rng.random() < 0.7:
            justify = rng.choice(JUSTIFY)
            args.extend(["-j", justify])

        direction = "auto"
        if rng.random() < 0.7:
            direction = rng.choice(DIRECTION)
            args.extend(["-D", direction])

        reverse = False
        flip = False
        strip_nl = False
        norm_nl = False
        if rng.random() < 0.25:
            reverse = True
            args.append("-r")
        if rng.random() < 0.25:
            flip = True
            args.append("-F")
        if rng.random() < 0.25:
            strip_nl = True
            args.append("-s")
        elif rng.random() < 0.25:
            norm_nl = True
            args.append("-n")

        if rng.random() < 0.5:
            text = rng.choice(RARE_TEXTS)
        else:
            num_words = rng.randint(1, 5)
            text = " ".join(rng.choice(WORDS) for _ in range(num_words))

        args.append(text)
        try:
            fig = get_figlet(font, width, justify, direction)
            out = fig.renderText(text)
            if reverse:
                out = out.reverse()
            if flip:
                out = out.flip()
            if strip_nl:
                out = out.strip_surrounding_newlines()
            elif norm_nl:
                out = out.normalize_surrounding_newlines()
            out_str = str(out)
        except CharNotPrinted:
            continue

        cases.append({"args": args, "out": out_str})
        if len(cases) % 100 == 0:
            print(f"{label}: {len(cases)}/{count} done...", flush=True)
    return cases

def main():
    print("Starting generation...", flush=True)
    rng = random.Random(7)
    out_dir = os.path.join(os.path.dirname(__file__), "data")
    os.makedirs(out_dir, exist_ok=True)

    visible = generate_dataset(rng, 2000, "visible")
    hidden = generate_dataset(rng, 2000, "hidden")

    with open(os.path.join(out_dir, "visible.jsonl"), "w", encoding="utf-8", newline="\n") as f:
        for item in visible:
            f.write(json.dumps(item, ensure_ascii=False) + "\n")

    with open(os.path.join(out_dir, "hidden.jsonl"), "w", encoding="utf-8", newline="\n") as f:
        for item in hidden:
            f.write(json.dumps(item, ensure_ascii=False) + "\n")

    print("Generated 2000 visible and 2000 hidden cases.", flush=True)

if __name__ == "__main__":
    main()
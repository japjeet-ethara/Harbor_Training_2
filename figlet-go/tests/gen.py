#!/usr/bin/env python3
import json
import random
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path


SEED = 7
COUNT = 2000
FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]
WORDS = [
    "hello",
    "Harbor",
    "practice",
    "FIGlet",
    "letters",
    "smushing",
    "slant",
    "wrap",
    "spaces",
    "mirror",
    "kerning",
]
TRICKY = ["|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">", "|_|", "/\\", "[]{}()<>"]
TEXTS = [
    "two  spaces",
    "a long sentence that should wrap when the selected width is narrow",
    "café ß ñ",
    "漢字",
]


def build_case(rng, maximum_widths):
    font = rng.choice(FONTS)
    safe_widths = [width for width in WIDTHS if width > maximum_widths[font]]
    args = [
        "-f",
        font,
        "-w",
        str(rng.choice(safe_widths)),
        "-j",
        rng.choice(JUSTIFY),
        "-D",
        rng.choice(DIRECTIONS),
    ]
    for flag in ("-r", "-F", "-s", "-n"):
        if rng.randrange(4) == 0:
            args.append(flag)
    text_kind = rng.randrange(4)
    if text_kind == 0:
        text = rng.choice(WORDS)
    elif text_kind == 1:
        text = " ".join(rng.choice(WORDS) for _ in range(rng.randint(2, 8)))
    elif text_kind == 2:
        text = rng.choice(TRICKY)
    else:
        text = rng.choice(TEXTS)
    args.append(text)
    return args


def run_original(source_root, args):
    result = subprocess.run(
        [sys.executable, "-m", "pyfiglet", *args],
        cwd=source_root,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
        timeout=5,
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"pyfiglet failed for args {args!r}: "
            f"{result.stderr.decode('utf-8', errors='replace')}"
        )
    return {"args": args, "out": result.stdout.decode("utf-8")}


def generate(source_root, output_path, rng, seen, maximum_widths):
    cases = []
    for _ in range(COUNT):
        for _attempt in range(1000):
            args = build_case(rng, maximum_widths)
            key = tuple(args)
            if key in seen:
                continue
            seen.add(key)
            cases.append(args)
            break
        else:
            raise RuntimeError("Could not generate a unique case after 1000 attempts")

    with ThreadPoolExecutor(max_workers=8) as executor:
        outputs = list(executor.map(lambda args: run_original(source_root, args), cases))
    lines = [json.dumps(case, ensure_ascii=False) for case in outputs]
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main():
    task_root = Path(__file__).resolve().parents[1]
    source_root = task_root.parent / "pyfiglet"
    if not (source_root / "pyfiglet").is_dir():
        raise FileNotFoundError(
            f"Expected the pinned pyfiglet checkout at {source_root}"
        )
    sys.path.insert(0, str(source_root))
    from pyfiglet import FigletFont

    maximum_widths = {
        font: max(FigletFont(font).width.values())
        for font in FONTS
    }

    random.seed(SEED)
    rng = random
    seen = set()
    data_dir = task_root / "tests" / "data"
    generate(source_root, data_dir / "visible.jsonl", rng, seen, maximum_widths)
    generate(source_root, data_dir / "hidden.jsonl", rng, seen, maximum_widths)


if __name__ == "__main__":
    main()

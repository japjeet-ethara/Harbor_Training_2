
import json
import random
import subprocess
import sys
from pathlib import Path

random.seed(7)

TASK_DIR = Path(__file__).resolve().parents[1]
PROJECT_DIR = TASK_DIR.parent
PYFIGLET_DIR = PROJECT_DIR / "pyfiglet"
DATA_DIR = TASK_DIR / "tests" / "data"

FONTS = [
    "standard", "slant", "small", "big",
    "mini", "banner", "shadow", "script"
]

WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFICATIONS = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]

TEXTS = [
    # Normal words
    "hello", "world", "figlet", "machine",
    "giant letters", "python to go",
    "the quick brown fox", "hello world",
    "open source software", "testing narrow widths",
    "Go language", "right to left",
    "center alignment", "abcdefghijklmnopqrstuvwxyz",

    # Long sentences and spacing
    "a long sentence that should wrap across multiple lines",
    "multiple words with double  spaces",
    "  leading spaces", "trailing spaces  ",
    "one two three four five six seven eight nine ten",

    # Special characters
    "||", "|||", "/\\", "\\/", "><",
    "_/", "/_", "$$", "$|", "|$",
    "[]", "{}", "()", "<>",
    "| / \\ _ [ ] { } ( ) < >",
    "////\\\\\\\\", "____----", "hello!!!",

    # Smushing combinations from NOTES.md
    "XA AY",
    "X_ |Y",
    "X| /Y",
    "X/ [Y",
    "X[ {Y",
    "X{ (Y",
    "X( <Y",
    "X[ ]Y",
    "X] [Y",
    "X{ }Y",
    "X( )Y",
    "X< >Y",
    "X/ \\Y",
    "X\\ /Y",
    "X> <Y",
    "X$ $Y",
    "X$ AY",
    "AA BB",

    # Latin-1 characters
    "é", "ß", "ñ", "café", "façade", "über",

    # Characters some fonts may not contain
    "日本語", "🙂",

    # Mixed letters and numbers
    "A1 B2 C3", "1234567890",
]

def make_case():
    font = random.choice(FONTS)
    width = random.choice(WIDTHS)
    justify = random.choice(JUSTIFICATIONS)
    direction = random.choice(DIRECTIONS)
    text = random.choice(TEXTS)

    args = [
        "-f", font,
        "-w", str(width),
        "-j", justify,
        "-D", direction,
    ]

    for flag in ["-r", "-F", "-s", "-n"]:
        if random.randrange(4) == 0:
            args.append(flag)

    args.append(text)
    return args


def render_case(args):
    code = r'''
import sys
from pyfiglet import Figlet

args = sys.argv[1:]

font = args[1]
width = int(args[3])
justify = args[5]
direction = args[7]
text = args[-1]

figlet = Figlet(
    font=font,
    width=width,
    justify=justify,
    direction=direction,
)

result = figlet.renderText(text)

if "-r" in args:
    result = result.reverse()

if "-F" in args:
    result = result.flip()

if "-s" in args:
    result = result.strip_surrounding_newlines()
elif "-n" in args:
    result = result.normalize_surrounding_newlines()

sys.stdout.buffer.write(result.encode("UTF-8") + b"\n")
'''

    env = None
    import os
    env = os.environ.copy()
    env["PYTHONPATH"] = str(PYFIGLET_DIR)

    completed = subprocess.run(
        [sys.executable, "-c", code, *args],
        cwd=str(PYFIGLET_DIR),
        env=env,
        capture_output=True,
        timeout=3,
    )

    if completed.returncode != 0:
        raise RuntimeError(
            completed.stderr.decode("utf-8", errors="replace")
        )

    return completed.stdout.decode("utf-8")


def generate_dataset(filename, count, seen_args):
    output_path = DATA_DIR / filename

    generated = 0
    attempts = 0
    max_attempts = count * 20

    # Always start with a fresh file. Never resume old records.
    with output_path.open("w", encoding="utf-8", newline="\n") as file:
        while generated < count and attempts < max_attempts:
            attempts += 1
            args = make_case()
            key = json.dumps(args, ensure_ascii=False)
            if key in seen_args:
                continue

            try:
                output = render_case(args)

            except subprocess.TimeoutExpired:
                print(
                    f"TIMEOUT: {filename}, attempt {attempts}, args={args}",
                    flush=True,
                )
                continue

            except Exception as error:
                print(
                    f"SKIPPED: {type(error).__name__}: {error}",
                    flush=True,
                )
                continue

            record = {"args": args, "out": output}
            seen_args.add(key)

            file.write(json.dumps(record, ensure_ascii=False) + "\n")
            file.flush()

            generated += 1

            if generated % 25 == 0:
                print(
                    f"{filename}: {generated}/{count}",
                    flush=True,
                )

    if generated != count:
        raise RuntimeError(
            f"{filename}: only generated {generated}/{count} cases"
        )

    print(f"{filename}: completed {generated} cases", flush=True)
def main():
    if not PYFIGLET_DIR.is_dir():
        raise SystemExit(f"Cannot find pyfiglet folder: {PYFIGLET_DIR}")

    DATA_DIR.mkdir(parents=True, exist_ok=True)
    seen_args = set()

    print("Generating visible dataset...", flush=True)
    generate_dataset("visible.jsonl", 2000, seen_args)

    print("Generating hidden dataset...", flush=True)
    generate_dataset("hidden.jsonl", 2000, seen_args)

    print("Generation complete.", flush=True)


if __name__ == "__main__":
    main()

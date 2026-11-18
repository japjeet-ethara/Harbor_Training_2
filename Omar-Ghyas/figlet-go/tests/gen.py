
#!/usr/bin/env python3

from __future__ import annotations

import io
import json
import pathlib
import random
import sys
import time

# Locate the original pyfiglet repository
TESTS_DIR = pathlib.Path(__file__).resolve().parent
TASK_DIR = TESTS_DIR.parent

PRIMARY_PYFIGLET_DIR = pathlib.Path("C:/HarborPractice/pyfiglet").resolve()
FALLBACK_PYFIGLET_DIR = (TASK_DIR.parent / "pyfiglet").resolve()

if PRIMARY_PYFIGLET_DIR.is_dir():
    pyfiglet_dir = PRIMARY_PYFIGLET_DIR
elif FALLBACK_PYFIGLET_DIR.is_dir():
    pyfiglet_dir = FALLBACK_PYFIGLET_DIR
else:
    print(
        f"Error: pyfiglet repository not found at "
        f"{PRIMARY_PYFIGLET_DIR} or {FALLBACK_PYFIGLET_DIR}",
        file=sys.stderr
    )
    sys.exit(1)

if str(pyfiglet_dir) not in sys.path:
    sys.path.insert(0, str(pyfiglet_dir))

try:
    import pyfiglet
except ImportError as exc:
    print(
        f"Error: failed to import pyfiglet from {pyfiglet_dir}: {exc}",
        file=sys.stderr
    )
    sys.exit(1)


DATA_DIR = TESTS_DIR / "data"
VISIBLE_PATH = DATA_DIR / "visible.jsonl"
HIDDEN_PATH = DATA_DIR / "hidden.jsonl"

FONTS = [
    "standard", "slant", "small", "big",
    "mini", "banner", "shadow", "script"
]

WIDTHS = [10, 20, 40, 60, 80, 120]

JUSTIFICATIONS = ["auto", "left", "center", "right"]

DIRECTIONS = ["auto", "left-to-right", "right-to-left"]

FONTS_LOADED = {
    font: pyfiglet.Figlet(font=font).Font
    for font in FONTS
}


NORMAL_WORDS = [
    "Hello", "World", "Go", "figlet", "Terminal", "code",
    "ASCII", "art", "test", "Harbor", "12345", "agent",
    "system", "run", "fast", "simple", "output", "stream",
    "buffer", "builder", "Linux", "Docker", "Python",
    "OpenSource", "Computer", "Font", "Render", "Command",
    "Option", "Arg", "compiler", "variable", "function",
    "package", "binary", "benchmark"
]


SMUSH_COMBINATIONS = [
    "||", "//", "\\\\", "++", "==", "##", "::", "**",
    "@@", "%%",
    "_|_", "_/", "\\_", "_[_", "_]_", "_{_", "_}_",
    "_(_", "_)_", "_<_", "_>_", "_/_", "_\\_",
    "_A_", "_|_/_", "_\\_|_",
    "|/", "/|", "|\\", "\\|", "/[", "]/", "/{", "}/",
    "/<", ">/", "|[", "]|", "|{", "}|", "|(", ")|",
    "|<", ">|", "/[", "]\\", "\\[", "/{", "}\\",
    "\\{", "}/", "[{", "}]", "[<", ">]", "{(", ")}",
    "{<", ">}", "(<", ">)", "[]", "][", "{}", "}{",
    "()", ")(", "[()]", "{[]}", "({})", "][{}",
    "/\\", "\\/", "><", "<>", "/\\/\\", ">|<",
    "[|]", "(|)", "{|}", "()[]{}", "_|_/\\_",
    "|<_>|", "(<||>)", "[/\\/]", "<\\/ >", "{_|_}",
    "|/\\|", "/\\/\\/\\"
]


HARDBLANK_TEXTS = [
    "$", "$$", "$$$", "$$$$", "$100", "foo$bar",
    "A$B$C", "$test$", "Price: $50", "Cash: $$",
    "$$/$$", "100$ + 200$", "$$$$ money",
    "$ [|] $", "$/\\$", "_$_", "$foo  bar$", "cost: $9.99"
]


DOUBLE_SPACES_TEXTS = [
    "hello  world", "foo   bar", "a  b   c    d",
    "leading  space", "space  in  middle",
    "double  space  test", "one   two  three",
    "word1  word2   word3", "A  B   C",
    "Go  Figlet   Run", "test  1  2  3",
    "foo   bar    baz", "  leading  and  trailing  ",
    "two  spaces  here", "multiple   spaces   test"
]


LATIN_TEXTS = [
    "é", "ß", "ñ", "café", "año", "Groß", "Straße",
    "über", "naïve", "München", "niño", "Äpfel",
    "Öl", "Übung", "weiß", "crème", "façade",
    "piñata", "résumé", "señor", "Fußball", "Häuser",
    "élan", "voilà", "jalapeño", "cliché",
    "Ä Ö Ü ä ö ü ß", "Señor Café", "Große Straße",
    "Niño über café"
]


MISSING_TEXTS = [
    "Hello 世界", "Go ❤️", "Russian Дa",
    "Emoji 🚀", "Greek αβγ", "∞ infinity",
    "Arabic مرحبا", "★ star ☆", "✓ check",
    "☺ smile", "Test テスト", "♫ music ♬",
    "▶ play ◀", "CJK: 你好", "⚙ gears ✅",
    "✨ magic ✨"
]


LONG_SENTENCES = [
    "The quick brown fox jumps over the lazy dog.",
    "Pack my box with five dozen liquor jugs.",
    "How vexingly quick daft zebras jump!",
    "Sphinx of black quartz, judge my vow.",
    "FIGlet is a computer program that generates text banners in various decorative ASCII fonts.",
    "Porting pyfiglet to Go requires implementing smushing, kerning, justification, and direction rules.",
    "A very long sentence designed specifically to test terminal width wrapping across multiple lines with various widths like 10, 20, 40, and 60.",
    "To be or not to be, that is the question: whether 'tis nobler in the mind to suffer the slings and arrows of outrageous fortune.",
    "Two households, both alike in dignity, In fair Verona, where we lay our scene, From ancient grudge break to new mutiny.",
    "All that glitters is not gold; Often have you heard that told; Many a man his life hath sold Outside to behold.",
    "A long text with symbols: [alpha] /\\ {beta} >< (gamma) | _low_ and double  spaces throughout.",
    "Wrapping test: narrow widths require splitting lines cleanly across words or at max column boundaries."
]


def generate_text() -> str:
    category = random.choices(
        [
            "normal", "smush", "hardblank", "spaces",
            "latin", "missing", "long", "composite",
            "dynamic_smush"
        ],
        weights=[14, 18, 10, 14, 12, 10, 8, 14, 10],
        k=1
    )[0]

    if category == "normal":
        if random.random() < 0.5:
            text = random.choice(NORMAL_WORDS)
        else:
            text = (
                f"{random.choice(NORMAL_WORDS)} "
                f"{random.choice(NORMAL_WORDS)}"
            )

    elif category == "smush":
        text = random.choice(SMUSH_COMBINATIONS)

    elif category == "hardblank":
        text = random.choice(HARDBLANK_TEXTS)

    elif category == "spaces":
        text = random.choice(DOUBLE_SPACES_TEXTS)

    elif category == "latin":
        text = random.choice(LATIN_TEXTS)

    elif category == "missing":
        text = random.choice(MISSING_TEXTS)

    elif category == "long":
        text = random.choice(LONG_SENTENCES)

    elif category == "dynamic_smush":
        chars = list("|/\\_[]{}()<>$") + ["A", "x", "+", "="]
        length = random.randint(2, 6)
        text = "".join(random.choices(chars, k=length))

    else:
        parts = []
        count = random.randint(2, 4)

        for _ in range(count):
            pool = random.choice([
                NORMAL_WORDS,
                SMUSH_COMBINATIONS,
                HARDBLANK_TEXTS,
                LATIN_TEXTS,
                MISSING_TEXTS
            ])
            parts.append(random.choice(pool))

        separator = random.choice([" ", "  "])
        text = separator.join(parts)

    while text.startswith("-"):
        text = text[1:]

    return text if text else "figlet"



def run_pyfiglet(args: list[str]) -> str | None:
    """Execute pyfiglet.main() and capture stdout byte-for-byte."""
    buf = io.BytesIO()
    old_stdout = sys.stdout
    old_argv = sys.argv

    wrapper = io.TextIOWrapper(
        buf,
        encoding="utf-8",
        newline="\n"
    )

    try:
        sys.stdout = wrapper
        sys.argv = ["pyfiglet"] + args

        ret = pyfiglet.main()

        if ret not in (0, None):
            return None

        wrapper.flush()

        return buf.getvalue().decode("utf-8")

    except Exception:
        return None

    finally:
        sys.stdout = old_stdout
        sys.argv = old_argv


def generate_case() -> tuple[list[str], str] | None:
    font = random.choice(FONTS)
    width = random.choice(WIDTHS)
    justify = random.choice(JUSTIFICATIONS)
    direction = random.choice(DIRECTIONS)

    text = generate_text()

    font_object = FONTS_LOADED[font]

    # Avoid characters that cannot fit within the requested width.
    if any(
        font_object.width.get(ord(char), 0) >= width
        for char in text
    ):
        return None

    args = [
        "-f", font,
        "-w", str(width),
        "-j", justify,
        "-D", direction
    ]

    # Each optional flag is independently selected.
    if random.random() < 0.25:
        args.append("-r")

    if random.random() < 0.25:
        args.append("-F")

    if random.random() < 0.25:
        args.append("-s")

    if random.random() < 0.25:
        args.append("-n")

    args.append(text)

    output = run_pyfiglet(args)

    if output is None:
        return None

    return args, output


def generate_dataset(
    count: int,
    name: str,
    used_inputs: set[str]
) -> list[dict]:
    dataset = []
    attempts = 0

    while len(dataset) < count:
        attempts += 1
        case = generate_case()
        if case is None:
            continue

        args, output = case
        input_key = json.dumps(args, ensure_ascii=False)

        if input_key in used_inputs:
            continue

        used_inputs.add(input_key)
        dataset.append({
            "args": args,
            "out": output
        })

        if len(dataset) % 500 == 0:
            print(
                f"[{name}] Generated {len(dataset)}/{count} "
                f"cases ({attempts} attempts)...",
                flush=True
            )

    return dataset


def write_jsonl(path: pathlib.Path, dataset: list[dict]) -> None:
    with path.open("w", encoding="utf-8", newline="\n") as file:
        for item in dataset:
            file.write(
                json.dumps(item, ensure_ascii=False) + "\n"
            )


def main():
    start_time = time.time()
    random.seed(7)

    DATA_DIR.mkdir(parents=True, exist_ok=True)

    used_inputs = set()

    print("Generating 2000 visible cases...", flush=True)
    visible_data = generate_dataset(2000, "visible", used_inputs)
    write_jsonl(VISIBLE_PATH, visible_data)
    print(
        f"Wrote {len(visible_data)} visible cases to {VISIBLE_PATH}",
        flush=True
    )

    print("Generating 2000 hidden cases...", flush=True)
    hidden_data = generate_dataset(2000, "hidden", used_inputs)

    # Explicit verification checks
    visible_keys = [json.dumps(item["args"], ensure_ascii=False) for item in visible_data]
    hidden_keys = [json.dumps(item["args"], ensure_ascii=False) for item in hidden_data]
    unique_visible = set(visible_keys)
    unique_hidden = set(hidden_keys)
    overlap = unique_visible & unique_hidden

    if len(visible_data) != 2000:
        raise RuntimeError(f"Expected exactly 2000 visible cases, got {len(visible_data)}")

    if len(hidden_data) != 2000:
        raise RuntimeError(f"Expected exactly 2000 hidden cases, got {len(hidden_data)}")

    if len(unique_visible) != 2000:
        raise RuntimeError(f"Expected exactly 2000 unique visible argument lists, got {len(unique_visible)}")

    if len(unique_hidden) != 2000:
        raise RuntimeError(f"Expected exactly 2000 unique hidden argument lists, got {len(unique_hidden)}")

    if len(overlap) != 0:
        raise RuntimeError(f"Found {len(overlap)} overlapping argument lists between visible and hidden datasets")

    print("Verification passed:", flush=True)
    print(f"- Exactly {len(visible_data)} visible cases.", flush=True)
    print(f"- Exactly {len(hidden_data)} hidden cases.", flush=True)
    print(f"- Exactly {len(unique_visible)} unique visible argument lists.", flush=True)
    print(f"- Exactly {len(unique_hidden)} unique hidden argument lists.", flush=True)
    print("- Zero overlap between visible and hidden argument lists.", flush=True)

    # Write the hidden dataset only after all validation checks pass
    write_jsonl(HIDDEN_PATH, hidden_data)
    print(
        f"Wrote {len(hidden_data)} hidden cases to {HIDDEN_PATH}",
        flush=True
    )

    elapsed = time.time() - start_time
    print(f"Completed in {elapsed:.2f} seconds.", flush=True)


if __name__ == "__main__":
    main()

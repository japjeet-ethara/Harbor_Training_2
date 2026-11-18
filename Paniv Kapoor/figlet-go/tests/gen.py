#!/usr/bin/env python3
"""
Deterministic test-case generator for figlet-go.
Generates test cases by invoking the original PyFiglet CLI from the sibling pyfiglet directory.
Outputs:
  - figlet-go/tests/data/visible.jsonl (2000 cases)
  - figlet-go/tests/data/hidden.jsonl (2000 distinct cases)
"""

import hashlib
import json
import os
import random
import subprocess
import sys
import time
from collections import Counter
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

# Seed must be set before any random generation
RANDOM_SEED = 7
random.seed(RANDOM_SEED)

# Locate directories
SCRIPT_DIR = Path(__file__).resolve().parent
REPO_DIR = SCRIPT_DIR.parent
WORKSPACE_DIR = REPO_DIR.parent
PYFIGLET_DIR = (WORKSPACE_DIR / "pyfiglet").resolve()
DATA_DIR = SCRIPT_DIR / "data"
VISIBLE_PATH = DATA_DIR / "visible.jsonl"
HIDDEN_PATH = DATA_DIR / "hidden.jsonl"

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFICATIONS = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]
FLAG_COMBOS = [
    [],
    ["-r"],
    ["-F"],
    ["-s"],
    ["-n"],
    ["-r", "-F"],
    ["-r", "-s"],
    ["-r", "-n"],
    ["-F", "-s"],
    ["-F", "-n"],
    ["-s", "-n"],  # tests precedence: -s wins over -n
    ["-r", "-F", "-s"],
    ["-r", "-F", "-n"],
    ["-r", "-s", "-n"],
    ["-F", "-s", "-n"],
    ["-r", "-F", "-s", "-n"],
]

# Characters guaranteed to fit in width 10 across fonts
W10_NARROW_CHARS = [
    "i", "l", "!", ".", ":", ";", "1", "a", "c", "e", "o", "r", "s",
    "t", "u", "v", "x", "z", "_", "|", "/", "\\", "[", "]", "(", ")",
    "{", "}", "<", ">"
]

# Words guaranteed to fit in width 20 across fonts
W20_WORDS = [
    "hi", "go", "cat", "dog", "12", "ok", "run", "test", "code",
    "font", "py", "sm", "bg", "x", "y", "z", "yes", "no", "up",
    "top", "end", "one", "two", "six", "ten", "box", "key"
]

# Smushing rule test cases according to NOTES.md
EQUAL_SMUSH = [
    "AA", "BB", "CC", "DD", "EE", "FF", "GG", "HH", "II", "JJ", "KK", "LL",
    "MM", "NN", "OO", "PP", "QQ", "RR", "SS", "TT", "UU", "VV", "WW", "XX",
    "YY", "ZZ", "aa", "bb", "cc", "dd", "ee", "ff", "gg", "hh", "ii", "jj",
    "kk", "ll", "mm", "nn", "oo", "pp", "qq", "rr", "ss", "tt", "uu", "vv",
    "ww", "xx", "yy", "zz", "00", "11", "22", "33", "44", "55", "66", "77",
    "88", "99", "==", "++", "**", "##", "@@", "%%", "&&", "::", "__", "||"
]

UNDERSCORE_SMUSH = [
    "_|", "|_", "_/", "/_", "_\\", "\\_", "_[", "]_", "_]", "[_",
    "_{", "}_", "_}", "{_", "_(", ")_", "_)", "(_", "_<", ">_", "_>", "<_",
    "_|_/_\\_[]_{}_()_<>", "<>_()_{}_[]_\\_/_|_", "a_|_b_/_c", "x_[_y_]_z",
    "1_{_2_}_3", "p_(_q_)_r", "m_<_n_>_o", "_=_+_*_#_"
]

HIERARCHY_SMUSH = [
    "|/", "/|", "|\\", "\\|",
    "/[", "[/", "\\]", "]\\", "/]", "]/", "[\\", "\\[",
    "[{", "{[", "]{", "{]", "[}", "}]",
    "{(", "({", "}{", "){", "{)", "){",
    "(<", ">(", ")<", ">)", "(>", "<)",
    "|<", ">|", "|(", ")|", "|{", "}|", "|[", "]|",
    "/<", ">/", "\\<", ">\\", "/{", "}/", "/(", ")/",
    "[<", ">[", "]<", ">]", "[(", ")[",
    "{<", ">{", "}<", ">}",
    "|/\\_[]{}()<>"
]

OPPOSITE_PAIR_SMUSH = [
    "[]", "][", "{}", "}{", "()", ")(",
    "([{}])", "][}{)(", "[([{}])]", "()[]{}", "({[]})", "{[()]}", "][)(}{",
    "[(])", "{[}]", "([)]", "{[(])}"
]

BIG_X_SMUSH = [
    "/\\", "\\/", "><",
    "/\\/\\", "\\/\\/", "><><",
    "/\\\\/><", "><\\//\\",
    "A/\\B", "X\\/Y", "M><N",
    "/\\/\\/\\", "\\/\\/\\/", "><><><"
]

SYMBOLS = [
    "| / \\ _ [ ] { } ( ) < >",
    ">< () {} [] \\ / _ |",
    "[test] <val> {key} (arg) /path\\file _var | pipe",
    "a|b/c\\d_e[f]g{h}i(j)k<l>m",
    "() [] {} <> /\\ | _",
    "if (x < y) { return [x, y]; } // path\\to | sort",
    "<><><> [][][] {{{}}} ((())) ///\\\\\\ ||| ___",
    "|\\_/[{()}]\\_/|",
    "\\_||_/",
    "(a + b) * [c - d] / {e ^ f} <g> | h",
    "| | / / \\ \\ _ _ [ [ ] ] { { } } ( ( ) ) < < > >",
    "x = y ? [a] : {b}; (c < d) / (e > f) | g \\ h _ i",
    "<tag> [index] {scope} (group) /path\\name _id | chain"
]

DOUBLE_SPACES = [
    "hello  world",
    "foo   bar",
    "double  spaces  in  sentence",
    "a  b  c  d  e",
    "word1    word2",
    "start  middle   end",
    "code   review   system",
    "two  spaces",
    "left   right",
    "figlet   go   port",
    "smush  rule   applied",
    "multi   word    with     varying      spaces",
    "  leading and trailing  ",
    "double  space  with  symbols  |  /  \\"
]

LONG_SENTENCES = [
    "The quick brown fox jumps over the lazy dog.",
    "FIGlet is a program that creates large characters out of ordinary screen characters.",
    "All human beings are born free and equal in dignity and rights.",
    "In computer science, determinism ensures reproducible results across all systems.",
    "Sphinx of black quartz, judge my vow, while pack my box with five dozen liquor jugs.",
    "Smushing and kerning create compact ASCII art representations of text.",
    "PyFiglet porting to Go standard library with zero external dependencies.",
    "To be or not to be, that is the question: whether tis nobler in the mind.",
    "One small step for man, one giant leap for mankind.",
    "The only thing we have to fear is fear itself.",
    "Beauty is truth, truth beauty, that is all ye know on earth, and all ye need to know.",
    "Somewhere, something incredible is waiting to be known."
]

LATIN1_AND_UNSUPPORTED = [
    # Supported Latin-1 (present in fonts)
    "Straße", "München", "über", "Groß", "Fräulein", "schön", "Maße", "Süß",
    "ä ö ü ß", "Über die Straße",
    # Unsupported Latin-1
    "café", "español", "naïve", "façade", "crème brûlée", "señor", "niño", "château", "voilà",
    "Latin-1: é, ß, ñ, ü, ö, ä, ç, à, è, ù, î, ô, û",
    # Non-Latin Unsupported
    "Hello 🚀", "Test 🎉 🔥", "Agent 🤖", "世界", "日本語", "こんにちは",
    "∑ ∫ √ π ∞ ≠ ≤ ≥", "★ ♠ ♣ ♥ ♦", "€ 100", "£ 50", "¥ 500",
    "┌─┐│└─┘", "← ↑ → ↓",
    # Mixed
    "Café Straße 🚀", "München 世界", "ñ & é & ß", "Symbol [★] and {€}"
]

MULTI_ARGS = [
    ["hello", "world"],
    ["the", "quick", "fox"],
    ["figlet", "in", "go"],
    ["a", "b", "c"],
    ["one", "two", "three", "four"],
    ["smush", "and", "kern"],
    ["test", "case", "generation"],
    ["porting", "pyfiglet", "to", "go"],
    ["deterministic", "outputs", "verified"]
]


def build_args(font: str, width: int, justify: str, direction: str, flags: list[str], text_item, omit_defaults: bool = True) -> list[str]:
    """Construct argument list, optionally omitting default values to test CLI defaults."""
    args = []
    # Font (default: standard)
    if not (omit_defaults and font == "standard" and random.random() < 0.4):
        args.extend(["-f", font])
    # Width (default: 80)
    if not (omit_defaults and width == 80 and random.random() < 0.4):
        args.extend(["-w", str(width)])
    # Justification (default: auto)
    if not (omit_defaults and justify == "auto" and random.random() < 0.4):
        args.extend(["-j", justify])
    # Direction (default: auto)
    if not (omit_defaults and direction == "auto" and random.random() < 0.4):
        args.extend(["-D", direction])
    # Flags
    for flag in flags:
        args.append(flag)
    # Positional text
    if isinstance(text_item, list):
        args.extend(text_item)
    else:
        args.append(text_item)
    return args


def get_text_for_category(category: str, width: int, font: str):
    """Select text suited to the given test category and width constraints."""
    if width == 10:
        if font == "mini" and random.random() < 0.5:
            return random.choice(["hi", "go", "cat", "dog", "12", "ok", "run", "a", "b", "c", "*", "!"])
        return random.choice(W10_NARROW_CHARS)
    elif width == 20:
        if random.random() < 0.6:
            return random.choice(W20_WORDS)
        else:
            return random.choice(W10_NARROW_CHARS)

    # For width >= 40:
    if category == "smushing":
        subcat = random.choice(["equal", "underscore", "hierarchy", "opposite", "big_x", "kerning"])
        if subcat == "equal":
            return random.choice(EQUAL_SMUSH)
        elif subcat == "underscore":
            return random.choice(UNDERSCORE_SMUSH)
        elif subcat == "hierarchy":
            return random.choice(HIERARCHY_SMUSH)
        elif subcat == "opposite":
            return random.choice(OPPOSITE_PAIR_SMUSH)
        elif subcat == "big_x":
            return random.choice(BIG_X_SMUSH)
        else:
            return random.choice(["BANNER KERNING", "SMUSH RULE", "UNIVERSAL OVERLAP", "FIGLET TEST"])
    elif category == "symbols":
        return random.choice(SYMBOLS)
    elif category == "latin_and_unsupported":
        return random.choice(LATIN1_AND_UNSUPPORTED)
    elif category == "spacing_and_sentences":
        subcat = random.choice(["double_spaces", "long_sentence", "multi_arg", "newlines"])
        if subcat == "double_spaces":
            return random.choice(DOUBLE_SPACES)
        elif subcat == "long_sentence":
            return random.choice(LONG_SENTENCES)
        elif subcat == "multi_arg":
            return random.choice(MULTI_ARGS)
        else:
            return random.choice(["line1\nline2", "hello\nworld", "first\nsecond\nthird", "A\n\nB"])
    elif category == "widths_and_wrapping":
        return random.choice(LONG_SENTENCES + DOUBLE_SPACES + SYMBOLS)
    elif category == "options_and_combos":
        pool = EQUAL_SMUSH + SYMBOLS + LATIN1_AND_UNSUPPORTED + DOUBLE_SPACES + ["Hello World", "Testing 1 2 3"]
        return random.choice(pool)
    else:
        return "Hello World"


def generate_set(set_name: str, target_count: int, seen_cases: set[tuple[str, ...]]) -> list[list[str]]:
    """Generate a diverse and deterministic list of test arguments for visible or hidden sets."""
    cases = []
    allocations = [
        ("smushing", 400),
        ("symbols", 300),
        ("latin_and_unsupported", 300),
        ("spacing_and_sentences", 300),
        ("widths_and_wrapping", 350),
        ("options_and_combos", 350),
    ]
    assert sum(count for _, count in allocations) == target_count, "Allocation sum mismatch"

    for category, count in allocations:
        generated = 0
        attempts = 0
        max_attempts = count * 200
        while generated < count and attempts < max_attempts:
            attempts += 1
            font = random.choice(FONTS)
            width = random.choice(WIDTHS)
            justify = random.choice(JUSTIFICATIONS)
            direction = random.choice(DIRECTIONS)
            flags = random.choice(FLAG_COMBOS)
            text_item = get_text_for_category(category, width, font)

            args = build_args(font, width, justify, direction, flags, text_item)
            args_key = tuple(args)
            if args_key in seen_cases:
                continue

            seen_cases.add(args_key)
            cases.append(args)
            generated += 1

        if generated < count:
            raise RuntimeError(
                f"Failed to generate {count} unique cases for category {category} in {set_name} (got {generated})"
            )

    return cases


def run_pyfiglet(args: list[str]) -> str:
    """Execute PyFiglet CLI from the sibling directory and return exact stdout string."""
    env = os.environ.copy()
    env["PYTHONPATH"] = str(PYFIGLET_DIR)
    proc = subprocess.run(
        [sys.executable, "-m", "pyfiglet", *args],
        cwd=str(PYFIGLET_DIR),
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if proc.returncode != 0:
        stderr_msg = proc.stderr.decode("utf-8", errors="replace")
        raise RuntimeError(
            f"PyFiglet CLI failed (code {proc.returncode}) for args {args}:\n{stderr_msg}"
        )
    return proc.stdout.decode("utf-8")


def write_jsonl_file(path: Path, cases: list[list[str]], outputs: list[str]) -> None:
    """Write test cases and expected outputs to a JSONL file."""
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        for args, out in zip(cases, outputs):
            record = {"args": args, "out": out}
            f.write(json.dumps(record, ensure_ascii=False) + "\n")


def compute_file_sha256(path: Path) -> str:
    """Compute SHA-256 hash of a file."""
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest()


def count_file_lines(path: Path) -> int:
    """Count number of lines in a file."""
    with open(path, "r", encoding="utf-8") as f:
        return sum(1 for _ in f)


def print_coverage_summary(name: str, cases: list[list[str]], path: Path) -> None:
    """Print structured coverage summary for a generated test set."""
    fonts = Counter()
    widths = Counter()
    justifications = Counter()
    directions = Counter()
    flags_cnt = Counter()

    for args in cases:
        f = "standard"
        if "-f" in args:
            f = args[args.index("-f") + 1]
        fonts[f] += 1

        w = 80
        if "-w" in args:
            w = int(args[args.index("-w") + 1])
        widths[w] += 1

        j = "auto"
        if "-j" in args:
            j = args[args.index("-j") + 1]
        justifications[j] += 1

        d = "auto"
        if "-D" in args:
            d = args[args.index("-D") + 1]
        directions[d] += 1

        for fl in ["-r", "-F", "-s", "-n"]:
            if fl in args:
                flags_cnt[fl] += 1

    line_count = count_file_lines(path)
    file_sha = compute_file_sha256(path)

    print(f"\n==========================================")
    print(f"Coverage Summary: {name} ({path.name})")
    print(f"==========================================")
    print(f"Total Lines: {line_count}")
    print(f"SHA-256: {file_sha}")
    print(f"Fonts ({len(fonts)}): {dict(sorted(fonts.items()))}")
    print(f"Widths ({len(widths)}): {dict(sorted(widths.items()))}")
    print(f"Justifications ({len(justifications)}): {dict(sorted(justifications.items()))}")
    print(f"Directions ({len(directions)}): {dict(sorted(directions.items()))}")
    print(f"Option Flags: {dict(sorted(flags_cnt.items()))}")


def main() -> None:
    t_start = time.time()
    print("=" * 60)
    print("PyFiglet Test Case Generator (figlet-go)")
    print(f"Random seed: {RANDOM_SEED}")
    print(f"PyFiglet directory: {PYFIGLET_DIR}")
    print(f"Visible target: {VISIBLE_PATH}")
    print(f"Hidden target: {HIDDEN_PATH}")
    print("=" * 60)

    if not (PYFIGLET_DIR / "pyfiglet").is_dir():
        raise FileNotFoundError(f"PyFiglet source package not found at {PYFIGLET_DIR}")

    # Generate argument lists deterministically
    random.seed(RANDOM_SEED)
    seen_cases: set[tuple[str, ...]] = set()

    print("\n[1/4] Generating 2000 visible argument sets...")
    visible_cases = generate_set("visible", 2000, seen_cases)
    assert len(visible_cases) == 2000

    print("[2/4] Generating 2000 distinct hidden argument sets...")
    hidden_cases = generate_set("hidden", 2000, seen_cases)
    assert len(hidden_cases) == 2000
    assert len(seen_cases) == 4000, f"Expected 4000 distinct cases, got {len(seen_cases)}"

    # Check that visible and hidden have zero overlap
    visible_keys = set(tuple(c) for c in visible_cases)
    hidden_keys = set(tuple(c) for c in hidden_cases)
    intersection = visible_keys.intersection(hidden_keys)
    assert len(intersection) == 0, f"Visible and hidden overlap by {len(intersection)} cases!"

    # Run PyFiglet CLI to generate outputs
    num_workers = min(12, os.cpu_count() or 4)
    print(f"\n[3/4] Executing PyFiglet CLI for visible cases ({num_workers} parallel workers)...")
    t0 = time.time()
    with ThreadPoolExecutor(max_workers=num_workers) as executor:
        visible_outputs = list(executor.map(run_pyfiglet, visible_cases))
    print(f"      Visible outputs generated in {time.time() - t0:.2f}s")
    write_jsonl_file(VISIBLE_PATH, visible_cases, visible_outputs)

    print(f"\n[4/4] Executing PyFiglet CLI for hidden cases ({num_workers} parallel workers)...")
    t0 = time.time()
    with ThreadPoolExecutor(max_workers=num_workers) as executor:
        hidden_outputs = list(executor.map(run_pyfiglet, hidden_cases))
    print(f"      Hidden outputs generated in {time.time() - t0:.2f}s")
    write_jsonl_file(HIDDEN_PATH, hidden_cases, hidden_outputs)

    # Print summaries
    print_coverage_summary("Visible Set", visible_cases, VISIBLE_PATH)
    print_coverage_summary("Hidden Set", hidden_cases, hidden_cases_path := HIDDEN_PATH)

    print(f"\nCompleted in {time.time() - t_start:.2f}s.")


if __name__ == "__main__":
    main()

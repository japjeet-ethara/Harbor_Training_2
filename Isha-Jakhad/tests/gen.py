"""Generate visible.jsonl and hidden.jsonl using the original pyfiglet CLI."""

import json
import os
import random
import subprocess
import sys
from collections import Counter
from pathlib import Path


GEN_PY = Path(__file__).resolve()
REPO_ROOT = GEN_PY.parent.parent.parent
PYFIGLET_PATH = REPO_ROOT / "pyfiglet"
OUT_DIR = GEN_PY.parent / "data"

if not (PYFIGLET_PATH / "pyfiglet" / "__init__.py").exists():
    sys.exit(
        f"ERROR: cannot find pyfiglet package at {PYFIGLET_PATH}.\n"
        "Expected pyfiglet to be a sibling of the figlet-go repository."
    )

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFICATIONS = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]
FLAGS = ["-r", "-F", "-s", "-n"]
TARGET_CASES = 2000
MAX_ATTEMPTS_PER_DATASET = TARGET_CASES * 10
CLI_TIMEOUT_SECONDS = 10

TEXTS = [
    "hello", "world", "foo", "bar", "test", "hi", "go", "ok",
    "Python", "Harbor", "FIGlet", "pyfiglet",
    "||", "//", "\\\\", "[[", "]]", "{{", "}}", "((", "))", "<<", ">>",
    "_|", "_/", "_\\", "_[", "_]", "_{", "_}", "_(", "_)", "_<", "_>",
    "|_", "/_", "\\_", "[_", "]_", "{_", "}_", "(_", ")_", "<_", ">_",
    "|/", "|[", "|{", "|(", "|<", "/[", "/{", "/(", "[{", "[(", "[<",
    "{(", "{<", "(<", "[]", "][", "{}", "}{", "()", ")(", "/\\", "\\/", "><", "$$",
    "|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">",
    "|| // \\\\ _ [] {} () <>",
    "hi  go", "foo  bar", "a  b",
    "quick fox", "lazy dog", "hello world", "pack my box", "black quartz",
    "Hello!", "1+1=2", "100%", "@user", "#tag", "foo-bar", "foo_bar",
    "café", "naïve", "über", "straße", "piñata", "é", "ß", "ñ",
    "漢", "A", "Z", "a", "z", "0", "9", "42", "3.14", "2024", "007",
    "foo123", "Hi", "AB", "XY", "OK",
    "The quick brown fox jumps over the lazy dog and then runs home.",
    "A long sentence with several words should wrap cleanly at narrow widths.",
    "supercalifragilisticexpialidocious",
]
WRAPPING_TEXTS = set(TEXTS[-3:])
LONG_SENTENCES = set(TEXTS[-3:-1])
NORMAL_WORDS = {
    "hello", "world", "foo", "bar", "test", "hi", "go", "ok",
    "Python", "Harbor", "FIGlet", "pyfiglet",
}
SYMBOL_CHARS = set("|/\\_[]{}()<>")
# Concrete overlap examples documented under the six smushing rules in NOTES.md.
NOTES_RARE_CASES = {
    "||", "//", "_|", "/_", "|/", "[{", "(<",
    "[]", "][", "{}", "}{", "()", ")(",
    "/\\", "\\/", "><", "$$",
}
LATIN1_REQUIRED_CHARS = set("éßñ")
LATIN1_SOURCE_EXAMPLES = {"café", "é", "ß", "ñ"}
UNSUPPORTED_TEXTS = {"漢"}
FLAG_FREQUENCY_MIN = 0.20
FLAG_FREQUENCY_MAX = 0.30
FONT_HEIGHT_CACHE = {}


class CandidateRejected(Exception):
    def __init__(self, args, category, reason):
        self.cli_args = args
        self.category = category
        self.reason = reason
        super().__init__(reason)


def run_cli(args):
    """Run the original CLI in an isolated process and return its exact stdout."""
    env = os.environ.copy()
    existing_pythonpath = env.get("PYTHONPATH")
    env["PYTHONPATH"] = str(PYFIGLET_PATH) + (
        os.pathsep + existing_pythonpath if existing_pythonpath else ""
    )
    command = [sys.executable, "-m", "pyfiglet"] + args

    try:
        completed = subprocess.run(
            command,
            cwd=str(PYFIGLET_PATH),
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=CLI_TIMEOUT_SECONDS,
            check=False,
        )
    except subprocess.TimeoutExpired as exc:
        stdout = exc.stdout or b""
        stderr = exc.stderr or b""
        if isinstance(stdout, bytes):
            stdout = stdout.decode("utf-8", errors="replace")
        if isinstance(stderr, bytes):
            stderr = stderr.decode("utf-8", errors="replace")
        raise CandidateRejected(
            args,
            "timeout",
            f"timed out after {CLI_TIMEOUT_SECONDS}s; "
            f"stdout={stdout!r}; stderr={stderr!r}",
        ) from exc

    if completed.returncode != 0:
        stderr = completed.stderr.decode("utf-8", errors="replace")
        stdout = completed.stdout.decode("utf-8", errors="replace")
        raise CandidateRejected(
            args,
            f"exit code {completed.returncode}",
            f"pyfiglet CLI exited {completed.returncode}; "
            f"stdout={stdout!r}; stderr={stderr!r}"
        )

    try:
        return completed.stdout.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise RuntimeError(
            f"pyfiglet CLI emitted non-UTF-8 stdout for args {args!r}"
        ) from exc


def validate_source_texts():
    source_chars = set("".join(TEXTS))
    missing = set()
    missing.update(NOTES_RARE_CASES - set(TEXTS))
    missing.update(LATIN1_SOURCE_EXAMPLES - set(TEXTS))
    missing.update(LATIN1_REQUIRED_CHARS - source_chars)
    missing.update(SYMBOL_CHARS - source_chars)
    missing.update(UNSUPPORTED_TEXTS - set(TEXTS))
    if missing:
        raise RuntimeError(
            f"TEXTS is missing required source characters/cases: {sorted(missing)!r}"
        )


def candidate_pool():
    validate_source_texts()
    non_wrapping_texts = set(TEXTS) - WRAPPING_TEXTS
    minimum_unique_candidates = (
        len(non_wrapping_texts)
        * len(FONTS)
        * (len(WIDTHS) - 1)
        * len(JUSTIFICATIONS)
        * len(DIRECTIONS)
    )
    if minimum_unique_candidates < MAX_ATTEMPTS_PER_DATASET:
        raise RuntimeError(
            f"TEXTS provide only {minimum_unique_candidates} structurally unique "
            f"candidates; at least {MAX_ATTEMPTS_PER_DATASET} are required"
        )

    candidates = []
    candidate_keys = set()
    for text in TEXTS:
        for font in FONTS:
            for width in WIDTHS:
                for justify in JUSTIFICATIONS:
                    for direction in DIRECTIONS:
                        candidate_font = font
                        candidate_width = width
                        candidate_text = text
                        if text in WRAPPING_TEXTS:
                            candidate_width = 20
                        elif width == 10:
                            candidate_font = "standard"
                            candidate_text = "A"

                        args = [
                            "-f", candidate_font,
                            "-w", str(candidate_width),
                            "-j", justify,
                            "-D", direction,
                        ]
                        args.extend(
                            flag for flag in FLAGS if random.random() < 0.25
                        )
                        args.append(candidate_text)
                        args_key = tuple(args)
                        if args_key in candidate_keys:
                            continue
                        candidate_keys.add(args_key)
                        candidates.append(args)

    random.shuffle(candidates)
    if len(candidates) < MAX_ATTEMPTS_PER_DATASET:
        raise RuntimeError(
            f"Candidate pool has {len(candidates)} unique argument combinations; "
            f"at least {MAX_ATTEMPTS_PER_DATASET} are required for bounded retries"
        )
    return candidates


def font_height(font):
    if font not in FONT_HEIGHT_CACHE:
        font_paths = [
            PYFIGLET_PATH / "pyfiglet" / "fonts" / f"{font}.flf",
            PYFIGLET_PATH / "pyfiglet" / "fonts-standard" / f"{font}.flf",
        ]
        for font_path in font_paths:
            if font_path.is_file():
                with font_path.open(encoding="ascii") as font_file:
                    header = font_file.readline().split()
                if len(header) < 2:
                    raise RuntimeError(f"Invalid FIGlet font header: {font_path}")
                try:
                    FONT_HEIGHT_CACHE[font] = int(header[1])
                except ValueError as exc:
                    raise RuntimeError(
                        f"Invalid height in FIGlet font header: {font_path}"
                    ) from exc
                break
        else:
            raise RuntimeError(f"Cannot find FIGlet font file for {font!r}")
    return FONT_HEIGHT_CACHE[font]


def is_wrapped_output(args, output):
    if args[-1] not in LONG_SENTENCES or int(option_value(args, "-w")) != 20:
        return False
    height = font_height(option_value(args, "-f"))
    # Account for main()'s final newline and optional -n padding.
    non_wrapped_line_limit = height + (
        2 if "-n" in args and "-s" not in args else 1
    )
    return len(output.splitlines()) > non_wrapped_line_limit


def potential_case_coverage(args):
    text = args[-1]
    tokens = {
        ("font", option_value(args, "-f")),
        ("width", int(option_value(args, "-w"))),
        ("justification", option_value(args, "-j")),
        ("direction", option_value(args, "-D")),
    }
    if text in NORMAL_WORDS:
        tokens.add(("normal word",))
    if text in NOTES_RARE_CASES:
        tokens.add(("rare case", text))
    tokens.update(("symbol", char) for char in text if char in SYMBOL_CHARS)
    if "  " in text:
        tokens.add(("double spaces",))
    if text in LONG_SENTENCES and int(option_value(args, "-w")) == 20:
        tokens.add(("long wrapping sentence",))
    tokens.update(
        ("Latin-1", char) for char in text if char in LATIN1_REQUIRED_CHARS
    )
    if text in UNSUPPORTED_TEXTS:
        tokens.add(("unsupported character",))
    return tokens


def actual_case_coverage(args, output):
    tokens = potential_case_coverage(args)
    if args[-1] in UNSUPPORTED_TEXTS and output.strip():
        tokens.discard(("unsupported character",))
    if ("long wrapping sentence",) in tokens and not is_wrapped_output(args, output):
        tokens.discard(("long wrapping sentence",))
    return tokens


def required_case_coverage():
    tokens = {
        ("font", font) for font in FONTS
    } | {
        ("width", width) for width in WIDTHS
    } | {
        ("justification", justify) for justify in JUSTIFICATIONS
    } | {
        ("direction", direction) for direction in DIRECTIONS
    }
    tokens.update(("rare case", text) for text in NOTES_RARE_CASES)
    tokens.update(("symbol", char) for char in SYMBOL_CHARS)
    tokens.update(("Latin-1", char) for char in LATIN1_REQUIRED_CHARS)
    tokens.update({
        ("normal word",),
        ("double spaces",),
        ("long wrapping sentence",),
        ("unsupported character",),
    })
    return tokens


def generate_dataset(label, candidates, used_args, rejected_args):
    cases = []
    failures = []
    repeated_rejections = 0
    duplicate_candidates = 0
    attempts = 0
    covered = set()
    required_coverage = required_case_coverage()
    remaining_candidates = list(candidates)
    while (
        len(cases) < TARGET_CASES
        and attempts < MAX_ATTEMPTS_PER_DATASET
        and remaining_candidates
    ):
        missing_coverage = required_coverage - covered
        candidate_index = 0
        if missing_coverage:
            candidate_index = next(
                (
                    index for index, candidate in enumerate(remaining_candidates)
                    if potential_case_coverage(candidate) & missing_coverage
                ),
                None,
            )
            if candidate_index is None:
                break
        args = remaining_candidates.pop(candidate_index)
        attempts += 1
        args_key = tuple(args)
        if args_key in used_args:
            duplicate_candidates += 1
            print(
                f"  [{label}] skipped previously used args: "
                f"{json.dumps(args, ensure_ascii=False)}",
                flush=True,
            )
            continue
        if args_key in rejected_args:
            repeated_rejections += 1
            print(
                f"  [{label}] skipped repeated rejected args: "
                f"{json.dumps(args, ensure_ascii=False)}; "
                f"original reason={rejected_args[args_key]}",
                flush=True,
            )
            continue

        try:
            output = run_cli(args)
        except CandidateRejected as exc:
            failure = {
                "args": exc.cli_args,
                "category": exc.category,
                "reason": exc.reason,
            }
            failures.append(failure)
            rejected_args[args_key] = exc.reason
            print(
                f"  [{label}] rejected candidate #{attempts}: "
                f"args={json.dumps(exc.cli_args, ensure_ascii=False)}; "
                f"reason={exc.reason}",
                flush=True,
            )
            continue

        used_args.add(args_key)
        cases.append({"args": args, "out": output})
        covered.update(actual_case_coverage(args, output))
        done = len(cases)
        if done % 200 == 0:
            print(f"  [{label}] {done}/{TARGET_CASES}", flush=True)

    if len(cases) != TARGET_CASES:
        failure_reasons = Counter(failure["category"] for failure in failures)
        last_failure = failures[-1] if failures else None
        if attempts >= MAX_ATTEMPTS_PER_DATASET:
            stop_reason = "attempt limit reached"
        elif not remaining_candidates:
            stop_reason = f"candidate pool exhausted at {len(candidates)} combinations"
        elif required_coverage - covered:
            stop_reason = "no remaining candidate can satisfy required coverage"
        else:
            stop_reason = "candidate pool exhausted"
        missing_summary = sorted(required_coverage - covered, key=repr)
        last_summary = (
            f"; last rejected args={json.dumps(last_failure['args'], ensure_ascii=False)}"
            f"; reason={last_failure['reason']}"
            if last_failure else ""
        )
        raise RuntimeError(
            f"{label} dataset stopped after {attempts}/{MAX_ATTEMPTS_PER_DATASET} "
            f"attempts ({stop_reason}) with {len(cases)}/{TARGET_CASES} successful cases and "
            f"{len(failures)} rejected candidates; failure reasons="
            f"{dict(failure_reasons)}; {repeated_rejections} repeated rejected "
            f"candidates suppressed; {duplicate_candidates} previously used "
            f"candidates skipped after {attempts} attempts; "
            f"missing coverage={missing_summary!r}{last_summary}. "
            "Each rejection was logged above."
        )

    print(
        f"  [{label}] Done — {len(cases)} successful cases, "
        f"{len(failures)} rejected and {repeated_rejections} repeated rejected "
        f"candidates suppressed, {duplicate_candidates} used candidates skipped "
        f"in {attempts} attempts.",
        flush=True,
    )
    return cases, failures


def option_value(args, option):
    return args[args.index(option) + 1]


def report_dataset(label, cases, failures):
    coverage = {
        "font": Counter(option_value(case["args"], "-f") for case in cases),
        "width": Counter(int(option_value(case["args"], "-w")) for case in cases),
        "justification": Counter(option_value(case["args"], "-j") for case in cases),
        "direction": Counter(option_value(case["args"], "-D") for case in cases),
    }
    flag_counts = Counter(
        flag for case in cases for flag in FLAGS if flag in case["args"]
    )
    argument_counts = Counter(tuple(case["args"]) for case in cases)
    duplicate_count = len(cases) - len(argument_counts)

    failure_reasons = Counter(failure["category"] for failure in failures)
    print(
        f"{label} successful cases: {len(cases)}/{TARGET_CASES}; "
        f"rejected candidates: {len(failures)}; "
        f"failure reasons: {dict(failure_reasons) if failures else 'none'}",
        flush=True,
    )
    print(f"{label} parameter coverage:", flush=True)
    for parameter, counts in coverage.items():
        print(f"  {parameter}: {dict(counts)}", flush=True)
    print(
        "  flag frequencies: "
        + ", ".join(
            f"{flag}={flag_counts[flag]}/{len(cases)}"
            for flag in FLAGS
        ),
        flush=True,
    )
    min_flag_count = int(TARGET_CASES * FLAG_FREQUENCY_MIN)
    max_flag_count = int(TARGET_CASES * FLAG_FREQUENCY_MAX)
    for flag in FLAGS:
        if not min_flag_count <= flag_counts[flag] <= max_flag_count:
            raise RuntimeError(
                f"{label} flag {flag} appeared {flag_counts[flag]}/{len(cases)} "
                f"times; expected approximately 25% "
                f"({min_flag_count}-{max_flag_count})"
            )
    print(
        f"  unique argument combinations: {len(argument_counts)}/{len(cases)}; "
        f"duplicate cases: {duplicate_count}",
        flush=True,
    )
    if duplicate_count:
        duplicate_examples = [
            (args, count)
            for args, count in argument_counts.items()
            if count > 1
        ][:5]
        for args, count in duplicate_examples:
            print(
                f"    repeated {count} times: "
                f"{json.dumps(list(args), ensure_ascii=False)}",
                flush=True,
            )
        raise RuntimeError(
            f"{label} dataset contains {duplicate_count} duplicate argument "
            "combinations"
        )

    expected_coverage = {
        "font": set(FONTS),
        "width": set(WIDTHS),
        "justification": set(JUSTIFICATIONS),
        "direction": set(DIRECTIONS),
    }
    for parameter, expected in expected_coverage.items():
        if set(coverage[parameter]) != expected:
            raise RuntimeError(
                f"{label} dataset is missing {parameter} coverage: "
                f"{expected - set(coverage[parameter])}"
            )
    if len(cases) != TARGET_CASES:
        raise RuntimeError(
            f"{label} dataset has {len(cases)} cases, expected {TARGET_CASES}"
        )
    actual_texts = {case["args"][-1] for case in cases}
    actual_symbols = {
        char for text in actual_texts for char in text if char in SYMBOL_CHARS
    }
    actual_latin1_chars = {
        char for text in actual_texts for char in text
        if char in LATIN1_REQUIRED_CHARS
    }
    categories = {
        "normal words": bool(actual_texts & NORMAL_WORDS),
        "NOTES.md rare cases": NOTES_RARE_CASES <= actual_texts,
        "symbols": SYMBOL_CHARS <= actual_symbols,
        "double spaces": any("  " in text for text in actual_texts),
        "long wrapping sentences": any(
            is_wrapped_output(case["args"], case["out"])
            for case in cases
        ),
        "Latin-1 characters": LATIN1_REQUIRED_CHARS <= actual_latin1_chars,
        "unsupported characters": any(
            case["args"][-1] in UNSUPPORTED_TEXTS and not case["out"].strip()
            for case in cases
        ),
    }
    missing_categories = [
        category for category, covered in categories.items() if not covered
    ]
    missing_rare_cases = NOTES_RARE_CASES - actual_texts
    if missing_rare_cases:
        raise RuntimeError(
            f"{label} dataset is missing documented NOTES.md rare cases: "
            f"{sorted(missing_rare_cases)!r}"
        )
    if missing_categories:
        raise RuntimeError(
            f"{label} dataset is missing text categories: {missing_categories!r}"
        )
    print(
        f"  text categories covered: {', '.join(categories)}",
        flush=True,
    )
    for case in cases:
        args = case["args"]
        font = option_value(args, "-f")
        width = int(option_value(args, "-w"))
        text = args[-1]
        if text in WRAPPING_TEXTS and width != 20:
            raise RuntimeError(
                f"{label} wrapping text must use width 20: args={args!r}"
            )
        if width == 10 and (font != "standard" or text != "A"):
            raise RuntimeError(
                f"{label} width-10 case is outside the verified safe input: "
                f"args={args!r}"
            )

def write_jsonl(path, cases):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="\n") as output_file:
        for case in cases:
            output_file.write(json.dumps(case, ensure_ascii=False) + "\n")
    print(f"Written {len(cases)} cases to {path}", flush=True)


def main():
    random.seed(7)
    print("=== pyfiglet CLI test generator ===", flush=True)
    print(f"pyfiglet: {PYFIGLET_PATH}", flush=True)
    print(f"output: {OUT_DIR}", flush=True)
    candidates = candidate_pool()
    print(f"unique candidate combinations: {len(candidates)}", flush=True)

    print("Generating visible dataset...", flush=True)
    used_args = set()
    rejected_args = {}
    visible, visible_failures = generate_dataset(
        "visible", candidates, used_args, rejected_args
    )
    report_dataset("visible", visible, visible_failures)
    print("Generating hidden dataset...", flush=True)
    hidden, hidden_failures = generate_dataset(
        "hidden", candidates, used_args, rejected_args
    )
    report_dataset("hidden", hidden, hidden_failures)

    visible_args = {tuple(case["args"]) for case in visible}
    hidden_args = {tuple(case["args"]) for case in hidden}
    overlapping_args = visible_args & hidden_args
    if overlapping_args:
        examples = sorted(overlapping_args)[:5]
        raise RuntimeError(
            f"Visible and hidden datasets share {len(overlapping_args)} argument "
            f"combinations; examples={examples!r}"
        )
    if visible == hidden:
        raise RuntimeError("Visible and hidden datasets are identical")

    write_jsonl(OUT_DIR / "visible.jsonl", visible)
    write_jsonl(OUT_DIR / "hidden.jsonl", hidden)


if __name__ == "__main__":
    main()

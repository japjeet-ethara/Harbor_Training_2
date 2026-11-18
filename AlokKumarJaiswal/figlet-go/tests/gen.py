#!/usr/bin/env python3
"""
Test dataset generator for FIGlet in Go.

Generates tests/data/visible.jsonl and tests/data/hidden.jsonl, each containing
exactly 2000 test cases captured from the pinned original pyfiglet reference CLI.
"""

import hashlib
import json
import os
import pathlib
import random
import subprocess
import sys
from typing import NamedTuple

EXPECTED_COMMIT = "255827efbc09d662875dad50fa0edc1e1263bf45"
TARGET_PER_SET = 2000
MAX_TIMEOUTS = 10
MAX_ATTEMPTS = 12000
MIN_RTL_NARROW = 50

FONTS = [
    "standard",
    "slant",
    "small",
    "big",
    "mini",
    "banner",
    "shadow",
    "script",
]

WIDTHS = [10, 20, 40, 60, 80, 120]

JUSTIFICATIONS = ["auto", "left", "center", "right"]

DIRECTIONS = ["auto", "left-to-right", "right-to-left"]

FLAGS = ["-r", "-F", "-s", "-n"]

# Curated pools of text to ensure healthy variety and comprehensive coverage
NORMAL_WORDS = [
    "hello", "world", "figlet", "test", "Antigravity", "Go", "Python",
    "code", "terminal", "output", "buffer", "matrix", "ascii", "banner",
    "slant", "standard", "shadow", "script", "font", "glyph", "char",
    "line", "space", "wrap", "center", "left", "right", "reverse",
    "flip", "strip", "clean", "simple", "quick", "lazy", "brown",
    "fox", "dog", "jump", "run", "fast", "slow", "bright", "dark",
    "light", "heavy", "small", "large", "mini", "tall", "short",
    "wide", "deep", "open", "close", "start", "stop", "true", "false",
    "value", "stream", "byte", "integer", "string", "slice", "array",
    "struct", "pointer", "channel", "routine", "mutex", "lock", "fiber",
]

ADJECTIVES = [
    "quick", "lazy", "brave", "calm", "sharp", "soft", "bold", "cool",
    "warm", "wild", "pure", "grand", "swift", "quiet", "proud", "amber",
    "azure", "crisp", "dense", "eager", "frost", "giant", "hardy", "ionic",
    "jolly", "keen", "lucid", "noble", "polar", "rusty",
]

NOUNS = [
    "fox", "dog", "hawk", "wolf", "bear", "lion", "tiger", "eagle",
    "otter", "coder", "server", "client", "packet", "socket", "thread",
    "kernel", "daemon", "parser", "buffer", "canvas", "stream", "engine",
    "signal", "vector", "matrix", "module", "router", "worker", "beacon", "cipher",
]

VERBS = [
    "runs", "jumps", "builds", "renders", "parses", "wraps", "smushes",
    "mirrors", "flips", "prints", "loads", "stores", "spawns", "checks",
    "scans", "traces", "routes", "binds", "shifts", "starts",
]

DOUBLE_SPACES = [
    "hello  world",
    "double  space",
    "left  right",
    "foo  bar  baz",
    "two  spaces  here",
    "wide  margin",
    "a  b  c",
    "split  view",
    "spaced   out   words",
    "first  second  third",
    "alpha  beta  gamma",
    "cat   dog",
    "sun  moon  stars",
    "start  middle  end",
    "open  source  project",
    "input  and  output",
    "leading   trailing",
    "center  aligned  text",
    "render  figlet  font",
    "column  layout  test",
]

WRAPPING_SENTENCES = [
    "The quick brown fox jumps over the lazy dog.",
    "Pack my box with five dozen liquor jugs.",
    "Sphinx of black quartz, judge my vow.",
    "How vexingly quick daft zebras jump!",
    "Two driven jocks help fax my big quiz.",
    "Bright vixens jump; dozy fowl quack.",
    "FIGlet produces large text characters out of ordinary screen characters.",
    "A command line utility that renders banners using standard fonts.",
    "Word wrapping occurs when lines exceed the terminal width boundary.",
    "Smushing combines adjacent character borders according to font rules.",
    "Terminal formatting requires handling justification and print direction.",
    "Reversing and flipping lines provides horizontal and vertical reflection.",
    "Surrounding newlines can be stripped or normalized for clean output.",
    "A comprehensive suite of test cases ensures full behavioral compliance.",
    "The Go implementation faithfully matches the original Python reference.",
]

LATIN1_TEXTS = [
    "café",
    "Straße",
    "año",
    "señor",
    "über",
    "naïve",
    "crème brûlée",
    "coöperation",
    "résumé",
    "piñata",
    "façade",
    "fiancée",
    "château",
    "El Niño",
    "Groß",
    "süß",
    "weiß",
    "Gruß",
    "Mañana",
    "Olé",
    "cliché",
    "déjà vu",
    "vis-à-vis",
    "voilà",
    "cœur",
    "jalapeño",
    "smörgåsbord",
    "faustgroß",
    "Fußgänger",
    "Hélène",
]

MISSING_GLYPHS = [
    "hello 🚀 world",
    "test 🌟 figlet",
    "code 💻 Go",
    "fire 🔥 art",
    "alpha α beta β",
    "gamma γ delta δ",
    "pi π and theta θ",
    "sigma ∑ sum",
    "CJK 中文 测试",
    "Japanese 日本語",
    "Korean 한국어",
    "arrow → direction ←",
    "check ✓ and cross ✗",
    "heart ♥ spade ♠",
    "music ♫ score",
    "infinity ∞ symbol",
    "math √ square ± root",
    "figlet 🎨 output",
    "emoji 🎉 celebration",
    "coffee ☕ break",
]

PUNCTUATION_TEXTS = [
    "|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">",
    "SMUSH", "_/", "/\\", "\\/", "><", "<>", "[]", "{}", "()",
    "|_|", "(/)", "[/]", "<_>", "{/}", "|/\\|",
    "/\\_/", "()[]{}", "<>/\\", "_|_", "[[ ]]", "(( ))", "{{ }}",
    "a | b", "x / y", "x \\ y", "(a) + [b] = {c}", "<tag>", "</tag>",
    "_lowline_", "/* comment */", "func(x []int)", "<Tag/>",
    "foo_bar", "{key: value}", "(foo|bar)", "[1, 2, 3]",
    "a/b\\c", "d>e<f", "g[h]i", "j{k}l", "m(n)o",
    "|a|b|c|", "(1+2)*(3-4)", "[a-z0-9]", "<header.h>",
    "{a: [1, 2]}", "x_y_z", "a/b/c", "c:\\test\\path",
]

HYPHEN_TEXTS = [
    "-v", "-h", "-n", "-s", "-r", "-F", "--test", "--flag",
    "-help", "-abc", "--version", "-w", "-f", "-j", "-D",
    "-123", "--output", "-text", "--start", "-x", "-y", "-z",
    "-foo", "--bar", "--width=80", "-all", "-none", "--dry-run",
    "-debug", "--verbose",
]

SHORT_TEXTS_FOR_NARROW = [
    "a", "b", "c", "x", "y", "z", "1", "2", "3", "!", "?", "#", "$", "*",
    "-", "+", "=", "hi", "ok", "go", "up", "in", "no", "to", "on", "it",
    "at", "by", "cat", "dog", "sun", "box", "run", "sky", "art", "top",
    "end", "joy", "42", "99", "A", "Z", "X", "M", "W", "RTL", "LTR",
    "-1", "-x", "-a", "-go", "é", "ß", "ñ", "/", "\\", "|", "_", "<", ">",
    "(", ")", "[", "]", "{", "}", "/\\", "\\/", "><", "<>", "[]", "{}",
    "()", "_/", "|_|", "o_o", "^_^", "!?!", "10", "20",
]


def get_git_head(repo_dir: pathlib.Path) -> str:
    """Determine the current git commit HEAD of repo_dir via git rev-parse HEAD without fallback."""
    try:
        proc = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=str(repo_dir),
            capture_output=True,
            timeout=5,
        )
    except subprocess.TimeoutExpired as e:
        stderr_str = e.stderr.decode("utf-8", errors="replace").strip() if e.stderr else "(none)"
        raise RuntimeError(
            f"git rev-parse HEAD timed out after 5 seconds in {repo_dir}.\nStderr: {stderr_str}"
        ) from e
    except OSError as e:
        raise RuntimeError(f"Failed to execute git rev-parse HEAD in {repo_dir}: {e}") from e

    if proc.returncode != 0:
        stderr_str = proc.stderr.decode("utf-8", errors="replace").strip() if proc.stderr else "(none)"
        raise RuntimeError(
            f"git rev-parse HEAD exited with code {proc.returncode} in {repo_dir}.\nStderr: {stderr_str}"
        )

    commit = proc.stdout.decode("utf-8").strip()
    if not commit:
        stderr_str = proc.stderr.decode("utf-8", errors="replace").strip() if proc.stderr else "(none)"
        raise RuntimeError(
            f"git rev-parse HEAD returned empty output in {repo_dir}.\nStderr: {stderr_str}"
        )

    return commit


def verify_reference_environment(clone_root: pathlib.Path, child_env: dict[str, str]) -> pathlib.Path:
    """Verify git HEAD and child process import path for reference pyfiglet."""
    if not clone_root.is_dir():
        print(f"ERROR: pyfiglet clone not found at {clone_root}", file=sys.stderr)
        sys.exit(1)

    try:
        actual_head = get_git_head(clone_root)
        if actual_head != EXPECTED_COMMIT:
            print(
                f"ERROR: git HEAD mismatch: expected {EXPECTED_COMMIT}, got {actual_head}",
                file=sys.stderr,
            )
            sys.exit(1)
    except Exception as e:
        print(f"ERROR: git verification failed: {e}", file=sys.stderr)
        sys.exit(1)

    expected_init = (clone_root / "pyfiglet" / "__init__.py").resolve()
    init_cmd = [
        sys.executable,
        "-B",
        "-c",
        "import pathlib, pyfiglet; print(pathlib.Path(pyfiglet.__file__).resolve())",
    ]

    try:
        proc = subprocess.run(
            init_cmd,
            cwd=str(clone_root),
            env=child_env,
            capture_output=True,
            timeout=5,
        )
    except subprocess.TimeoutExpired as e:
        stderr_str = e.stderr.decode("utf-8", errors="replace") if e.stderr else "(none)"
        print(
            f"ERROR: Initial import check timed out after 5 seconds.\nStderr: {stderr_str}",
            file=sys.stderr,
        )
        sys.exit(1)

    if proc.returncode != 0:
        stderr_str = proc.stderr.decode("utf-8", errors="replace") if proc.stderr else "(none)"
        print(
            f"ERROR: Initial import check exited with code {proc.returncode}.\nStderr: {stderr_str}",
            file=sys.stderr,
        )
        sys.exit(1)

    if proc.stderr:
        stderr_str = proc.stderr.decode("utf-8", errors="replace")
        print(
            f"ERROR: Initial import check had non-empty stderr.\nStderr: {stderr_str}",
            file=sys.stderr,
        )
        sys.exit(1)

    try:
        raw_path = proc.stdout.decode("utf-8", errors="strict").strip()
        resolved_init = pathlib.Path(raw_path).resolve()
        if resolved_init != expected_init:
            print(
                f"ERROR: Imported pyfiglet path {resolved_init} does not match expected {expected_init}",
                file=sys.stderr,
            )
            sys.exit(1)
    except Exception as e:
        print(f"ERROR: Could not verify pyfiglet import path: {e}", file=sys.stderr)
        sys.exit(1)

    return resolved_init


def load_font_widths(clone_root: pathlib.Path, child_env: dict[str, str]) -> dict[str, dict[int, int]]:
    """
    In one verified child process, use the original pyfiglet loader to obtain
    actual stored glyph widths for all eight allowed fonts, returned as JSON.
    """
    code = (
        "import json, sys, pyfiglet\n"
        f"fonts = {FONTS!r}\n"
        "widths = {f: pyfiglet.FigletFont(f).width for f in fonts}\n"
        "sys.stdout.write(json.dumps(widths))\n"
    )
    cmd = [sys.executable, "-B", "-c", code]
    try:
        proc = subprocess.run(
            cmd,
            cwd=str(clone_root),
            env=child_env,
            capture_output=True,
            timeout=5,
        )
    except subprocess.TimeoutExpired as e:
        stderr_str = e.stderr.decode("utf-8", errors="replace") if e.stderr else "(none)"
        print(
            f"ERROR: Font width inspection timed out after 5 seconds.\nStderr: {stderr_str}",
            file=sys.stderr,
        )
        sys.exit(1)

    if proc.returncode != 0:
        stderr_str = proc.stderr.decode("utf-8", errors="replace") if proc.stderr else "(none)"
        print(
            f"ERROR: Font width inspection exited with code {proc.returncode}.\nStderr: {stderr_str}",
            file=sys.stderr,
        )
        sys.exit(1)

    if proc.stderr:
        stderr_str = proc.stderr.decode("utf-8", errors="replace")
        print(
            f"ERROR: Font width inspection had non-empty stderr.\nStderr: {stderr_str}",
            file=sys.stderr,
        )
        sys.exit(1)

    try:
        raw = json.loads(proc.stdout.decode("utf-8", errors="strict"))
        font_widths: dict[str, dict[int, int]] = {}
        for f in FONTS:
            if f not in raw:
                raise ValueError(f"Missing font {f!r} in font width inspection output")
            font_widths[f] = {int(k): v for k, v in raw[f].items()}
        return font_widths
    except Exception as e:
        print(f"ERROR: Could not decode font widths JSON: {e}", file=sys.stderr)
        sys.exit(1)


def run_cli_invocation(
    clone_root: pathlib.Path,
    child_env: dict[str, str],
    cli_args: list[str],
) -> tuple[bool, str, str]:
    """
    Executes sys.executable -B -m pyfiglet <cli_args> in clone_root.
    Returns (success, stdout_str, failure_reason).
    """
    cmd = [sys.executable, "-B", "-m", "pyfiglet"] + cli_args
    try:
        proc = subprocess.run(
            cmd,
            cwd=str(clone_root),
            env=child_env,
            capture_output=True,
            timeout=5,
            shell=False,
        )
    except subprocess.TimeoutExpired:
        return False, "", "timeout"

    if proc.returncode != 0:
        stderr_sample = proc.stderr.decode("utf-8", errors="replace").strip()[:120]
        return False, "", f"nonzero exit {proc.returncode}: {stderr_sample}"

    if proc.stderr != b"":
        stderr_sample = proc.stderr.decode("utf-8", errors="replace").strip()[:120]
        return False, "", f"non-empty stderr: {stderr_sample}"

    try:
        decoded_stdout = proc.stdout.decode("utf-8", errors="strict")
        if decoded_stdout.encode("utf-8") != proc.stdout:
            return False, "", "utf-8 round-trip mismatch"
    except Exception as e:
        return False, "", f"utf-8 decode error: {e}"

    return True, decoded_stdout, ""


def generate_candidate(rng: random.Random) -> list[str]:
    """
    Generates a candidate CLI argument list using stable pseudo-random choices.
    Candidate generation relies on font and text pools; it does not measure glyph widths.
    Inserts '--' before the final text argument.
    """
    category = rng.randint(0, 9)

    font = None
    width = None
    justify = None
    direction = None
    text = ""

    if category == 0:
        # Category 0: Narrow width (10 or 20) with right-to-left
        width = rng.choice([10, 20])
        direction = "right-to-left"
        if width == 10:
            font = rng.choice(["mini", "small", "standard", "slant", "shadow", "script"])
        else:
            font = rng.choice(FONTS)
        justify = rng.choice(JUSTIFICATIONS) if rng.random() > 0.1 else None
        text = rng.choice(SHORT_TEXTS_FOR_NARROW)
    elif category == 1:
        # Category 1: Wrapping sentences
        width = rng.choice([20, 40, 60])
        font = rng.choice(FONTS) if rng.random() > 0.1 else None
        justify = rng.choice(JUSTIFICATIONS) if rng.random() > 0.1 else None
        direction = rng.choice(DIRECTIONS) if rng.random() > 0.1 else None
        text = rng.choice(WRAPPING_SENTENCES)
    elif category == 2:
        # Category 2: Double spaces
        width = rng.choice([40, 60, 80, 120]) if rng.random() > 0.1 else None
        font = rng.choice(FONTS) if rng.random() > 0.1 else None
        justify = rng.choice(JUSTIFICATIONS) if rng.random() > 0.1 else None
        direction = rng.choice(DIRECTIONS) if rng.random() > 0.1 else None
        text = rng.choice(DOUBLE_SPACES)
    elif category == 3:
        # Category 3: Latin-1 characters
        width = rng.choice([40, 60, 80, 120]) if rng.random() > 0.1 else None
        font = rng.choice(FONTS) if rng.random() > 0.1 else None
        justify = rng.choice(JUSTIFICATIONS) if rng.random() > 0.1 else None
        direction = rng.choice(DIRECTIONS) if rng.random() > 0.1 else None
        text = rng.choice(LATIN1_TEXTS)
    elif category == 4:
        # Category 4: Missing glyphs
        width = rng.choice([40, 60, 80, 120]) if rng.random() > 0.1 else None
        font = rng.choice(FONTS) if rng.random() > 0.1 else None
        justify = rng.choice(JUSTIFICATIONS) if rng.random() > 0.1 else None
        direction = rng.choice(DIRECTIONS) if rng.random() > 0.1 else None
        text = rng.choice(MISSING_GLYPHS)
    elif category == 5:
        # Category 5: Punctuation and smushing combinations
        width = rng.choice([20, 40, 60, 80, 120]) if rng.random() > 0.1 else None
        font = rng.choice(FONTS) if rng.random() > 0.1 else None
        justify = rng.choice(JUSTIFICATIONS) if rng.random() > 0.1 else None
        direction = rng.choice(DIRECTIONS) if rng.random() > 0.1 else None
        text = rng.choice(PUNCTUATION_TEXTS)
    elif category == 6:
        # Category 6: Leading hyphens (option boundary verification)
        width = rng.choice([40, 60, 80, 120]) if rng.random() > 0.1 else None
        font = rng.choice(FONTS) if rng.random() > 0.1 else None
        justify = rng.choice(JUSTIFICATIONS) if rng.random() > 0.1 else None
        direction = rng.choice(DIRECTIONS) if rng.random() > 0.1 else None
        text = rng.choice(HYPHEN_TEXTS)
    elif category == 7:
        # Category 7: Normal words
        width = rng.choice([20, 40, 60, 80, 120]) if rng.random() > 0.1 else None
        font = rng.choice(FONTS) if rng.random() > 0.1 else None
        justify = rng.choice(JUSTIFICATIONS) if rng.random() > 0.1 else None
        direction = rng.choice(DIRECTIONS) if rng.random() > 0.1 else None
        text = rng.choice(NORMAL_WORDS)
    elif category == 8:
        # Category 8: Generated multi-word phrases
        width = rng.choice([40, 60, 80, 120]) if rng.random() > 0.1 else None
        font = rng.choice(FONTS) if rng.random() > 0.1 else None
        justify = rng.choice(JUSTIFICATIONS) if rng.random() > 0.1 else None
        direction = rng.choice(DIRECTIONS) if rng.random() > 0.1 else None
        phrase_type = rng.randint(0, 2)
        if phrase_type == 0:
            text = f"{rng.choice(ADJECTIVES)} {rng.choice(NOUNS)}"
        elif phrase_type == 1:
            text = f"{rng.choice(VERBS)} {rng.choice(NOUNS)}"
        else:
            text = f"{rng.choice(ADJECTIVES)} {rng.choice(NOUNS)} {rng.choice(VERBS)}"
    else:
        # Category 9: General diverse mix
        width = rng.choice(WIDTHS) if rng.random() > 0.1 else None
        if width == 10:
            font = rng.choice(["mini", "small", "standard", "slant", "shadow", "script"])
            text = rng.choice(SHORT_TEXTS_FOR_NARROW)
        else:
            font = rng.choice(FONTS) if rng.random() > 0.1 else None
            text = rng.choice(NORMAL_WORDS + PUNCTUATION_TEXTS + SHORT_TEXTS_FOR_NARROW)
        justify = rng.choice(JUSTIFICATIONS) if rng.random() > 0.1 else None
        direction = rng.choice(DIRECTIONS) if rng.random() > 0.1 else None

    # Independently enable each boolean flag with probability 1/4
    flag_r = rng.random() < 0.25
    flag_F = rng.random() < 0.25
    flag_s = rng.random() < 0.25
    flag_n = rng.random() < 0.25

    args: list[str] = []
    if font is not None:
        args.extend(["-f", font])
    if width is not None:
        args.extend(["-w", str(width)])
    if justify is not None:
        args.extend(["-j", justify])
    if direction is not None:
        args.extend(["-D", direction])
    if flag_r:
        args.append("-r")
    if flag_F:
        args.append("-F")
    if flag_s:
        args.append("-s")
    if flag_n:
        args.append("-n")

    args.extend(["--", text])
    return args


def get_effective_options(args: list[str]) -> dict:
    """Parse CLI args and extract effective options including default values and text."""
    opts = {
        "font": "standard",
        "width": 80,
        "justify": "auto",
        "direction": "auto",
        "-r": False,
        "-F": False,
        "-s": False,
        "-n": False,
        "text": "",
    }
    i = 0
    while i < len(args):
        arg = args[i]
        if arg == "--":
            opts["text"] = " ".join(args[i + 1:])
            break
        elif arg == "-f" and i + 1 < len(args):
            opts["font"] = args[i + 1]
            i += 1
        elif arg == "-w" and i + 1 < len(args):
            opts["width"] = int(args[i + 1])
            i += 1
        elif arg == "-j" and i + 1 < len(args):
            opts["justify"] = args[i + 1]
            i += 1
        elif arg == "-D" and i + 1 < len(args):
            opts["direction"] = args[i + 1]
            i += 1
        elif arg in ("-r", "-F", "-s", "-n"):
            opts[arg] = True
        elif not arg.startswith("-"):
            opts["text"] = " ".join(args[i:])
            break
        i += 1
    return opts


class EquivalenceKey(NamedTuple):
    font: str
    width: int
    direction: str
    justify: str
    reverse: bool
    flip: bool
    newline_mode: str
    text: str


def compute_equivalence_key(args_or_opts: list[str] | dict) -> EquivalenceKey:
    """
    Constructs an EquivalenceKey representing the effective rendering behavior
    of the given CLI argument list or parsed options dictionary.

    Equivalence semantics:
    - font: font name
    - width: column width (int)
    - direction: 'auto' resolves to 'left-to-right' for the eight permitted fonts
    - justify: 'auto' follows resolved direction ('left' for 'left-to-right', 'right' for 'right-to-left')
    - reverse: boolean flag (-r)
    - flip: boolean flag (-F)
    - newline_mode: '-s' ('strip') takes precedence over '-n' ('normalize'), else 'neither'
    - text: exact text argument without normalization
    """
    if isinstance(args_or_opts, dict):
        opts = args_or_opts
    else:
        opts = get_effective_options(args_or_opts)

    font = opts["font"]
    width = opts["width"]

    # For the eight permitted fonts, auto direction resolves to left-to-right
    if opts["direction"] == "auto":
        resolved_direction = "left-to-right"
    else:
        resolved_direction = opts["direction"]

    # Auto justification follows resolved direction
    if opts["justify"] == "auto":
        if resolved_direction == "right-to-left":
            resolved_justify = "right"
        else:
            resolved_justify = "left"
    else:
        resolved_justify = opts["justify"]

    reverse = opts["-r"]
    flip = opts["-F"]

    # -s takes precedence over -n
    if opts["-s"]:
        newline_mode = "strip"
    elif opts["-n"]:
        newline_mode = "normalize"
    else:
        newline_mode = "neither"

    text = opts["text"]

    return EquivalenceKey(
        font=font,
        width=width,
        direction=resolved_direction,
        justify=resolved_justify,
        reverse=reverse,
        flip=flip,
        newline_mode=newline_mode,
        text=text,
    )


def is_unsafe_width(
    font_widths: dict[str, dict[int, int]],
    font: str,
    width: int,
    text: str,
) -> bool:
    """
    Check if candidate text contains any supported glyph whose stored width
    is greater than or equal to the selected width.
    Unsupported characters are skipped according to the original loader's behavior.
    """
    glyphs = font_widths.get(font, {})
    for ch in text:
        code = ord(ch)
        if code in glyphs:
            if glyphs[code] >= width:
                return True
    return False


def compute_and_verify_coverage(cases: list[dict], set_name: str) -> dict:
    """
    Computes coverage statistics for a dataset, asserts all required coverage,
    and returns the statistics dictionary.
    """
    font_counts = {f: 0 for f in FONTS}
    width_counts = {w: 0 for w in WIDTHS}
    justify_counts = {j: 0 for j in JUSTIFICATIONS}
    direction_counts = {d: 0 for d in DIRECTIONS}
    flag_counts = {fl: 0 for fl in FLAGS}
    both_sn_count = 0
    rtl_narrow_count = 0

    for case in cases:
        opts = get_effective_options(case["args"])
        font_counts[opts["font"]] = font_counts.get(opts["font"], 0) + 1
        width_counts[opts["width"]] = width_counts.get(opts["width"], 0) + 1
        justify_counts[opts["justify"]] = justify_counts.get(opts["justify"], 0) + 1
        direction_counts[opts["direction"]] = direction_counts.get(opts["direction"], 0) + 1
        for fl in FLAGS:
            if opts[fl]:
                flag_counts[fl] += 1
        if opts["-s"] and opts["-n"]:
            both_sn_count += 1
        if opts["direction"] == "right-to-left" and opts["width"] in (10, 20):
            rtl_narrow_count += 1

    # Assertions
    for f in FONTS:
        assert font_counts.get(f, 0) > 0, f"Font {f!r} was not exercised in {set_name}"
    for w in WIDTHS:
        assert width_counts.get(w, 0) > 0, f"Width {w} was not exercised in {set_name}"
    for j in JUSTIFICATIONS:
        assert justify_counts.get(j, 0) > 0, f"Justification {j!r} was not exercised in {set_name}"
    for d in DIRECTIONS:
        assert direction_counts.get(d, 0) > 0, f"Direction {d!r} was not exercised in {set_name}"
    for fl in FLAGS:
        assert flag_counts.get(fl, 0) > 0, f"Flag {fl!r} was not exercised in {set_name}"
    assert both_sn_count > 0, f"Both -s and -n together were not exercised in {set_name}"
    assert rtl_narrow_count >= MIN_RTL_NARROW, (
        f"Expected at least {MIN_RTL_NARROW} cases combining right-to-left and width 10 or 20 in {set_name}, "
        f"got {rtl_narrow_count}"
    )

    return {
        "font_counts": font_counts,
        "width_counts": width_counts,
        "justify_counts": justify_counts,
        "direction_counts": direction_counts,
        "flag_counts": flag_counts,
        "both_sn_count": both_sn_count,
        "rtl_narrow_count": rtl_narrow_count,
    }


def write_jsonl(path: pathlib.Path, cases: list[dict]) -> None:
    """
    Writes cases to path as UTF-8 without BOM, with physical LF line endings,
    one JSON object per line. Writes via temporary file to ensure atomic replacement
    of an individual file.

    Note: Each file is replaced atomically in sequence, not as a two-file transaction.
    A crash between replacements would leave one file updated and the other untouched,
    rather than preserving the previous pair.
    """
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp_path = path.with_suffix(".jsonl.tmp")
    with open(tmp_path, "w", encoding="utf-8", newline="\n") as f:
        for case in cases:
            line = json.dumps({"args": case["args"], "out": case["out"]}, ensure_ascii=False)
            f.write(line + "\n")
    os.replace(tmp_path, path)


def compute_sha256(path: pathlib.Path) -> str:
    """Computes the SHA-256 hex digest of a file."""
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest()


def print_coverage_report(set_name: str, stats: dict) -> None:
    """Prints a clear coverage breakdown for a dataset."""
    print(f"Coverage Report - {set_name}:")
    print("  Fonts (effective):")
    for f in FONTS:
        print(f"    {f:10s}: {stats['font_counts'].get(f, 0):4d}")
    print("  Widths (effective):")
    for w in WIDTHS:
        print(f"    {w:10d}: {stats['width_counts'].get(w, 0):4d}")
    print("  Justifications (effective):")
    for j in JUSTIFICATIONS:
        print(f"    {j:10s}: {stats['justify_counts'].get(j, 0):4d}")
    print("  Directions (effective):")
    for d in DIRECTIONS:
        print(f"    {d:15s}: {stats['direction_counts'].get(d, 0):4d}")
    print("  Boolean Flags:")
    for fl in FLAGS:
        print(f"    {fl:10s}: {stats['flag_counts'].get(fl, 0):4d}")
    print(f"    both -s/-n: {stats['both_sn_count']:4d}")
    print(f"  Right-to-left with width 10 or 20: {stats['rtl_narrow_count']:4d}")


def main() -> None:
    # 1. Anchor paths to __file__
    tests_dir = pathlib.Path(__file__).resolve().parent
    figlet_go_dir = tests_dir.parent
    clone_root = (figlet_go_dir.parent / "pyfiglet").resolve()
    data_dir = tests_dir / "data"
    visible_path = data_dir / "visible.jsonl"
    hidden_path = data_dir / "hidden.jsonl"

    # 2. Child environment
    child_env = os.environ.copy()
    child_env["PYTHONDONTWRITEBYTECODE"] = "1"
    child_env["PYTHONUTF8"] = "1"

    # 3. Source and git verification
    resolved_init = verify_reference_environment(clone_root, child_env)
    font_widths = load_font_widths(clone_root, child_env)

    # 4. Initialize random seed and candidate state
    random.seed(7)
    rng = random.Random(7)

    visible_cases: list[dict] = []
    hidden_cases: list[dict] = []
    seen_args: set[tuple[str, ...]] = set()
    visible_keys: set[EquivalenceKey] = set()
    hidden_keys: set[EquivalenceKey] = set()

    total_attempts = 0
    rejected_timeout = 0
    rejected_nonzero = 0
    rejected_stderr = 0
    rejected_duplicate = 0
    unsafe_width = 0
    rejected_equivalent_visible = 0
    rejected_equivalent_hidden = 0

    # 5. Candidate generation loop
    while len(visible_cases) < TARGET_PER_SET or len(hidden_cases) < TARGET_PER_SET:
        total_attempts += 1
        if total_attempts > MAX_ATTEMPTS:
            print(
                f"ERROR: Aborting: reached maximum attempts ({MAX_ATTEMPTS}) "
                f"with visible={len(visible_cases)}, hidden={len(hidden_cases)}",
                file=sys.stderr,
            )
            sys.exit(1)

        cli_args = generate_candidate(rng)
        args_tuple = tuple(cli_args)

        if args_tuple in seen_args:
            rejected_duplicate += 1
            print(f"[REJECT: duplicate] attempt={total_attempts} args={' '.join(cli_args)}")
            continue

        opts = get_effective_options(cli_args)
        if is_unsafe_width(font_widths, opts["font"], opts["width"], opts["text"]):
            unsafe_width += 1
            print(f"[REJECT: unsafe_width] attempt={total_attempts} args={' '.join(cli_args)}")
            continue

        cand_key = None
        if len(visible_cases) >= TARGET_PER_SET:
            cand_key = compute_equivalence_key(opts)
            if cand_key in visible_keys:
                rejected_equivalent_visible += 1
                print(f"[REJECT: equivalent_visible] attempt={total_attempts} args={' '.join(cli_args)}")
                continue
            if cand_key in hidden_keys:
                rejected_equivalent_hidden += 1
                print(f"[REJECT: equivalent_hidden] attempt={total_attempts} args={' '.join(cli_args)}")
                continue

        success, stdout_str, failure_reason = run_cli_invocation(clone_root, child_env, cli_args)

        if not success:
            if failure_reason == "timeout":
                rejected_timeout += 1
                print(f"[REJECT: timeout] attempt={total_attempts} args={' '.join(cli_args)}")
                if rejected_timeout >= MAX_TIMEOUTS:
                    print(
                        f"ERROR: Aborting: reached {MAX_TIMEOUTS} timeouts at attempt {total_attempts}",
                        file=sys.stderr,
                    )
                    sys.exit(1)
            elif failure_reason.startswith("nonzero exit"):
                rejected_nonzero += 1
                print(
                    f"[REJECT: nonzero] attempt={total_attempts} ({failure_reason}) args={' '.join(cli_args)}"
                )
            else:
                rejected_stderr += 1
                print(
                    f"[REJECT: stderr/utf8] attempt={total_attempts} ({failure_reason}) args={' '.join(cli_args)}"
                )
            continue

        # Accepted candidate
        seen_args.add(args_tuple)
        case_data = {"args": cli_args, "out": stdout_str}

        if len(visible_cases) < TARGET_PER_SET:
            visible_cases.append(case_data)
            visible_keys.add(compute_equivalence_key(opts))
        else:
            hidden_cases.append(case_data)
            hidden_keys.add(cand_key)

    # 6. Validate and assert coverage on complete datasets before writing
    assert len(visible_cases) == TARGET_PER_SET, (
        f"Expected {TARGET_PER_SET} visible cases, got {len(visible_cases)}"
    )
    assert len(hidden_cases) == TARGET_PER_SET, (
        f"Expected {TARGET_PER_SET} hidden cases, got {len(hidden_cases)}"
    )
    assert len(seen_args) == TARGET_PER_SET * 2, (
        f"Expected {TARGET_PER_SET * 2} unique args across both sets, got {len(seen_args)}"
    )

    # Equivalence assertions across datasets
    visible_keys_set = {compute_equivalence_key(c["args"]) for c in visible_cases}
    hidden_keys_set = {compute_equivalence_key(c["args"]) for c in hidden_cases}

    overlap = visible_keys_set & hidden_keys_set
    assert len(overlap) == 0, (
        f"Equivalence overlap assertion failed: found {len(overlap)} overlapping keys between visible and hidden sets"
    )
    assert len(hidden_keys_set) == TARGET_PER_SET, (
        f"Hidden uniqueness assertion failed: expected {TARGET_PER_SET} unique hidden keys, got {len(hidden_keys_set)}"
    )

    # Assert serialized visible.jsonl retains exact expected hash before replacing output files
    expected_visible_hash = "f38c234706244f4b08e6ea0cde272d0976dd47932130c3c72200f1c0d937790c"
    visible_serialized = "".join(
        json.dumps({"args": c["args"], "out": c["out"]}, ensure_ascii=False) + "\n"
        for c in visible_cases
    ).encode("utf-8")
    actual_visible_hash = hashlib.sha256(visible_serialized).hexdigest()
    assert actual_visible_hash == expected_visible_hash, (
        f"Visible hash assertion failed: expected {expected_visible_hash}, got {actual_visible_hash}"
    )

    visible_stats = compute_and_verify_coverage(visible_cases, "visible.jsonl")
    hidden_stats = compute_and_verify_coverage(hidden_cases, "hidden.jsonl")

    # 7. Write output files (validates both sets before writing; replaces each file atomically in sequence)
    write_jsonl(visible_path, visible_cases)
    write_jsonl(hidden_path, hidden_cases)

    # 8. Compute hashes of written files
    visible_sha256 = compute_sha256(visible_path)
    hidden_sha256 = compute_sha256(hidden_path)
    assert visible_sha256 == expected_visible_hash, (
        f"Written visible.jsonl hash mismatch: expected {expected_visible_hash}, got {visible_sha256}"
    )

    # 9. Print final report
    print("=" * 60)
    print("REFERENCE VERIFICATION:")
    print(f"  Source path:    {resolved_init}")
    print(f"  git HEAD:       {EXPECTED_COMMIT}")
    print(f"  Python version: {sys.version.splitlines()[0]}")
    print("=" * 60)
    print("EXECUTION SUMMARY:")
    print(f"  Total candidate attempts: {total_attempts}")
    print(f"  Accepted visible cases:   {len(visible_cases)}")
    print(f"  Accepted hidden cases:    {len(hidden_cases)}")
    print(f"  Total accepted cases:     {len(visible_cases) + len(hidden_cases)}")
    print(
        f"  Rejections: timeout={rejected_timeout}, nonzero={rejected_nonzero}, "
        f"stderr={rejected_stderr}, duplicate={rejected_duplicate}, unsafe_width={unsafe_width}, "
        f"equivalent_visible={rejected_equivalent_visible}, equivalent_hidden={rejected_equivalent_hidden} "
        f"(total={rejected_timeout + rejected_nonzero + rejected_stderr + rejected_duplicate + unsafe_width + rejected_equivalent_visible + rejected_equivalent_hidden})"
    )
    print("=" * 60)
    print("EQUIVALENCE SUMMARY:")
    print(f"  Visible unique equivalence keys: {len(visible_keys_set)}")
    print(f"  Hidden unique equivalence keys:  {len(hidden_keys_set)}")
    print(f"  Equivalence-key overlap:        {len(overlap)}")
    print(f"  Excluded equivalent visible:     {rejected_equivalent_visible}")
    print(f"  Excluded equivalent hidden:      {rejected_equivalent_hidden}")
    print("=" * 60)
    print_coverage_report("visible.jsonl", visible_stats)
    print("-" * 60)
    print_coverage_report("hidden.jsonl", hidden_stats)
    print("=" * 60)
    print("OUTPUT FILES:")
    print(f"  visible: {visible_path}")
    print(f"           SHA-256: {visible_sha256}")
    print(f"  hidden:  {hidden_path}")
    print(f"           SHA-256: {hidden_sha256}")
    print("=" * 60)


if __name__ == "__main__":
    main()

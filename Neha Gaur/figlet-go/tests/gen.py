#!/usr/bin/env python3
"""
Test generator for the Harbor figlet-go task.
Generates visible and hidden test datasets by invoking the original pyfiglet CLI.
"""

import json
import os
import pathlib
import random
import subprocess
import sys

# Paths
SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
TASK_DIR = SCRIPT_DIR.parent
DATA_DIR = SCRIPT_DIR / "data"
VISIBLE_FILE = DATA_DIR / "visible.jsonl"
HIDDEN_FILE = DATA_DIR / "hidden.jsonl"

# Locate the original pyfiglet repository
pyfiglet_candidates = [
    (TASK_DIR.parent / "pyfiglet").resolve(),
    (TASK_DIR / ".." / "pyfiglet").resolve(),
    pathlib.Path("../pyfiglet").resolve(),
    (SCRIPT_DIR / ".." / ".." / "pyfiglet").resolve(),
]

PYFIGLET_DIR = None
for candidate in pyfiglet_candidates:
    if (candidate / "pyfiglet").is_dir():
        PYFIGLET_DIR = candidate
        break

if PYFIGLET_DIR is None:
    PYFIGLET_DIR = (TASK_DIR.parent / "pyfiglet").resolve()

# Add pyfiglet directory to sys.path to access font metadata
if str(PYFIGLET_DIR) not in sys.path:
    sys.path.insert(0, str(PYFIGLET_DIR))

try:
    from pyfiglet import FigletFont
except ImportError:
    FigletFont = None

# Parameter domains
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

# Per-case rendering timeout (seconds). Prevents individual pathological cases
# from blocking dataset generation.
RENDER_TIMEOUT_SECONDS = 5.0

# Stop searching for a replacement instead of looping forever.
MAX_REPLACEMENT_ATTEMPTS = 80

class RenderTimeoutError(RuntimeError):
    """Raised when a single pyfiglet rendering case exceeds the timeout."""
    pass

# Text generation pools
NORMAL_WORDS = [
    "Hello", "World", "Harbor", "Go", "Python", "Figlet", "CLI", "ASCII",
    "Banner", "Standard", "Buffer", "Render", "Output", "String", "Number",
    "Terminal", "System", "Kernel", "Process", "Thread", "Memory", "Network",
    "Server", "Client", "Protocol", "Digital", "Graphic", "Design", "Package",
    "Module", "Computer", "Science", "Algorithm", "Binary", "Compiler", "Format",
]

# Rare cases documented in NOTES.md:
# - Rule 1 (SM_EQUAL): Equal characters smush (e.g. '|' + '|' -> '|', hardblanks excluded)
# - Rule 2 (SM_LOWLINE): Underscore '_' loses to: | / \ [ ] { } ( ) < >
# - Rule 3 (SM_HIERARCHY): Ranked classes: 1: |, 2: / \, 3: [ ], 4: { }, 5: ( ), 6: < >
# - Rule 4 (SM_PAIR): Opposite bracket pairs: [] -> |, {} -> |, () -> |
# - Rule 5 (SM_BIGX): / + \ -> |, \ + / -> Y, > + < -> X, and <>
# - Rule 6 (SM_HARDBLANK): Hardblanks ($ + $ -> $, hardblank next to any char is not smushed)
# - Kerning (SM_KERN): '##' and '##' slide together to '####' without character replacement
# - Universal overlapping: later character wins in LTR, earlier in RTL; hardblank loses
NOTES_RARE_CASES = [
    # Rule 1: Equal characters (SM_EQUAL)
    "||", "//", "\\\\", "++", "##", "@@", "==", "**", "''", '""',

    # Rule 2: Underscore smushing (SM_LOWLINE)
    "_|", "|_", "_/", "/_", "_\\", "\\_", "_[", "[_", "_]", "]_",
    "_{", "{_", "_}", "}_", "_(", "(_", "_)", ")_", "_<", "<_", "_>", ">_",

    # Rule 3: Hierarchy smushing (SM_HIERARCHY)
    "|/", "/|", "|\\", "\\|",
    "|[", "[|", "|]", "]|", "|{", "{|", "|}", "}|",
    "|(", "(|", "|)", ")|", "|<", "<|", "|>", ">|",
    "/[", "[/", "\\[", "[\\", "/]", "]/", "\\]", "]\\",
    "/{", "{/", "\\{", "{\\", "/}", "}/", "\\}", "}\\",
    "/(", "(/{", "\\(", "(\\", "/)", ")/", "\\)", ")\\",
    "/<", "</", "\\<", "<\\", "/>", ">/", "\\>", ">\\",
    "[{", "{[", "]{", "{]", "[}", "}[", "]}", "}]",
    "[(", "([", "](", "(]", "[)", ")]", "])", ")]",
    "[<", "<[", "]<", "<]", "[>", ">[", "]>", ">]",
    "{(", "({", "}(", "(}", "{)", "){", "})", ")}",
    "{<", "<{", "}<", "<}", "{>", ">{", "}>", ">}",
    "(<", "<(", ")<", "<)", "(>", ">(", ")>", ">)",

    # Rule 4: Opposite pair smushing (SM_PAIR)
    "[]", "][", "{}", "}{", "()", ")(",
    "[][]", "{}{}", "()()", "[{()}]", "([{}])", "][}{)(",

    # Rule 5: Big X smushing (SM_BIGX)
    "/\\", "\\/", "><", "<>",
    "/\\ \\/", ">< <>", "/\\/\\", "\\/\\/", "><><",
    "A /\\ B", "A \\/ B", "A >< B", "X >< Y",

    # Rule 6: Hardblank smushing (SM_HARDBLANK)
    # The font's internal hardblank ('$' in standard.flf) is not the input character '$',
    # but an internal blank character embedded in glyph definitions such as '-', '=', '^', '~',
    # '"', ''', '`', and space ' '. When these glyphs meet, hardblank smushing ($ + $ -> $)
    # and hardblank non-smushing ($ adjacent to other chars blocked) are exercised:
    "--", "---", "----", "==", "====", "^^", "~~~~", "''", '""', "``",
    "-=", "=-", "-=-=-", "^~", "~^", "-^", "^-", "=^=", "~=~", "'-'", '\"=\"',
    "=== BANNER ===", "-test-", "~wave~", "^high^", "'quoted'", '"double"',
    "- - -", "= = =", "~ ~ ~", "^ ^ ^", " - - ",
    "--||--", "==//==", "--__--", "-[]-", "^{}<>",
    # Literal '$' input strings to test ASCII dollar glyph rendering:
    "$$", "$$$", "$$$$", "$|", "|$", "$/", "/$", "$\\", "\\$",
    "$_", "_$", "$[", "]$", "$A", "A$", "$Hello$", "$World$",

    # Kerning (SM_KERN) and universal overlapping
    "##", "####", "#### ####", "## ##",
    "| | |", "/ / /", "\\ \\ \\", "< > < >",
]

SPECIAL_CHARACTERS = [
    "| / \\ _ [ ] { } ( ) < >",
    "|/\\_[]{}()<>",
    "<({[|/\\_]})>",
    "[|] (/\\) {<>}",
    "| | / / \\ \\ _ _ [ ] { } ( ) < >",
    "<><><> /\\/\\/\\ [][][] {{}} ()()",
    "\\_[]_/\\_{}()<>",
    "| _ / \\ [ ] { } ( ) < > |",
]

DOUBLE_SPACE_TEMPLATES = [
    "Hello  World",
    "Foo  Bar  Baz",
    "  leading  double  space",
    "trailing  double  space  ",
    "Double  space  wrapping  test",
    "A  B  C  D  E",
    "One  Two  Three  Four",
    "Special  |/\\_  Double  Space",
    "Sentence  with  multiple  double  spaces  throughout",
    "Harbor  figlet  go  task",
]

LONG_SENTENCES = [
    "The quick brown fox jumps over the lazy dog repeatedly until the line wraps around.",
    "Software engineering requires careful design thorough evaluation and attention to detail across all components.",
    "In computer science an algorithm is a finite sequence of rigorous instructions typically used to solve specific problems.",
    "FIGlet is a computer program that generates text banners in various typefaces composed of smaller ASCII characters.",
    "Go is an open source programming language supported by Google that makes it easy to build simple fast and reliable software.",
    "Pyfiglet is a full port of FIGlet written in Python which takes ASCII text and renders it in artistic ASCII fonts.",
    "Smushing is the process of sliding two letters together until their ink touches and merging characters according to font rules.",
    "Justification controls whether rendered lines are positioned at the left margin centered or aligned to the right column limit.",
    "Writing direction determines whether characters are placed left to right or right to left across the display columns.",
    "Building robust command line interfaces requires handling terminal width constraints edge cases and user options gracefully.",
]

LATIN1_TEXTS = [
    "café straße señor",
    "Grüße aus Köln und München",
    "Über naïve façade und garçon",
    "El niño come jalapeño y piña en el otoño",
    "René Descartes et François Viète",
    "Ångström và København",
    "ÄÖÜ äöü ß é è à ñ ç ô î ê ë ï",
    "Zürich Überlingen Straße 42",
    "Mañana por la tarde con café y azúcar",
    "Curaçao façade naïve Noël",
]

UNSUPPORTED_CHAR_TEXTS = [
    "Hello 世界",
    "Figlet 日本語",
    "中文 测试",
    "Привет мир",
    "Москва Россия",
    "Alpha α Beta β Gamma γ",
    "Ωmega π",
    "Launch 🚀 rocket",
    "Party 🎉 time",
    "Fire 🔥 code",
    "Idea 💡 AI 🤖",
    "Stars ★★★★★",
    "Infinity ∞ and beyond",
    "Math ∑ √ π ± ≠",
    "Suits ♠ ♥ ♦ ♣",
    "Check ✓ and cross ✗",
    "Hello ☺ smile",
    "Currency € 100",
]

_FONT_CACHE = {}


def get_font_obj(font_name: str):
    """Retrieve and cache FigletFont object."""
    if font_name not in _FONT_CACHE:
        if FigletFont is not None:
            _FONT_CACHE[font_name] = FigletFont(font_name)
        else:
            _FONT_CACHE[font_name] = None
    return _FONT_CACHE[font_name]


def text_fits_width(font_name: str, text: str, width: int) -> bool:
    """Check that every glyph is no wider than the column limit.

    Equal-width glyphs stay eligible. A glyph exactly as wide as the limit is
    only unsafe in specific render conditions; those are rejected separately by
    case_would_hang so valid boundary-width cases are kept.
    """
    font_obj = get_font_obj(font_name)
    if font_obj is None:
        return True
    for ch in text:
        w = font_obj.width.get(ord(ch), 0)
        if w > width:
            return False
    return True


def resolved_direction(font_obj, direction: str) -> str:
    """Match Figlet.direction for auto, left-to-right, and right-to-left."""
    if direction != "auto":
        return direction
    if font_obj is not None and font_obj.printDirection == 1:
        return "right-to-left"
    return "left-to-right"


def _case_params(args: list[str]):
    """Return font, width, direction, and text from a pyfiglet argv list."""
    font = "standard"
    width = 80
    direction = "auto"
    i = 0
    while i < len(args) - 1:
        flag = args[i]
        if flag == "-f":
            font = args[i + 1]
            i += 2
        elif flag == "-w":
            width = int(args[i + 1])
            i += 2
        elif flag == "-D":
            direction = args[i + 1]
            i += 2
        elif flag == "-j":
            i += 2
        elif flag in ("-r", "-F", "-s", "-n"):
            i += 1
        else:
            break
    text = args[-1] if args else ""
    return font, width, direction, text


def empty_line_smush(font_obj, ch: str, direction: str) -> int:
    """Smush amount pyfiglet computes when placing ch on an empty line.

    This follows FigletBuilder.smushAmount for an empty buffer. On an empty
    line prevCharWidth is 0, so real characters do not overlap; only spaces do.
    """
    glyph_width = font_obj.width.get(ord(ch), 0)
    # Kerning and smushing both disabled: pyfiglet refuses to slide the glyph.
    if (font_obj.smushMode & (64 | 128)) == 0:
        return 0

    rows = font_obj.chars.get(ord(ch), [])
    max_smush = glyph_width
    rtl = direction == "right-to-left"
    for row in rows:
        if rtl:
            line_left, line_right = row, ""
        else:
            line_left, line_right = "", row

        linebd = len(line_left.rstrip(" ")) - 1
        if linebd < 0:
            linebd = 0
        if linebd < len(line_left):
            ch1 = line_left[linebd]
        else:
            linebd = 0
            ch1 = ""

        charbd = len(line_right) - len(line_right.lstrip(" "))
        amt = charbd + len(line_left) - 1 - linebd
        if ch1 == "" or ch1 == " ":
            amt += 1

        if amt < max_smush:
            max_smush = amt
    return max_smush


def glyph_would_hang(font_obj, ch: str, width: int, direction: str) -> bool:
    """True when placing this character loops forever inside pyfiglet.

    FigletBuilder wraps when currentTotalWidth >= width, then steps back one
    character if the line has no space to break on. The render loop then moves
    forward again, so the same character is retried forever when an empty line
    still cannot hold it.

    That happens only when the glyph width equals the column limit and empty-line
    smushing cannot pull it under the limit (smush amount 0). A narrower glyph
    fits. A wider glyph raises CharNotPrinted instead of looping. A space still
    advances, because it is recorded as a break point before the wrap.
    """
    if ch == " " or font_obj is None:
        return False
    code = ord(ch)
    if code not in font_obj.chars:
        return False
    glyph_width = font_obj.width.get(code, 0)
    if glyph_width != width:
        return False
    return empty_line_smush(font_obj, ch, direction) == 0


def case_would_hang(args: list[str]) -> bool:
    """True when this CLI case hits the empty-line wrap loop."""
    font_name, width, direction, text = _case_params(args)
    font_obj = get_font_obj(font_name)
    if font_obj is None:
        return False
    direction = resolved_direction(font_obj, direction)
    return any(glyph_would_hang(font_obj, ch, width, direction) for ch in text)


def generate_text(font_name: str, width: int) -> str:
    """Generate a diverse text covering all required categories, guaranteed to fit width."""
    return generate_text_for_category(font_name, width, random.choice(TEXT_CATEGORIES))


def generate_case_args() -> list[str]:
    """Generate command-line arguments for a single test case."""
    font = random.choice(FONTS)
    width = random.choice(WIDTHS)
    justify = random.choice(JUSTIFICATIONS)
    direction = random.choice(DIRECTIONS)

    args = [
        "-f", font,
        "-w", str(width),
        "-j", justify,
        "-D", direction,
    ]

    # Independently enable each of these options in approx 1 out of 4 cases
    if random.random() < 0.25:
        args.append("-r")
    if random.random() < 0.25:
        args.append("-F")
    if random.random() < 0.25:
        args.append("-s")
    if random.random() < 0.25:
        args.append("-n")

    text = generate_text(font, width)
    args.append(text)
    return args


def run_pyfiglet(args: list[str], timeout: float = RENDER_TIMEOUT_SECONDS) -> str:
    """Run original pyfiglet CLI as a subprocess and capture stdout strictly.

    A per-case timeout prevents pathological renderer hangs from blocking the
    entire generator. Timed-out cases raise RenderTimeoutError and must be
    skipped/replaced by the caller (never recorded).
    """
    env = os.environ.copy()
    env["PYTHONPATH"] = str(PYFIGLET_DIR) + os.pathsep + env.get("PYTHONPATH", "")
    env["PYTHONIOENCODING"] = "utf-8"
    cmd = [sys.executable, "-m", "pyfiglet", *args]
    try:
        proc = subprocess.run(
            cmd,
            cwd=str(PYFIGLET_DIR),
            env=env,
            capture_output=True,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired as exc:
        raise RenderTimeoutError(
            f"pyfiglet rendering timed out after {timeout}s: {subprocess.list2cmdline(cmd)}"
        ) from exc

    if proc.returncode != 0:
        stderr_msg = proc.stderr.decode("utf-8", errors="replace")
        stdout_msg = proc.stdout.decode("utf-8", errors="replace")
        raise RuntimeError(
            f"pyfiglet execution failed (exit code {proc.returncode})!\n"
            f"Command: {subprocess.list2cmdline(cmd)}\n"
            f"Stdout: {stdout_msg}\n"
            f"Stderr: {stderr_msg}"
        )

    try:
        return proc.stdout.decode("utf-8", errors="strict")
    except UnicodeDecodeError as exc:
        raise RuntimeError(
            f"pyfiglet output contains invalid UTF-8 bytes!\n"
            f"Command: {subprocess.list2cmdline(cmd)}\n"
            f"Decode error: {exc}\n"
            f"Raw stdout bytes (truncated): {proc.stdout[:200]!r}"
        ) from exc


def try_render_case(args: list[str], timeout: float = RENDER_TIMEOUT_SECONDS):
    """Return (output, None) on success or (None, reason) if the case must be skipped."""
    if case_would_hang(args):
        reason = (
            "would time out: glyph width equals the column limit and "
            "empty-line smushing cannot place it"
        )
        print("Skipping known hanging case: "
              + subprocess.list2cmdline([sys.executable, "-m", "pyfiglet", *args]))
        return None, reason
    try:
        out = run_pyfiglet(args, timeout=timeout)
        return out, None
    except RenderTimeoutError as exc:
        cmd_str = subprocess.list2cmdline([sys.executable, '-m', 'pyfiglet', *args])
        safe_cmd = cmd_str.encode('utf-8', errors='backslashreplace').decode('ascii', errors='ignore')
        print(f"Skipping timed-out case: {safe_cmd}")
        return None, str(exc)
    except RuntimeError as exc:
        # Non-timeout execution failures should not silently become records.
        # Treat as skipped so generation can continue with a safe replacement.
        msg = str(exc).encode('utf-8', errors='backslashreplace').decode('ascii', errors='ignore')
        print(f"Skipping failed case: {msg}")
        return None, str(exc)


def _make_guaranteed_args(seen_signatures: set) -> list[tuple[list[str], str]]:
    """
    Return a small list of argument lists that together guarantee at least one
    case for every font, width, justification, direction, flag (-r/-F/-s/-n),
    and text category.  All choices use the shared RNG so the result is
    deterministic under random.seed(7) and counts against seen_signatures.
    """
    guaranteed: list[tuple[list[str], str]] = []

    def _add(font, width, justify, direction, flags, category):
        """Build one args list from explicit parameter choices."""
        # Build the text for the given font/width/category using the RNG so
        # the RNG state advances deterministically.
        text = generate_text_for_category(font, width, category)
        args = _build_case_args(font, width, justify, direction, flags, text)
        sig = tuple(args)
        if sig not in seen_signatures:
            seen_signatures.add(sig)
            guaranteed.append((args, category))

    # --- 1. Every font (cycle through widths / justifications / directions) ---
    for i, font in enumerate(FONTS):
        width  = WIDTHS[i % len(WIDTHS)]
        just   = JUSTIFICATIONS[i % len(JUSTIFICATIONS)]
        dirn   = DIRECTIONS[i % len(DIRECTIONS)]
        cat    = TEXT_CATEGORIES[i % len(TEXT_CATEGORIES)]
        _add(font, width, just, dirn, [], cat)

    # --- 2. Every width ---
    for i, width in enumerate(WIDTHS):
        font  = FONTS[i % len(FONTS)]
        just  = JUSTIFICATIONS[(i + 1) % len(JUSTIFICATIONS)]
        dirn  = DIRECTIONS[(i + 1) % len(DIRECTIONS)]
        cat   = TEXT_CATEGORIES[(i + 2) % len(TEXT_CATEGORIES)]
        _add(font, width, just, dirn, [], cat)

    # --- 3. Every justification value ---
    for i, just in enumerate(JUSTIFICATIONS):
        font  = FONTS[(i + 2) % len(FONTS)]
        width = WIDTHS[(i + 2) % len(WIDTHS)]
        dirn  = DIRECTIONS[i % len(DIRECTIONS)]
        cat   = TEXT_CATEGORIES[(i + 3) % len(TEXT_CATEGORIES)]
        _add(font, width, just, dirn, [], cat)

    # --- 4. Every direction value ---
    for i, dirn in enumerate(DIRECTIONS):
        font  = FONTS[(i + 4) % len(FONTS)]
        width = WIDTHS[(i + 3) % len(WIDTHS)]
        just  = JUSTIFICATIONS[(i + 2) % len(JUSTIFICATIONS)]
        cat   = TEXT_CATEGORIES[(i + 4) % len(TEXT_CATEGORIES)]
        _add(font, width, just, dirn, [], cat)

    # --- 5. Every boolean flag (-r, -F, -s, -n) individually ---
    for i, flag in enumerate(["-r", "-F", "-s", "-n"]):
        font  = FONTS[(i + 5) % len(FONTS)]
        width = WIDTHS[(i + 1) % len(WIDTHS)]
        just  = JUSTIFICATIONS[(i + 3) % len(JUSTIFICATIONS)]
        dirn  = DIRECTIONS[(i + 2) % len(DIRECTIONS)]
        cat   = TEXT_CATEGORIES[(i + 5) % len(TEXT_CATEGORIES)]
        _add(font, width, just, dirn, [flag], cat)

    # --- 6. Every text category explicitly ---
    for i, cat in enumerate(TEXT_CATEGORIES):
        font  = FONTS[(i + 3) % len(FONTS)]
        width = WIDTHS[(i + 2) % len(WIDTHS)]
        just  = JUSTIFICATIONS[(i + 1) % len(JUSTIFICATIONS)]
        dirn  = DIRECTIONS[(i + 2) % len(DIRECTIONS)]
        _add(font, width, just, dirn, [], cat)

    return guaranteed


def generate_text_for_category(font_name: str, width: int, category: str) -> str:
    """
    Generate text for a *specific* category, with up to 50 retries on
    width-fitness, then fall back to safe printable ASCII characters.
    """
    for _ in range(50):
        if category == "normal_word":
            if random.random() < 0.5:
                text = random.choice(NORMAL_WORDS)
            else:
                text = " ".join(random.sample(NORMAL_WORDS, k=random.randint(2, 4)))
        elif category == "notes_rare":
            text = " ".join(random.sample(NOTES_RARE_CASES, k=random.randint(1, 3)))
        elif category == "special_chars":
            if random.random() < 0.5:
                text = random.choice(SPECIAL_CHARACTERS)
            else:
                chars = ["|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">"]
                text = "".join(random.choice(chars) for _ in range(random.randint(3, 12)))
        elif category == "double_space":
            if random.random() < 0.5:
                text = random.choice(DOUBLE_SPACE_TEMPLATES)
            else:
                words = random.sample(NORMAL_WORDS, k=random.randint(2, 4))
                text = "  ".join(words)
        elif category == "long_sentence":
            if width <= 20:
                text = (random.choice(NORMAL_WORDS) + " "
                        + random.choice(NORMAL_WORDS) + " "
                        + random.choice(NORMAL_WORDS))
            else:
                text = random.choice(LONG_SENTENCES)
        elif category == "latin1":
            text = random.choice(LATIN1_TEXTS)
        elif category == "unsupported":
            text = random.choice(UNSUPPORTED_CHAR_TEXTS)
        else:  # "mixed"
            parts = [
                random.choice(NORMAL_WORDS),
                random.choice(NOTES_RARE_CASES),
                random.choice(["é", "ß", "ñ", "ü"]),
                random.choice(["世界", "🚀", "★"]),
            ]
            random.shuffle(parts)
            text = "  ".join(parts) if random.random() < 0.5 else " ".join(parts)

        if text_fits_width(font_name, text, width):
            return text

    # Safe fallback: printable ASCII characters that fit the width
    font_obj = get_font_obj(font_name)
    if font_obj:
        safe_chars = [
            chr(c) for c in range(32, 127)
            if 0 < font_obj.width.get(c, 0) <= width
        ]
        if safe_chars:
            fallback = "".join(random.choices(safe_chars, k=random.randint(1, 4)))
            if text_fits_width(font_name, fallback, width):
                return fallback

    raise RuntimeError(
        f"Unable to generate any text fitting font '{font_name}' "
        f"with column width {width}. "
        f"No character in font '{font_name}' has width <= {width}."
    )


# Ordered list of all text category names (used by both generate_text and
# the guaranteed-coverage logic above).
TEXT_CATEGORIES = [
    "normal_word",
    "notes_rare",
    "special_chars",
    "double_space",
    "long_sentence",
    "latin1",
    "unsupported",
    "mixed",
]


def _args_signature(args: list[str]):
    return tuple(args)


def _option_list(args: list[str]):
    """Return font, width, justify, direction, and boolean flags for a case."""
    font, width, direction, _text = _case_params(args)
    justify = "auto"
    if "-j" in args[:-1]:
        justify = args[args.index("-j") + 1]
    flags = [flag for flag in ["-r", "-F", "-s", "-n"] if flag in args[:-1]]
    return font, width, justify, direction, flags


def _build_case_args(font, width, justify, direction, flags, text) -> list[str]:
    return ["-f", font, "-w", str(width), "-j", justify, "-D", direction, *flags, text]


def _replacement_for_failed_case(failed_args: list[str], seen_signatures: set, category: str | None):
    """Return a rendered replacement, or raise after a fixed number of attempts.

    Coverage slots pass their text category and keep the same font, width,
    justification, direction, and flags. Random fill slots may vary all of those.
    Timed-out output is never invented: only a successful pyfiglet render is returned.
    """
    font, width, justify, direction, flags = _option_list(failed_args)
    attempts = 0

    while attempts < MAX_REPLACEMENT_ATTEMPTS:
        attempts += 1
        if category is None:
            args = generate_case_args()
        else:
            text = generate_text_for_category(font, width, category)
            args = _build_case_args(font, width, justify, direction, flags, text)
        sig = _args_signature(args)
        if sig in seen_signatures or sig == _args_signature(failed_args):
            continue
        if case_would_hang(args):
            continue
        out, err = try_render_case(args)
        if err is None:
            seen_signatures.add(sig)
            return args, out

    role = category if category is not None else "random"
    raise RuntimeError(
        "Unable to produce a safe rendered replacement after "
        f"{MAX_REPLACEMENT_ATTEMPTS} attempts "
        f"(font={font}, width={width}, justify={justify}, "
        f"direction={direction}, flags={flags}, category={role})."
    )


def generate_dataset(count: int, seen_signatures: set, desc: str) -> list[dict]:
    """
    Generate exactly `count` unique, successfully rendered cases for a dataset.

    Starts from a guaranteed-coverage prefix (covering all required fonts,
    widths, justifications, directions, flags, and text categories) then fills
    randomly. Any case that times out or fails is skipped and replaced with a
    safe alternative; timed-out cases are never written.
    """
    planned = _make_guaranteed_args(seen_signatures)

    cases: list[dict] = []
    planned_index = 0
    attempts = 0
    # Each successful case takes one attempt. Duplicates and skipped cases may
    # take more. Stop with an error well before an unbounded search.
    max_attempts = max(count * 20, count + MAX_REPLACEMENT_ATTEMPTS)
    print(f"Generating {count} {desc} cases with per-case timeout protection...")

    while len(cases) < count:
        attempts += 1
        if attempts > max_attempts:
            raise RuntimeError(
                f"Could not produce {count} {desc} cases after {max_attempts} attempts "
                f"({len(cases)} rendered). Refusing to keep searching."
            )

        if planned_index < len(planned):
            args, category = planned[planned_index]
            planned_index += 1
            sig = _args_signature(args)
        else:
            category = None
            args = generate_case_args()
            sig = _args_signature(args)
            if sig in seen_signatures:
                continue
            seen_signatures.add(sig)

        out, err = try_render_case(args)
        if err is not None:
            seen_signatures.discard(sig)
            repl_args, repl_out = _replacement_for_failed_case(
                args, seen_signatures, category
            )
            cases.append({"args": repl_args, "out": repl_out})
            continue

        seen_signatures.add(sig)
        cases.append({"args": args, "out": out})

    return cases[:count]


def write_jsonl(filepath: pathlib.Path, cases: list[dict]) -> None:
    """Write cases to a JSONL file."""
    filepath.parent.mkdir(parents=True, exist_ok=True)
    with open(filepath, "w", encoding="utf-8", newline="\n") as f:
        for case in cases:
            f.write(json.dumps(case, ensure_ascii=False) + "\n")


def main():
    if not (PYFIGLET_DIR / "pyfiglet").is_dir():
        sys.stderr.write(f"Error: pyfiglet directory not found at {PYFIGLET_DIR}\n")
        sys.exit(1)

    print(f"Using pyfiglet at: {PYFIGLET_DIR}")
    random.seed(7)

    DATA_DIR.mkdir(parents=True, exist_ok=True)

    seen_signatures = set()

    print("Generating visible cases (2,000)...")
    visible_cases = generate_dataset(2000, seen_signatures, "visible")
    write_jsonl(VISIBLE_FILE, visible_cases)
    print(f"Wrote {len(visible_cases)} visible cases to {VISIBLE_FILE}")

    print("Generating hidden cases (2,000)...")
    hidden_cases = generate_dataset(2000, seen_signatures, "hidden")
    write_jsonl(HIDDEN_FILE, hidden_cases)
    print(f"Wrote {len(hidden_cases)} hidden cases to {HIDDEN_FILE}")

    print("Done! Both datasets generated successfully.")


if __name__ == "__main__":
    main()

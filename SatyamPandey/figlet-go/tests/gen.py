#!/usr/bin/env python3
"""
Generate visible and hidden test-case JSONL files for figlet-go.
Runs on the developer's laptop, NOT inside Docker.
Uses the sibling pyfiglet source directly as the reference implementation.
"""

import io
import json
import os
import random
import sys

random.seed(7)

# ---------------------------------------------------------------------------
# Locate sibling pyfiglet repository and import from source directly
# ---------------------------------------------------------------------------
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
TASK_ROOT = os.path.dirname(SCRIPT_DIR)
PYFIGLET_REPO = os.path.join(os.path.dirname(TASK_ROOT), "pyfiglet")

sys.path.insert(0, PYFIGLET_REPO)

import pyfiglet

# ---------------------------------------------------------------------------
# Cache fonts to accelerate batch rendering
# ---------------------------------------------------------------------------
FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]

font_cache = {}
orig_init = pyfiglet.FigletFont.__init__

def cached_font_init(self, font=pyfiglet.DEFAULT_FONT):
    if font in font_cache:
        cached = font_cache[font]
        for k, v in cached.__dict__.items():
            setattr(self, k, v)
    else:
        orig_init(self, font)
        font_cache[font] = self

pyfiglet.FigletFont.__init__ = cached_font_init

# Preload the 8 supported fonts
preloaded_fonts = {f: pyfiglet.FigletFont(font=f) for f in FONTS}

# ---------------------------------------------------------------------------
# Generation options and pools
# ---------------------------------------------------------------------------
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFIES = ["auto", "left", "right", "center"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]

ENGLISH_WORDS = [
    "hello", "world", "figlet", "harbor", "golang", "python", "test", "build",
    "quick", "brown", "fox", "jumps", "lazy", "dog", "terminal", "output",
    "render", "font", "width", "smush", "kerning", "standard", "ascii", "art",
    "system", "program", "string", "character", "buffer", "matrix", "column",
    "simple", "sample", "record", "player", "docker", "source", "hidden",
]

TRICKY_PARTS = [
    "|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">",
    "| / \\ _", "[{()}]", "<|/\\>", "/\\/\\", "><", "[]", "{}", "()",
    "a|b", "x/y", "c\\d", "foo_bar", "[test]", "{val}", "(abc)", "<tag>",
    "|/_", "<>", "][", "}{", ")(", "(_/\\_)", "<[[{()]]>",
]

DOUBLE_SPACES = [
    "hello  world", "foo  bar  baz", "a  b  c  d", "two  spaces",
    "left  and  right", "one   two   three", "wide  gap  test",
    "double  space  here", "gap  between  words", "spaced  out  text",
]

LONG_SENTENCES = [
    "The quick brown fox jumps over the lazy dog.",
    "Pack my box with five dozen liquor jugs.",
    "How vexingly quick daft zebras jump over fences.",
    "Sphinx of black quartz, judge my vow.",
    "To be or not to be, that is the question.",
    "A journey of a thousand miles begins with a single step.",
    "All that glitters is not gold, often told.",
    "In the beginning there was light and shadow.",
]

LATIN1_WORDS = [
    "café", "naïve", "résumé", "über", "straße", "señor", "piñata",
    "crème", "façade", "Zürich", "garçon", "Ångström", "fiancée",
    "Schloß", "Grüße", "é", "ß", "ñ", "ü", "ö", "ä", "Ä", "Ö", "Ü",
    "café latte", "über cool", "straße 42",
]

MISSING_CHAR_WORDS = [
    "hello★world", "test♠card", "symbol♣suit", "heart♥ace", "diamond♦king",
    "alpha α beta β", "gamma γ delta δ", "pi π omega Ω", "sum ∑ int ∫",
    "hello日world", "日本語テスト", "中文测试", "arrow→test", "music♫note",
]

def random_text():
    r = random.random()
    if r < 0.25:
        return " ".join(random.sample(ENGLISH_WORDS, k=random.randint(1, 3)))
    elif r < 0.45:
        return random.choice(TRICKY_PARTS)
    elif r < 0.60:
        return random.choice(DOUBLE_SPACES)
    elif r < 0.75:
        return random.choice(LONG_SENTENCES)
    elif r < 0.90:
        return random.choice(LATIN1_WORDS)
    else:
        return random.choice(MISSING_CHAR_WORDS)

def is_safe(font_name, width, text):
    font = preloaded_fonts[font_name]
    for ch in text:
        if font.width.get(ord(ch), 0) >= width:
            return False
    return True

# ---------------------------------------------------------------------------
# Capture stdout via io.StringIO from pyfiglet's main entry point
# ---------------------------------------------------------------------------
class CaptureStdout(io.StringIO):
    def detach(self):
        return self

    def write(self, s):
        if isinstance(s, bytes):
            s = s.decode("utf-8")
        return super().write(s)

def generate_one_case():
    while True:
        font = random.choice(FONTS)
        width = random.choice(WIDTHS)
        text = random_text()
        if not is_safe(font, width, text):
            continue

        args = ["-f", font, "-w", str(width)]
        if random.random() < 0.5:
            args.extend(["-j", random.choice(JUSTIFIES)])
        if random.random() < 0.5:
            args.extend(["-D", random.choice(DIRECTIONS)])
        for flag in ["-r", "-F", "-s", "-n"]:
            if random.random() < 0.25:
                args.append(flag)
        args.append(text)

        buf = CaptureStdout()
        old_stdout, old_argv = sys.stdout, sys.argv
        try:
            sys.stdout, sys.argv = buf, ["pyfiglet"] + args
            ret = pyfiglet.main()
            if ret == 0:
                return args, buf.getvalue()
        except Exception:
            pass
        finally:
            sys.stdout, sys.argv = old_stdout, old_argv

# ---------------------------------------------------------------------------
# Main generation runner
# ---------------------------------------------------------------------------
def main():
    data_dir = os.path.join(SCRIPT_DIR, "data")
    os.makedirs(data_dir, exist_ok=True)

    visible_path = os.path.join(data_dir, "visible.jsonl")
    hidden_path = os.path.join(data_dir, "hidden.jsonl")

    seen_visible = set()
    visible_cases = []
    while len(visible_cases) < 2000:
        args, out = generate_one_case()
        key = tuple(args)
        if key not in seen_visible:
            seen_visible.add(key)
            visible_cases.append({"args": args, "out": out})

    with open(visible_path, "w", encoding="utf-8", newline="\n") as f:
        for case in visible_cases:
            f.write(json.dumps(case, ensure_ascii=False) + "\n")

    seen_hidden = set()
    hidden_cases = []
    while len(hidden_cases) < 2000:
        args, out = generate_one_case()
        key = tuple(args)
        if key not in seen_visible and key not in seen_hidden:
            seen_hidden.add(key)
            hidden_cases.append({"args": args, "out": out})

    with open(hidden_path, "w", encoding="utf-8", newline="\n") as f:
        for case in hidden_cases:
            f.write(json.dumps(case, ensure_ascii=False) + "\n")

if __name__ == "__main__":
    main()

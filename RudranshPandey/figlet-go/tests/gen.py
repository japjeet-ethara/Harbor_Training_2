#!/usr/bin/env python3
"""Generate visible.jsonl and hidden.jsonl test cases for figlet-go task."""

import json
import os
import random
import sys
import io

# Add pyfiglet to path (assumes pyfiglet/ sits next to figlet-go/)
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PYFIGLET_DIR = os.path.join(os.path.dirname(SCRIPT_DIR), "..", "pyfiglet")
sys.path.insert(0, PYFIGLET_DIR)

import pyfiglet

class FakeBinaryStdout(io.BytesIO):
    """Custom stdout buffer that supports detach() and string/bytes writing."""
    def detach(self):
        return self
    def write(self, b):
        if isinstance(b, str):
            b = b.encode('utf-8')
        return super().write(b)

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFIES = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]

NORMAL_TEXTS = [
    "Hello", "World", "Hello World", "FIGlet", "Go port",
    "Testing 123", "ABC", "xyz", "UPPER lower", "Mixed Case Text",
    "Short", "ab", "The quick brown fox", "jumps over the lazy dog",
    "Antigravity", "pyfiglet", "Harbor task", "smushing test",
    "Open Source", "MIT License", "Build Pass", "No Python",
    "Standard Library", "byte for byte", "exact match",
    "wrap this long sentence that should definitely cause line wrapping in narrow widths",
    "another long line of text that needs to be wrapped across multiple figlet rows for testing",
    "one more really long sentence to make sure wrapping works correctly with all the different width settings",
]

TRICKY_TEXTS = [
    "|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">",
    "[]", "{}", "()", "<>", "><", ")(", "][", "}{",
    "|/\\", "[{(<>)}]", "(/)", "[\\]",
    "a|b", "x_y", "m/n",
    "double  spaces", "more  double  spaces  here",
    "  leading spaces", "trailing spaces  ",
    "a  b  c  d", "test  with   multiple   spaces",
    # Latin-1 / extended characters
    "café", "naïve", "résumé", "über", "straße", "señor", "año",
    "é", "ß", "ñ", "ü", "ö", "ä", "î", "ê",
    # Characters missing from some fonts
    "→", "★", "♠", "♦", "€", "£", "¥", "©", "®",
    # Mixed combinations
    "hello|world", "foo/bar\\baz", "test_case", "arr[0]",
    "func(x)", "map{k:v}", "a<b>c",
]

# Texts guaranteed safe when width == 10 (individual character width < 10 across fonts)
SAFE_TEXTS_FOR_10 = [
    "|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">",
    "a", "b", "c", "x", "y", "z", "1", "2",
]

def run_pyfiglet(args_list):
    """Run pyfiglet.main() with given args and capture exact stdout."""
    orig_stdout = sys.stdout
    buf = FakeBinaryStdout()
    sys.stdout = buf
    old_argv = sys.argv
    sys.argv = ["pyfiglet"] + args_list
    try:
        pyfiglet.main()
    except SystemExit:
        pass
    finally:
        sys.argv = old_argv
        sys.stdout = orig_stdout
    return buf.getvalue().decode("utf-8")


def generate_cases(n, rng):
    """Generate n test cases."""
    cases = []
    while len(cases) < n:
        font = rng.choice(FONTS)
        width = rng.choice(WIDTHS)
        justify = rng.choice(JUSTIFIES)
        direction = rng.choice(DIRECTIONS)

        use_r = rng.random() < 0.25
        use_F = rng.random() < 0.25
        use_s = rng.random() < 0.25
        use_n = rng.random() < 0.25

        # For narrow width (10), select from safe texts to prevent pyfiglet infinite wrap loop
        if width == 10:
            text = rng.choice(SAFE_TEXTS_FOR_10)
        else:
            if rng.random() < 0.5:
                text = rng.choice(NORMAL_TEXTS)
            else:
                text = rng.choice(TRICKY_TEXTS)

        args = ["-f", font, "-w", str(width), "-j", justify, "-D", direction]
        if use_r:
            args.append("-r")
        if use_F:
            args.append("-F")
        if use_s:
            args.append("-s")
        if use_n:
            args.append("-n")
        args.append(text)

        out = run_pyfiglet(args)
        cases.append({"args": args, "out": out})

    return cases


def write_jsonl(cases, filepath):
    os.makedirs(os.path.dirname(filepath), exist_ok=True)
    with open(filepath, "w", encoding="utf-8") as f:
        for case in cases:
            f.write(json.dumps(case, ensure_ascii=False) + "\n")


def main():
    rng = random.Random(7)
    data_dir = os.path.join(SCRIPT_DIR, "data")

    print("Generating 2000 visible cases...")
    visible = generate_cases(2000, rng)
    write_jsonl(visible, os.path.join(data_dir, "visible.jsonl"))
    print(f"  Written {len(visible)} cases to visible.jsonl")

    print("Generating 2000 hidden cases...")
    hidden = generate_cases(2000, rng)
    write_jsonl(hidden, os.path.join(data_dir, "hidden.jsonl"))
    print(f"  Written {len(hidden)} cases to hidden.jsonl")

    for name, cases in [("visible", visible), ("hidden", hidden)]:
        rtl = sum(1 for c in cases if "right-to-left" in c["args"])
        narrow_rtl = sum(
            1 for c in cases
            if "-D" in c["args"]
            and c["args"][c["args"].index("-D") + 1] == "right-to-left"
            and int(c["args"][c["args"].index("-w") + 1]) <= 20
        )
        print(f"  {name}: {len(cases)} total, {rtl} RTL, {narrow_rtl} RTL+narrow (<=20)")


if __name__ == "__main__":
    main()

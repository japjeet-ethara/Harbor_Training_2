#!/usr/bin/env python3
"""Generate the frozen answer files for the figlet-go task.

Runs on the task author's laptop (NOT inside Docker), with the pyfiglet checkout from
Step 1 (fonts already copied into pyfiglet/fonts/) next to the task folder:

    <dir>/pyfiglet/        <- git checkout 255827efbc09d662875dad50fa0edc1e1263bf45
    <dir>/figlet-go/tests/gen.py

Every "out" is produced by running the ORIGINAL pyfiglet main() and capturing its
stdout bytes exactly. Nothing is ever written by hand.

Usage: python tests/gen.py [--pyfiglet PATH]
"""
import argparse
import json
import random
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

N_VISIBLE = 2000
N_HIDDEN = 2000
BATCH = 1500
TIMEOUT_SEC = 4

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = [
    "hello", "world", "Go", "figlet", "Harbor", "port", "twin", "exam", "shout",
    "ABC", "xyz", "Mississippi", "WWW", "mmm", "ijl", "Quiz", "Hi!", "42", "2026",
    "OK?", "yes", "no", "a", "I", "Zz", "Jump", "lazy", "dog", "fox", "pyfiglet",
]

# The "rare cases" list from NOTES.md, plus the characters the task page asks for.
TRICKY = [
    "|_|", "/\\/\\", "\\/\\/", "[]{}()<>", "_/_\\_", "<><>", "><", "()()", "{[(<>)]}",
    "|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">", "||", "__", "/\\", "\\/",
    "[]", "{}", "()", "_|_", "|/|", "a|b", "x_y", "(_)", "<_>", "/_/", "\\_\\",
    "é", "ß", "ñ", "Ä", "Öl", "über", "Straße", "niño", "café", "Ünïcödé",
    "€", "ā", "→", "★", "\t", "日本", "😀", "~", "`", "^", "@#$%&*", "$", "x$y",
    "!!", "?", ".", ",", ";:", "'\"", "+-=", "0O0", "1l|I",
]

SENTENCES = [
    "the quick brown fox jumps over the lazy dog",
    "Hello World from a Go twin",
    "every rule has to be copied exactly space for space",
    "wrap me please because I am long",
    "one two three four five six seven eight nine ten",
    "a bb ccc dddd eeeee ffffff",
    "|_| /\\ [] {} () <> |_| /\\",
    "Grüße aus Köln, señor",
]


def make_text(rng):
    kind = rng.randrange(10)
    if kind <= 2:  # normal word(s)
        text = " ".join(rng.choice(WORDS) for _ in range(rng.randint(1, 2)))
    elif kind <= 4:  # tricky characters, often glued to letters
        parts = [rng.choice(TRICKY) for _ in range(rng.randint(1, 3))]
        if rng.random() < 0.5:
            parts.insert(rng.randrange(len(parts) + 1), rng.choice(WORDS))
        text = rng.choice(["", " "]).join(parts)
    elif kind == 5:  # double spaces / leading / trailing spaces
        a, b = rng.choice(WORDS), rng.choice(WORDS + TRICKY)
        text = rng.choice([f"{a}  {b}", f" {a}", f"{a} ", f"{a}   {b}", f"  {a}  {b}  "])
    elif kind <= 7:  # long sentences that must wrap
        text = rng.choice(SENTENCES)
        if rng.random() < 0.3:
            text += " " + rng.choice(SENTENCES)
    elif kind == 8:  # explicit newlines
        text = "\n".join(rng.choice(WORDS + TRICKY) for _ in range(rng.randint(2, 3)))
    else:  # random printable soup, including Latin-1 and characters fonts lack
        alphabet = [chr(c) for c in range(33, 127)] + list("éßñÄÖÜäöü€ā→ ")
        text = "".join(rng.choice(alphabet) for _ in range(rng.randint(1, 12)))
    return text or "x"


def make_case(rng):
    opts = []
    if rng.random() < 0.9:
        opts.append(["-f", rng.choice(FONTS)])
    if rng.random() < 0.9:
        opts.append(["-w", str(rng.choice(WIDTHS))])
    if rng.random() < 0.8:
        opts.append(["-j", rng.choice(JUSTIFY)])
    if rng.random() < 0.8:
        opts.append(["-D", rng.choice(DIRECTION)])
    for flag in ("-r", "-F", "-s", "-n"):
        if rng.random() < 0.25:
            opts.append([flag])
    rng.shuffle(opts)
    args = [a for o in opts for a in o]

    text = make_text(rng)
    if text.startswith("-"):
        text = "x" + text
    # Sometimes pass the text as several arguments (pyfiglet joins them with one space).
    if " " in text and "  " not in text and text.strip(" ") == text and rng.random() < 0.3:
        args += text.split(" ")
    else:
        args.append(text)
    return args


def run_original(pyfiglet_dir, args):
    """Run the original pyfiglet main() and return its stdout, or None if it fails."""
    cmd = [sys.executable, "-c", "import sys; from pyfiglet import main; sys.exit(main())", *args]
    try:
        p = subprocess.run(cmd, cwd=pyfiglet_dir, capture_output=True, timeout=TIMEOUT_SEC)
    except subprocess.TimeoutExpired:
        return None  # pyfiglet never finishes on this input (e.g. char as wide as -w)
    if p.returncode != 0:
        return None  # pyfiglet crashes on this input (e.g. CharNotPrinted)
    try:
        return p.stdout.decode("utf-8")
    except UnicodeDecodeError:
        return None


def main():
    here = Path(__file__).resolve().parent
    ap = argparse.ArgumentParser()
    ap.add_argument("--pyfiglet", default=str(here.parents[1] / "pyfiglet"))
    ns = ap.parse_args()
    pyfiglet_dir = Path(ns.pyfiglet).resolve()
    if not (pyfiglet_dir / "pyfiglet" / "fonts" / "standard.flf").is_file():
        sys.exit(f"{pyfiglet_dir}: pyfiglet/fonts/ missing - do the Makefile font step first")

    random.seed(7)
    rng = random  # the seeded module-level generator
    seen = set()
    cases = []
    skipped = 0
    with ThreadPoolExecutor(max_workers=8) as pool:
        while len(cases) < N_VISIBLE + N_HIDDEN:
            batch = []
            for _ in range(BATCH):
                args = make_case(rng)
                key = json.dumps(args)
                if key in seen:  # never the same command line twice (also across files)
                    continue
                seen.add(key)
                batch.append(args)
            outs = pool.map(lambda a: run_original(pyfiglet_dir, a), batch)
            for args, out in zip(batch, outs):
                if out is None:
                    skipped += 1
                elif len(cases) < N_VISIBLE + N_HIDDEN:
                    cases.append({"args": args, "out": out})
            print(f"  {len(cases)} cases, {skipped} skipped", file=sys.stderr)

    data = here / "data"
    data.mkdir(exist_ok=True)
    for name, part in (("visible", cases[:N_VISIBLE]), ("hidden", cases[N_VISIBLE:])):
        with open(data / f"{name}.jsonl", "w", encoding="utf-8", newline="\n") as fh:
            for c in part:
                fh.write(json.dumps(c, ensure_ascii=True) + "\n")
        rtl = sum("right-to-left" in c["args"] for c in part)
        rtl_narrow = sum("right-to-left" in c["args"] and any(w in c["args"] for w in ("10", "20"))
                         for c in part)
        wrapped = sum(c["out"].count("\n") > 12 for c in part)
        print(f"{name}: {len(part)} cases, right-to-left {rtl}, right-to-left+narrow {rtl_narrow}, "
              f"multi-line {wrapped}")


if __name__ == "__main__":
    main()

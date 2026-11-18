#!/usr/bin/env python3
"""Generate frozen test cases by running the ORIGINAL pyfiglet as a subprocess."""
import json
import os
import random
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", "..", "pyfiglet"))
DATA = os.path.join(HERE, "data")

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = ["hello", "world", "go", "figlet", "Harbor", "abc", "Zebra", "quick",
         "fox", "a", "I", "12345", "Hi!", "test", "pyfiglet", "smush", "MiXeD",
         "x", "jump", "Hello World", "oOoO", "lll", "iii", "AVAV"]
TRICKY = ["|_|", "||", "|||", "/\\", "\\/", "/\\/\\", "_/", "\\_", "_|_", "[]",
          "{}", "()", "<>", "><", "][", "}{", ")(", "/ /", "| |", "_ _", "|/",
          "|\\", "/|", "\\|", "^v", "=-", "...", "!?", "~~", "#@", "L_L", "T_T"]
TRICKY_CHARS = list("|/\\_[]{}()<>") + [" "]
LONG = [
    "the quick brown fox jumps over the lazy dog",
    "pack my box with five dozen liquor jugs",
    "a very long sentence that must wrap across several lines",
    "wrapping words at narrow widths is tricky",
    "one two three four five six seven eight nine ten",
]
LATIN = ["\u00e9", "\u00df", "\u00f1", "caf\u00e9", "Stra\u00dfe", "se\u00f1or",
         "\u00c0\u00c9\u00ce\u00d5\u00dc", "na\u00efve"]
UNKNOWN = ["\u65e5\u672c\u8a9e", "\u20ac", "\U0001F600", "hi\u20ac",
           "a\U0001F600b", "\u03a9mega"]


def make_text():
    r = random.random()
    if r < 0.22:
        return random.choice(WORDS)
    if r < 0.42:
        return random.choice(TRICKY)
    if r < 0.60:
        while True:
            t = "".join(random.choice(TRICKY_CHARS) for _ in range(random.randint(2, 8)))
            if t.strip():
                return t
    if r < 0.70:
        return random.choice(LONG)
    if r < 0.78:
        return random.choice(WORDS) + "  " + random.choice(WORDS)
    if r < 0.85:
        return random.choice(LATIN) + random.choice(WORDS)
    if r < 0.91:
        return random.choice(WORDS) + random.choice(UNKNOWN)
    return random.choice(WORDS) + " " + random.choice(TRICKY) + " " + random.choice(WORDS)


def make_args():
    args = []
    if random.random() < 0.85:
        args += ["-f", random.choice(FONTS)]
    if random.random() < 0.85:
        args += ["-w", str(random.choice(WIDTHS))]
    if random.random() < 0.80:
        args += ["-j", random.choice(JUSTIFY)]
    if random.random() < 0.80:
        args += ["-D", random.choice(DIRECTION)]
    for flag in ["-r", "-F", "-s", "-n"]:
        if random.random() < 0.25:
            args.append(flag)
    args.append(make_text())
    return args


def run(args):
    env = dict(os.environ, PYTHONIOENCODING="utf-8", PYTHONPATH=REPO)
    try:
        p = subprocess.run([sys.executable, "-m", "pyfiglet"] + args, cwd=REPO,
                           env=env, stdin=subprocess.DEVNULL,
                           capture_output=True, timeout=4)
    except subprocess.TimeoutExpired:
        return None
    if p.returncode != 0 or not p.stdout:
        return None
    try:
        return p.stdout.decode("utf-8")
    except UnicodeDecodeError:
        return None


def write(name, cases):
    path = os.path.join(DATA, name)
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        for c in cases:
            f.write(json.dumps(c, ensure_ascii=False) + "\n")


def main():
    random.seed(7)
    seen = set()
    cands = []
    while len(cands) < 12000:
        args = make_args()
        key = json.dumps(args, ensure_ascii=False)
        if key in seen:
            continue
        seen.add(key)
        cands.append(args)

    cases = []
    i = 0
    with ThreadPoolExecutor(8) as ex:
        while len(cases) < 4000 and i < len(cands):
            chunk = cands[i:i + 400]
            i += 400
            for a, o in zip(chunk, ex.map(run, chunk)):
                if o is not None and len(cases) < 4000:
                    cases.append({"args": a, "out": o})
            print("tried", i, "kept", len(cases), file=sys.stderr, flush=True)
    if len(cases) < 4000:
        sys.exit("could not generate enough cases: %d" % len(cases))

    os.makedirs(DATA, exist_ok=True)
    write("visible.jsonl", cases[:2000])
    write("hidden.jsonl", cases[2000:4000])
    rtl = sum(1 for c in cases if "right-to-left" in c["args"])
    narrow = sum(1 for c in cases
                 if any(c["args"][k] == "-w" and c["args"][k + 1] in ("10", "20")
                        for k in range(len(c["args"]) - 1)))
    print("written:", len(cases), "right-to-left:", rtl, "width<=20:", narrow)


if __name__ == "__main__":
    main()

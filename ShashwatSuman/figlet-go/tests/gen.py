#!/usr/bin/env python3
"""Generate the frozen test cases for the figlet-go task.

Runs on the task author's machine (NOT inside Docker), with the pyfiglet
checkout from Step 1 next to the task folder:

    Shashwat Task 2/
      pyfiglet/      <- git checkout 255827ef..., fonts copied to pyfiglet/fonts
      figlet-go/     <- this task

Every "out" is produced by running the ORIGINAL pyfiglet main()
(`python -m pyfiglet <args>`) and capturing its stdout bytes exactly.
Cases where pyfiglet crashes or never finishes are discarded.
Output is deterministic: candidates are drawn from random.seed(7) in a fixed
order, and kept in that order no matter how the parallel runs finish.
"""
from __future__ import annotations

import json
import os
import random
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor

HERE = os.path.dirname(os.path.abspath(__file__))
PYFIGLET = os.path.normpath(os.path.join(HERE, "..", "..", "pyfiglet"))
DATA = os.path.join(HERE, "data")

N_VISIBLE = 2000
N_HIDDEN = 2000
N_CANDIDATES = 4400
TIMEOUT = 8

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = [
    "hello", "world", "Harbor", "Go", "figlet", "port", "twin", "smush",
    "shout", "giant", "letters", "test", "kitchen", "agent", "oracle",
    "ABC", "xyz", "Quick", "Brown", "Fox", "jumps", "lazy", "dog", "42",
    "2026", "OK", "hi", "WWW", "mmm", "Mississippi", "zigzag", "Vector",
]

# Rare cases from NOTES.md: smushing edges, hierarchy, pairs, big-X, etc.
TRICKY = [
    "|_|", "/\\", "\\/", "[]", "][", "{}", "}{", "()", ")(", "<>", "><",
    "_/", "\\_", "_|_", "||", "//", "\\\\", "__", "|/", "/|", "[{(<>)}]",
    "<<>>", "/\\/\\", "\\/\\/", "_-_", "$$", "$", "a$b", "#@!", "^v^",
    "~`'\"", ";:,.", "?!", "1+1=2", "50%", "&*", "a_b_c", "x/y\\z",
    "| / \\ _ [ ] { } ( ) < >", "a  b", "double  space", "  lead",
    "trail  ", "é", "ß", "ñ", "Ä Ö Ü", "äöü", "café", "straße", "niño",
    "€", "☃", "日本", "a b", "été", "Ł", "x\ty",
    "MAX", "WAVE", "PRRP", "bmw", "v^v", "AVA",
]

SENTENCES = [
    "the quick brown fox jumps over the lazy dog",
    "Harbor tasks need a fair exam and a twin in Go",
    "shout in giant letters made of slashes and underscores",
    "a b c d e f g h i j k l m n o p q r s t u v w x y z",
    "|_| /\\ [] {} () <> and some words to wrap around",
    "wrap me please because I am a rather long sentence",
    "Ünïcödé façade naïve résumé",
    "one  two   three    four",
]

SINGLE = list("|/\\_[]{}()<>")


def make_text(rng: random.Random) -> str:
    kind = rng.random()
    if kind < 0.25:
        return rng.choice(WORDS)
    if kind < 0.50:
        return rng.choice(TRICKY)
    if kind < 0.62:
        return rng.choice(SENTENCES)
    if kind < 0.75:
        return "".join(rng.choice(SINGLE) for _ in range(rng.randint(1, 6)))
    if kind < 0.90:
        parts = [rng.choice(WORDS + TRICKY) for _ in range(rng.randint(2, 5))]
        return (" " * rng.choice([1, 1, 1, 2])).join(parts)
    return rng.choice(WORDS) + rng.choice(TRICKY) + rng.choice(WORDS)


def make_args(rng: random.Random) -> list[str]:
    args: list[str] = []
    if rng.random() < 0.9:
        args += ["-f", rng.choice(FONTS)]
    if rng.random() < 0.9:
        args += ["-w", str(rng.choice(WIDTHS))]
    if rng.random() < 0.8:
        args += ["-j", rng.choice(JUSTIFY)]
    if rng.random() < 0.85:
        # right-to-left is weighted up: it is where ports break most.
        args += ["-D", rng.choice(DIRECTION + ["right-to-left"])]
    for flag in ("-r", "-F", "-s", "-n"):
        if rng.random() < 0.25:
            args.append(flag)

    text = make_text(rng)
    # Words must never look like options to optparse.
    text = " ".join(("x" + w) if w.startswith("-") else w for w in text.split(" "))
    if text.strip() == "":
        text = "x" + text
    if "  " not in text and " " in text.strip() and text == text.strip() and rng.random() < 0.3:
        args += text.split(" ")          # several argv words, joined by main()
    else:
        args.append(text)
    return args


def run_pyfiglet(args: list[str]) -> str | None:
    env = dict(os.environ, PYTHONIOENCODING="utf-8", PYTHONUTF8="1")
    try:
        p = subprocess.run(
            [sys.executable, "-m", "pyfiglet", *args],
            cwd=PYFIGLET, env=env, capture_output=True, timeout=TIMEOUT,
        )
    except subprocess.TimeoutExpired:
        return None
    if p.returncode != 0 or p.stderr:
        return None
    return p.stdout.decode("utf-8")


def main() -> None:
    if not os.path.isfile(os.path.join(PYFIGLET, "pyfiglet", "fonts", "standard.flf")):
        sys.exit(f"pyfiglet with fonts/ not found at {PYFIGLET} (do the Makefile font step)")

    random.seed(7)
    rng = random.Random(random.random())
    candidates: list[list[str]] = []
    seen: set[str] = set()
    while len(candidates) < N_CANDIDATES:
        a = make_args(rng)
        key = json.dumps(a, ensure_ascii=False)
        if key not in seen:
            seen.add(key)
            candidates.append(a)

    with ThreadPoolExecutor(max_workers=max(4, (os.cpu_count() or 4))) as pool:
        outs = list(pool.map(run_pyfiglet, candidates))

    good = [(a, o) for a, o in zip(candidates, outs) if o is not None]
    if len(good) < N_VISIBLE + N_HIDDEN:
        sys.exit(f"only {len(good)} usable cases, need {N_VISIBLE + N_HIDDEN}")

    os.makedirs(DATA, exist_ok=True)
    splits = {
        "visible.jsonl": good[:N_VISIBLE],
        "hidden.jsonl": good[N_VISIBLE:N_VISIBLE + N_HIDDEN],
    }
    for name, cases in splits.items():
        with open(os.path.join(DATA, name), "w", encoding="utf-8", newline="\n") as f:
            for a, o in cases:
                f.write(json.dumps({"args": a, "out": o}, ensure_ascii=False) + "\n")

    hidden = splits["hidden.jsonl"]
    rtl_narrow = sum(1 for a, _ in hidden
                     if "right-to-left" in a and any(w in a for w in ("10", "20", "40")))
    print(f"candidates={len(candidates)} usable={len(good)} "
          f"dropped={len(candidates) - len(good)}")
    print(f"hidden: right-to-left={sum('right-to-left' in a for a, _ in hidden)} "
          f"rtl+narrow(<=40)={rtl_narrow} "
          f"-r={sum('-r' in a for a, _ in hidden)} -F={sum('-F' in a for a, _ in hidden)} "
          f"-s={sum('-s' in a for a, _ in hidden)} -n={sum('-n' in a for a, _ in hidden)}")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Generate test cases for figlet-go by running the original pyfiglet."""

import json
import subprocess
import sys
import random
from pathlib import Path
from collections import defaultdict


def main():
    random.seed(7)
    rng = random.Random(7)

    fonts = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
    widths = [10, 20, 40, 60, 80, 120]
    justifies = ["auto", "left", "center", "right"]
    directions = ["auto", "left-to-right", "right-to-left"]

    # ---- text building blocks (all plain lists, never sets or dicts) ----

    simple_words = [
        "Hello", "World", "FIGlet", "Go", "Test", "ASCII", "art",
        "Python", "Rust", "Code", "Fun", "Big", "Small", "Quick",
        "Fox", "Lazy", "Dog", "Jump", "Over", "The", "Brown",
        "Rain", "Sun", "Moon", "Star", "Fire", "Ice", "Wind",
        "Cake", "Pie", "Jam", "Tea", "Milk", "Egg", "Ham",
        "Red", "Blue", "Green", "Gold", "Pink", "Gray", "Cyan",
        "One", "Two", "Six", "Ten", "Zip", "Max", "Mix",
    ]

    length_words = ["a", "ab", "hey", "four", "seven", "xy", "zoo", "hi", "bee", "cats"]

    accented_words = [
        "café", "mañana", "piñata", "naïve", "résumé",
        "über", "Ärger", "Öl", "Üniform", "straße",
        "é", "ß", "ñ", "Ä", "Ö", "Ü", "ä", "ö", "ü",
    ]

    special_singles = ["|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">"]

    special_pairs = [
        "[]", "{}", "()", "<>", "||", "|_|",
        "[]{}", "()<>", "||//", "/\\", "\\/",
        "()|", "<>_",
    ]

    unknown_chars_texts = [
        "€", "☃", "€☃", "Hello€World", "☃snowman",
        "café€", "Test☃ing", "Ä€ß",
    ]

    long_sentences = [
        "The quick brown fox jumps over the lazy dog",
        "Pack my box with five dozen liquor jugs",
        "How vexingly quick daft zebras jump",
        "The five boxing wizards jump quickly",
        "Sphinx of black quartz judge my vow",
        "Two driven jocks help fax my big quiz",
        "A wonderful serenity has taken possession of my entire soul",
        "Far far away behind the word mountains",
        "Even the all powerful Pointing has no control about the blind texts",
        "One morning when Gregor Samsa woke from troubled dreams",
    ]

    all_pool = (
        simple_words
        + length_words
        + accented_words
        + special_singles
        + special_pairs
        + unknown_chars_texts
        + long_sentences
    )

    def make_text():
        """Return a random text: never empty, never starts with '-'."""
        while True:
            strategy = rng.randint(0, 10)
            if strategy == 0:
                # single item from the pool
                t = rng.choice(all_pool)
            elif strategy == 1:
                # two simple words
                t = rng.choice(simple_words) + " " + rng.choice(simple_words)
            elif strategy == 2:
                # simple word + accented word
                t = rng.choice(simple_words) + " " + rng.choice(accented_words)
            elif strategy == 3:
                # special single + simple word
                t = rng.choice(special_singles) + " " + rng.choice(simple_words)
            elif strategy == 4:
                # long sentence
                t = rng.choice(long_sentences)
            elif strategy == 5:
                # 2-5 mixed tokens
                n = rng.randint(2, 5)
                t = " ".join(rng.choice(all_pool) for _ in range(n))
            elif strategy == 6:
                # trailing space
                t = rng.choice(all_pool) + " "
            elif strategy == 7:
                # accented word + special pair
                t = rng.choice(accented_words) + " " + rng.choice(special_pairs)
            elif strategy == 8:
                # length word + special single
                t = rng.choice(length_words) + " " + rng.choice(special_singles)
            elif strategy == 9:
                # double space between two words
                t = rng.choice(simple_words) + "  " + rng.choice(simple_words)
            else:
                # unknown-char text + simple word
                t = rng.choice(unknown_chars_texts) + " " + rng.choice(simple_words)

            if t and not t.startswith("-"):
                return t

    def make_args():
        font = rng.choice(fonts)
        width = rng.choice(widths)
        justify = rng.choice(justifies)
        direction = rng.choice(directions)

        args = ["-f", font, "-w", str(width), "-j", justify, "-D", direction]

        # each of -r, -F, -s, -n is added independently with probability 1/4
        if rng.randint(0, 3) == 0:
            args.append("-r")
        if rng.randint(0, 3) == 0:
            args.append("-F")
        if rng.randint(0, 3) == 0:
            args.append("-s")
        if rng.randint(0, 3) == 0:
            args.append("-n")

        args.append(make_text())
        return args

    # ---- paths ----
    script_dir = Path(__file__).resolve().parent          # tests/
    repo_dir = script_dir.parent                          # figlet-go/
    pyfiglet_dir = repo_dir.parent / "pyfiglet"           # ../pyfiglet/

    if not pyfiglet_dir.is_dir():
        print(f"ERROR: pyfiglet not found at {pyfiglet_dir}", file=sys.stderr)
        sys.exit(1)

    data_dir = script_dir / "data"
    data_dir.mkdir(parents=True, exist_ok=True)

    # ---- generate cases ----
    seen_args = set()       # for dedup only, never picked from
    all_cases = []
    ok = 0
    error = 0
    timeout = 0
    duplicate = 0
    blank = 0
    target = 4000

    print(f"Generating {target} test cases (this takes a few minutes) ...")

    while len(all_cases) < target:
        args = make_args()
        key = tuple(args)

        if key in seen_args:
            duplicate += 1
            continue
        seen_args.add(key)

        try:
            proc = subprocess.run(
                [sys.executable, "-m", "pyfiglet"] + args,
                cwd=str(pyfiglet_dir),
                capture_output=True,
                timeout=5,
            )
        except subprocess.TimeoutExpired:
            timeout += 1
            continue

        if proc.returncode != 0:
            error += 1
            continue

        out = proc.stdout.decode("utf-8")
        if not out.strip():
            blank += 1
            continue
        ok += 1
        all_cases.append({"args": args, "out": out})

        if ok % 500 == 0:
            print(f"  ... {ok} ok so far")

    # first 2000 -> visible, next 2000 -> hidden
    visible = all_cases[:2000]
    hidden = all_cases[2000:4000]

    # ---- write JSONL files ----
    for name, cases in [("visible.jsonl", visible), ("hidden.jsonl", hidden)]:
        path = data_dir / name
        with open(path, "w", encoding="utf-8", newline="") as f:
            for case in cases:
                f.write(json.dumps(case, ensure_ascii=False) + "\n")
        print(f"Wrote {path} ({len(cases)} cases)")

    # ---- summary stats ----
    print(f"\nRuns: ok={ok}  error={error}  timeout={timeout}  duplicate={duplicate}  blank={blank}")

    for label, cases in [("visible.jsonl", visible), ("hidden.jsonl", hidden)]:
        print(f"\n{'=' * 60}")
        print(f"{label}  ({len(cases)} cases)")
        print("=" * 60)

        fc = defaultdict(int)    # font
        wc = defaultdict(int)    # width
        dc = defaultdict(int)    # direction
        jc = defaultdict(int)    # justify
        flc = defaultdict(int)   # flags
        rtl_narrow = 0

        for c in cases:
            a = c["args"]
            # fixed layout: -f FONT -w COLS -j JUST -D DIR [flags...] TEXT
            fc[a[1]] += 1
            wc[int(a[3])] += 1
            jc[a[5]] += 1
            dc[a[7]] += 1

            # flags are between index 8 and the last element (text)
            for i in range(8, len(a) - 1):
                flc[a[i]] += 1

            if a[7] == "right-to-left" and int(a[3]) in (10, 20):
                rtl_narrow += 1

        print(f"  Font:        {dict(sorted(fc.items()))}")
        print(f"  Width:       {dict(sorted(wc.items()))}")
        print(f"  Direction:   {dict(sorted(dc.items()))}")
        print(f"  Justify:     {dict(sorted(jc.items()))}")
        print(f"  Flags:       {dict(sorted(flc.items()))}")
        print(f"  RTL w/10|20: {rtl_narrow}")


if __name__ == "__main__":
    main()

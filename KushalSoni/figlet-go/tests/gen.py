#!/usr/bin/env python3
"""Generate frozen test cases for the figlet-go task.

Runs on the task author's machine (NOT in Docker), with the pyfiglet checkout from
Step 1 next to the task folder:

    work/
      pyfiglet/      <- git checkout 255827ef..., with pyfiglet/fonts/ set up
      figlet-go/     <- this task
        tests/gen.py

Usage:  python3 figlet-go/tests/gen.py   (or set PYFIGLET_DIR=/path/to/pyfiglet)

Every "out" is produced by running the ORIGINAL pyfiglet main() with the case's
args and capturing stdout byte for byte. Nothing is written by hand.
"""
from __future__ import annotations

import collections
import io
import json
import os
import random
import signal
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
TASK = HERE.parent
PYFIGLET_DIR = Path(os.environ.get("PYFIGLET_DIR", TASK.parent / "pyfiglet")).resolve()
PINNED_SHA = "255827efbc09d662875dad50fa0edc1e1263bf45"

N_VISIBLE = 2000
N_HIDDEN = 2000
CASE_TIMEOUT_SEC = 2

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script",
         # added: ivrit is the only standard font whose header says right-to-left
         # (exercises -D auto / -j auto); term is a full-width (no overlap) font.
         "ivrit", "term"]
WIDTHS = ["10", "20", "40", "60", "80", "120"]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = """hello world figlet smush kern harbor go port python twin exam letters giant
shout chat banner shadow script mini big small slant standard quick brown fox jumps
over lazy dog wizard jackdaw quartz sphinx vow black zebra mix vexing cozy hijack
lumpy waltz nymph AI GPU CPU ok OK yes NO Hi Hey 42 2026 3.14 v1.0 x y z""".split()

# Rare cases from NOTES.md: smushing-rule pairs, narrow letters, wrap edges.
TRICKY = [
    "|", "||", "|||", "|_|", "_|_", "/\\", "\\/", "//", "\\\\", "/_\\", "_/", "\\_",
    "[]", "][", "{}", "}{", "()", ")(", "<>", "><", "[|]", "{|}", "(|)", "<|>",
    "_[_]_", "_{_}_", "_(_)_", "_<_>_", "[{(<>)}]", "/|\\", "\\|/", "|/|\\|",
    "lll", "iii", "ili", "ll|ll", "WWW", "MMM", "HIH", "TT", "VV", "AV", "VA",
    "a_b", "x__y", "--", "==", "++", "::", ";;", "..", ",,", "''", '""', "``",
    "^^", "~~", "**", "##", "$$", "%%", "&&", "@@", "!!", "??",
    "$", "a$b", "  ", "   ", " a", "a ", "  a  b  ", "a  b", "a   b", " ",
    "Wq", "jj", "gg", "yy", "fi", "fl", "rn", "il1I|", "0O0", "8B8",
]
SYMBOLS = list("|/\\_[]{}()<>")
LATIN = ["é", "ß", "ñ", "café", "Straße", "niño", "ÄÖÜ", "äöü", "Grüße", "Größe",
         "Āā", "Ćć", "Čč", "Ěě", "Łł", "Œœ", "Šš", "Žž", "Ğğ"]
OTHER_SCRIPTS = ["αβγ", "ΑΒΓΔ", "Ωμέγα", "Привет", "МИР", "שלום", "אבג", "ЖЖ", "λx"]
MISSING = ["€", "漢字", "☃", "→", "\t", "a\tb", "🙂", "x€y", "\u00a0", "ü€ü"]
SENTENCES = [
    "the quick brown fox jumps over the lazy dog",
    "Pack my box with five dozen liquor jugs",
    "Sphinx of black quartz, judge my vow!",
    "hello world this line must wrap somewhere",
    "a b c d e f g h i j k l m n o p",
    "one  two   three    four",
    "|_| /\\/\\ [_] {_} (_) <_>",
    "wrap_me_without_any_spaces_please",
    "Grüße aus Köln und Düsseldorf",
    "right to left is the hardest part",
    "x/y\\z|w_v[u]t{s}r(q)p<o>n",
    "    leading spaces",
    "trailing spaces    ",
]


def load_pyfiglet():
    if not (PYFIGLET_DIR / "pyfiglet" / "__init__.py").is_file():
        sys.exit(f"pyfiglet checkout not found at {PYFIGLET_DIR} (set PYFIGLET_DIR)")
    if not (PYFIGLET_DIR / "pyfiglet" / "fonts" / "standard.flf").is_file():
        sys.exit("pyfiglet/fonts/ is missing. Run the Makefile setup first:\n"
                 "  mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/")
    head = PYFIGLET_DIR / ".git" / "HEAD"
    if head.is_file():
        try:
            import subprocess
            sha = subprocess.run(["git", "-C", str(PYFIGLET_DIR), "rev-parse", "HEAD"],
                                 capture_output=True, text=True, check=True).stdout.strip()
            if sha != PINNED_SHA:
                sys.exit(f"pyfiglet is at {sha}, expected {PINNED_SHA}")
        except (OSError, subprocess.CalledProcessError):
            pass
    sys.path.insert(0, str(PYFIGLET_DIR))
    import pyfiglet  # noqa: E402
    return pyfiglet


class Timeout(Exception):
    pass


def _alarm(signum, frame):
    raise Timeout()


def run_original(pyfiglet, args: list[str]) -> str | None:
    """Run pyfiglet.main() with argv = args. Return stdout, or None if it failed."""
    saved_argv, saved_stdout = sys.argv, sys.stdout
    sys.argv = ["pyfiglet"] + list(args)
    sys.stdout = io.TextIOWrapper(io.BytesIO(), encoding="utf-8")
    signal.signal(signal.SIGALRM, _alarm)
    signal.alarm(CASE_TIMEOUT_SEC)
    try:
        rc = pyfiglet.main()
        raw = sys.stdout.getvalue() if isinstance(sys.stdout, io.BytesIO) else None
    except (Timeout, Exception, SystemExit):
        rc, raw = 1, None
    finally:
        signal.alarm(0)
        sys.argv, sys.stdout = saved_argv, saved_stdout
    if rc != 0 or raw is None:
        return None
    return raw.decode("utf-8")


def pick_text(rng: random.Random) -> str:
    kind = rng.random()
    if kind < 0.22:
        return rng.choice(WORDS)
    if kind < 0.40:
        return rng.choice(TRICKY)
    if kind < 0.52:
        return "".join(rng.choice(SYMBOLS) for _ in range(rng.randint(2, 6)))
    if kind < 0.62:
        return rng.choice(SENTENCES)
    if kind < 0.70:
        return rng.choice(LATIN)
    if kind < 0.76:
        return rng.choice(OTHER_SCRIPTS)
    if kind < 0.82:
        return rng.choice(WORDS) + rng.choice(MISSING) + rng.choice(WORDS)
    if kind < 0.92:
        parts = [rng.choice(WORDS + TRICKY + LATIN + SYMBOLS) for _ in range(rng.randint(2, 5))]
        return rng.choice([" ", "  ", ""]).join(parts)
    # long sentence that must wrap
    return " ".join(rng.choice(WORDS) for _ in range(rng.randint(5, 12)))


def make_args(rng: random.Random) -> list[str]:
    args: list[str] = []
    opts: list[list[str]] = []
    if rng.random() < 0.92:
        opts.append(["-f", rng.choice(FONTS)])
    if rng.random() < 0.85:
        # bias towards narrow widths so wrapping and justification are exercised
        opts.append(["-w", rng.choice(WIDTHS + ["10", "20", "40"])])
    if rng.random() < 0.75:
        opts.append(["-j", rng.choice(JUSTIFY)])
    if rng.random() < 0.75:
        # bias towards right-to-left, the hardest path
        opts.append(["-D", rng.choice(DIRECTION + ["right-to-left"])])
    for flag in ("-r", "-F", "-s", "-n"):
        if rng.random() < 0.25:
            opts.append([flag])
    rng.shuffle(opts)
    for o in opts:
        args.extend(o)

    text = pick_text(rng)
    if text.startswith("-"):
        text = " " + text
    # sometimes pass the text as several arguments (main() joins them with spaces)
    # (never split off a part that starts with "-": optparse would read it as an option)
    parts = text.split(" ")
    if " " in text.strip() and rng.random() < 0.3 and not any(p.startswith("-") for p in parts):
        args.extend(parts)
    else:
        args.append(text)
    return args


def build(rng: random.Random, pyfiglet, n: int, seen: set[str], stats) -> list[dict]:
    cases: list[dict] = []
    while len(cases) < n:
        args = make_args(rng)
        key = json.dumps(args, ensure_ascii=False)
        if key in seen:
            continue
        out = run_original(pyfiglet, args)
        if out is None:
            stats["skipped (pyfiglet error/timeout)"] += 1
            continue
        seen.add(key)
        cases.append({"args": args, "out": out})
    return cases


def describe(name: str, cases: list[dict]) -> None:
    c = collections.Counter()
    for case in cases:
        a = case["args"]
        opt = dict(zip(a, a[1:]))
        rtl = opt.get("-D") == "right-to-left" or (opt.get("-D", "auto") == "auto" and opt.get("-f") == "ivrit")
        narrow = opt.get("-w") in ("10", "20")
        c["rtl"] += rtl
        c["narrow"] += narrow
        c["rtl+narrow"] += rtl and narrow
        c["multi-line output"] += case["out"].count("\n") > 12
        for flag in ("-r", "-F", "-s", "-n"):
            c[flag] += flag in a
        c["font=" + opt.get("-f", "standard")] += 1
    print(f"{name}: {len(cases)} cases", file=sys.stderr)
    for k in sorted(c):
        print(f"  {k:28s} {c[k]}", file=sys.stderr)


def main() -> int:
    pyfiglet = load_pyfiglet()
    random.seed(7)
    rng = random.Random(7)
    stats: collections.Counter = collections.Counter()
    seen: set[str] = set()
    visible = build(rng, pyfiglet, N_VISIBLE, seen, stats)
    hidden = build(rng, pyfiglet, N_HIDDEN, seen, stats)

    data = HERE / "data"
    data.mkdir(exist_ok=True)
    for name, cases in (("visible.jsonl", visible), ("hidden.jsonl", hidden)):
        with open(data / name, "w", encoding="utf-8", newline="\n") as fh:
            for case in cases:
                fh.write(json.dumps(case, ensure_ascii=True) + "\n")
        describe(name, cases)
    print(f"skipped: {dict(stats)}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Generate tests/data/visible.jsonl and tests/data/hidden.jsonl.

Runs on the task author's machine, NOT inside Docker. Every "out" value is the
exact stdout of the ORIGINAL pyfiglet main() for that case's args, captured as
bytes from a subprocess. Nothing is written by hand.

Layout expected (override with --pyfiglet PATH):
    <parent>/pyfiglet/          clone at commit PINNED_SHA, with pyfiglet/fonts/
    <parent>/<task folder>/tests/gen.py

Cases where pyfiglet does not finish cleanly (exception, non-zero exit, empty
stdout, or timeout) are dropped and replaced by the next draw. The instruction
says errors are not checked, so those cases carry no meaning.
"""

import argparse
import concurrent.futures as cf
import filecmp
import json
import os
import pathlib
import random
import subprocess
import sys

PINNED_SHA = "255827efbc09d662875dad50fa0edc1e1263bf45"
N_PER_SET = 2000
TIMEOUT_S = 30

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]
FLAGS = ["-r", "-F", "-s", "-n"]

WORDS = """hello world figlet go port ascii art banner quick brown fox jumps over the
lazy dog python gopher font kerning smush layout width wrap justify left right
center mirror flip text output byte exact match terminal column line glyph
Hello World ABC xyz Zebra Quiz 2024 v1.0 OK""".split()

# Rare cases from NOTES.md ("Rare cases" + smushing edge cases), as text:
#   big X pairs (/\ -> |, \/ -> Y, >< -> X), <> must NOT smush,
#   hardblank-looking '$', width-1 glyphs.
RARE = ["/\\", "\\/", "><", "<>", "/\\/\\", "\\/\\/", "><><", "<><>", "$", "$$",
        "a$b", "|", "||", "l|l", "i!i", ".", ":", "!!"]
# Every character that has a smushing rule.
SMUSH_CHARS = list("|/\\_[]{}()<>")
SMUSH_PAIRS = ["||", "__", "_|", "|_", "_/", "[]", "][", "{}", "}{", "()", ")(",
               "|(", "(|", "/]", "]/", "{)", "<(", "[_]", "{|}", "(<>)", "/_\\"]
LATIN1 = list("éßñÄÖÜäöüàçøÅÿ¡¿")
# Characters the fonts do not define (pyfiglet silently skips them).
MISSING = ["€", "漢", "✓", "😀", "Ω", "→", " "]


def draw_text(rng):
    kind = rng.randrange(8)
    if kind == 0:  # plain words
        return " ".join(rng.choice(WORDS) for _ in range(rng.randint(1, 3)))
    if kind == 1:  # long sentence that must wrap
        return " ".join(rng.choice(WORDS) for _ in range(rng.randint(8, 20)))
    if kind == 2:  # rare cases, alone or between words
        r = rng.choice(RARE)
        return rng.choice([r, f"{rng.choice(WORDS)} {r}", f"{r}{rng.choice(WORDS)}{r}"])
    if kind == 3:  # smushing characters and pairs
        parts = [rng.choice(SMUSH_PAIRS + SMUSH_CHARS) for _ in range(rng.randint(1, 4))]
        return rng.choice(["", " "]).join(parts)
    if kind == 4:  # double (and triple) spaces
        a, b = rng.choice(WORDS), rng.choice(WORDS)
        return rng.choice([f"{a}  {b}", f"{a}   {b}", f"{a}  {b}  {rng.choice(WORDS)}"])
    if kind == 5:  # Latin-1 letters
        w = list(rng.choice(WORDS))
        for _ in range(rng.randint(1, 3)):
            w.insert(rng.randint(0, len(w)), rng.choice(LATIN1))
        return "".join(w)
    if kind == 6:  # characters the font does not have
        w = list(rng.choice(WORDS))
        w.insert(rng.randint(0, len(w)), rng.choice(MISSING))
        return "".join(w)
    # mix of everything
    pool = WORDS + RARE + SMUSH_PAIRS + LATIN1 + MISSING
    return " ".join(rng.choice(pool) for _ in range(rng.randint(2, 6)))


def draw_case(rng):
    font = rng.choice(FONTS)
    width = rng.choice(WIDTHS)
    justify = rng.choice(JUSTIFY)
    direction = rng.choice(DIRECTION)
    flags = [f for f in FLAGS if rng.random() < 0.25]

    # Options: each a separate argument, all before the text, in random order.
    # An option equal to its default is sometimes left out, to exercise defaults.
    opts = []
    for flag, value, default in (("-f", font, "standard"), ("-w", str(width), "80"),
                                 ("-j", justify, "auto"), ("-D", direction, "auto")):
        if value == default and rng.random() < 0.5:
            continue
        opts.append([flag, value])
    opts += [[f] for f in flags]
    rng.shuffle(opts)

    text = draw_text(rng)
    # Never let text look like an option to optparse.
    if text.startswith("-"):
        text = "x" + text
    # Sometimes pass the text as several arguments (pyfiglet joins with one space).
    # Only when that does not change the text.
    if rng.random() < 0.3 and " " in text and text == " ".join(text.split(" ")) \
            and "" not in text.split(" "):
        words = text.split(" ")
    else:
        words = [text]
    if any(w.startswith("-") for w in words):
        words = [text]

    return [a for o in opts for a in o] + words


RUNNER = (
    "import sys\n"
    "root = sys.argv[1]\n"
    "sys.path.insert(0, root)\n"
    "import pyfiglet\n"
    "sys.argv = ['pyfiglet'] + sys.argv[2:]\n"
    "sys.exit(pyfiglet.main())\n"
)


def run_pyfiglet(root, args):
    """Return exact stdout of the original pyfiglet main(), or None if it failed."""
    try:
        p = subprocess.run([sys.executable, "-I", "-c", RUNNER, str(root), *args],
                           capture_output=True, timeout=TIMEOUT_S)
    except subprocess.TimeoutExpired:
        return None, "timeout"
    if p.returncode != 0 or not p.stdout:
        last = (p.stderr.decode("utf-8", "replace").strip().splitlines() or ["?"])[-1]
        return None, f"exit {p.returncode}: {last}"
    return p.stdout.decode("utf-8"), None


def preflight(root):
    pkg = root / "pyfiglet"
    if not (pkg / "__init__.py").is_file():
        sys.exit(f"no pyfiglet package at {pkg}")
    sha = subprocess.run(["git", "-C", str(root), "rev-parse", "HEAD"],
                         capture_output=True, text=True).stdout.strip()
    if sha != PINNED_SHA:
        sys.exit(f"{root} is at {sha or '?'}, expected {PINNED_SHA}")
    dirty = subprocess.run(["git", "-C", str(root), "status", "--porcelain", "--", "pyfiglet"],
                           capture_output=True, text=True).stdout.splitlines()
    dirty = [d for d in dirty if not d.endswith("pyfiglet/fonts/")]
    if dirty:
        sys.exit(f"pyfiglet sources modified, refusing:\n" + "\n".join(dirty))
    for f in FONTS:
        a, b = pkg / "fonts" / f"{f}.flf", pkg / "fonts-standard" / f"{f}.flf"
        if not a.is_file() or not filecmp.cmp(a, b, shallow=False):
            sys.exit(f"{a} missing or differs from fonts-standard. Run the font setup:\n"
                     f"  mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/")
    where = subprocess.run([sys.executable, "-I", "-c",
                            "import sys; sys.path.insert(0, sys.argv[1]); "
                            "import pyfiglet; print(pyfiglet.__file__)", str(root)],
                           capture_output=True, text=True).stdout.strip()
    if pathlib.Path(where).resolve() != (pkg / "__init__.py").resolve():
        sys.exit(f"imported pyfiglet from {where!r}, expected {pkg / '__init__.py'}")


def main():
    here = pathlib.Path(__file__).resolve().parent
    ap = argparse.ArgumentParser()
    ap.add_argument("--pyfiglet", type=pathlib.Path,
                    default=here.parent.parent / "pyfiglet",
                    help="path to the pyfiglet clone (default: ../pyfiglet next to the task folder)")
    ap.add_argument("--jobs", type=int, default=os.cpu_count() or 4)
    opt = ap.parse_args()
    root = opt.pyfiglet.resolve()
    preflight(root)

    rng = random.Random(7)
    random.seed(7)  # also seed the global RNG, as required
    seen = set()
    sets = {"visible": [], "hidden": []}
    dropped = {}

    with cf.ThreadPoolExecutor(opt.jobs) as pool:
        for name in ("visible", "hidden"):
            cases = sets[name]
            while len(cases) < N_PER_SET:
                # Draw a batch in the main thread (deterministic), run it in parallel,
                # keep results in draw order.
                batch = []
                while len(batch) < (N_PER_SET - len(cases)) + 50:
                    args = draw_case(rng)
                    key = json.dumps(args)
                    if key in seen:
                        continue
                    seen.add(key)
                    batch.append(args)
                for args, (out, err) in zip(batch, pool.map(lambda a: run_pyfiglet(root, a), batch)):
                    if out is None:
                        dropped[err] = dropped.get(err, 0) + 1
                        continue
                    if len(cases) < N_PER_SET:
                        cases.append({"args": args, "out": out})
                print(f"{name}: {len(cases)}/{N_PER_SET}", file=sys.stderr)

    data = here / "data"
    data.mkdir(exist_ok=True)
    for name, cases in sets.items():
        with open(data / f"{name}.jsonl", "w", encoding="utf-8", newline="\n") as f:
            for c in cases:
                f.write(json.dumps(c, ensure_ascii=False) + "\n")
        print(f"wrote {data / (name + '.jsonl')}: {len(cases)} cases", file=sys.stderr)
    if dropped:
        print("dropped (pyfiglet failed, replaced by new draws):", file=sys.stderr)
        for k, v in sorted(dropped.items(), key=lambda kv: -kv[1]):
            print(f"  {v:5d}  {k}", file=sys.stderr)


if __name__ == "__main__":
    main()

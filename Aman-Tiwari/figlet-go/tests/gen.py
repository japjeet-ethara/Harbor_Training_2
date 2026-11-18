"""Generate the frozen answer files for figlet-go.

Runs on the task author's laptop (NOT in Docker), with the pyfiglet checkout from
Step 1 (fonts already set up) next to the task folder:

    <dir>/pyfiglet      <- git checkout 255827efbc09d662875dad50fa0edc1e1263bf45
    <dir>/figlet-go     <- this task

    python figlet-go/tests/gen.py

Every "out" is the exact stdout of the ORIGINAL pyfiglet main() (`python -m pyfiglet`).
Nothing is ever written by hand. Cases on which pyfiglet crashes (non-zero exit) or
hangs are dropped, because the instruction says those are not checked.
"""

import json
import os
import random
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

HERE = Path(__file__).resolve().parent
PYFIGLET_DIR = Path(os.environ.get("PYFIGLET_DIR", HERE.parent.parent / "pyfiglet")).resolve()
DATA_DIR = HERE / "data"
N_PER_FILE = 2000
TIMEOUT_SEC = 5
WORKERS = 8
CHUNK = 400

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = [
    "hello", "world", "go", "port", "figlet", "Harbor", "test", "ASCII", "art", "big",
    "letters", "Quick", "brown", "fox", "jumps", "over", "lazy", "dog", "OK", "yes",
    "nice", "human", "shout", "Gopher", "Python", "twin", "exam", "MIT", "2026", "42",
    "x", "I", "il1", "WMW", "AVA", "Tj", "rnm", "mm", "VV", "ff", "q", "Zz", "0x7f",
]

# Rare cases from NOTES.md section 5, plus the smushing-rule probes.
TRICKY = [
    "/\\", "\\/", "><", "<>", "|_|", "||", "__", "_/", "/_", "|/", "\\|",
    "[]", "][", "{}", "}{", "()", ")(", "[_]", "{|}", "(<>)", "<_>", "/\\/\\",
    "\\\\//", ">-<", "^v^", "$$", "$", "ii", "i!", "l|l", ":;", ".,", "''", '""',
    "AVAV", "MWMW", "P/R", "b\\d", "`~", "@#%&*", "+=+", "?!", "a_b", "x|y",
    "\\_/", "/_\\", "|-|", "||| |||",
]
SYMBOLS = list("|/\\_[]{}()<>")
LATIN1 = ["é", "ß", "ñ", "Ä", "Ö", "Ü", "ä", "ö", "ü", "ç", "ø", "Å", "æ", "¡", "¿", "©", "°", "±", "½"]
MISSING = ["€", "☃", "中", "Ω", "😀", "\t", "\u2028", "→", "œ"]


def sentence(rng: random.Random, n: int) -> str:
    return " ".join(rng.choice(WORDS) for _ in range(n))


def make_text(rng: random.Random) -> str:
    kind = rng.random()
    if kind < 0.22:
        t = sentence(rng, rng.randint(1, 3))
    elif kind < 0.42:
        t = rng.choice(TRICKY)
        if rng.random() < 0.5:
            t = t + rng.choice(["", " ", "  "]) + rng.choice(TRICKY + WORDS)
    elif kind < 0.55:
        t = "".join(rng.choice(SYMBOLS) for _ in range(rng.randint(1, 6)))
        if rng.random() < 0.4:
            t = t.replace(rng.choice(SYMBOLS), " ")
    elif kind < 0.65:
        a, b = sentence(rng, 1), sentence(rng, rng.randint(1, 2))
        t = a + rng.choice(["  ", "   ", "  "]) + b
    elif kind < 0.80:
        t = sentence(rng, rng.randint(5, 12))  # long: must wrap
        if rng.random() < 0.3:
            t = t.replace(" ", "  ", 1)
    elif kind < 0.90:
        parts = [rng.choice(WORDS), "".join(rng.choice(LATIN1) for _ in range(rng.randint(1, 3)))]
        rng.shuffle(parts)
        t = rng.choice(["", " "]).join(parts)
    else:
        parts = [rng.choice(WORDS + TRICKY), rng.choice(MISSING)]
        if rng.random() < 0.5:
            parts.append(rng.choice(LATIN1 + SYMBOLS))
        rng.shuffle(parts)
        t = "".join(parts)
    if rng.random() < 0.1:
        t = " " + t
    if rng.random() < 0.1:
        t = t + " "
    return t


def text_args(rng: random.Random, text: str) -> list:
    # Sometimes pass the text as several arguments (pyfiglet joins them with one space).
    if " " in text.strip() and rng.random() < 0.3:
        args = [p for p in text.split(" ") if p != ""]
    else:
        args = [text]
    # An argument starting with "-" would be parsed as an option; avoid it.
    return [("~" + a[1:]) if a.startswith("-") else a for a in args]


def make_case(rng: random.Random) -> list:
    opts = []
    if rng.random() < 7 / 8:
        opts.append(["-f", rng.choice(FONTS)])
    if rng.random() < 7 / 8:
        opts.append(["-w", str(rng.choice(WIDTHS))])
    if rng.random() < 3 / 4:
        opts.append(["-j", rng.choice(JUSTIFY)])
    if rng.random() < 3 / 4:
        opts.append(["-D", rng.choice(DIRECTION)])
    for flag in ("-r", "-F", "-s", "-n"):
        if rng.random() < 1 / 4:
            opts.append([flag])
    rng.shuffle(opts)
    args = [a for o in opts for a in o]
    return args + text_args(rng, make_text(rng))


def run_original(args: list):
    """Exact stdout of the original pyfiglet main(), or None if it crashed or hung."""
    try:
        p = subprocess.run(
            [sys.executable, "-m", "pyfiglet", *args],
            cwd=PYFIGLET_DIR,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            timeout=TIMEOUT_SEC,
        )
    except subprocess.TimeoutExpired:
        return None
    if p.returncode != 0:
        return None
    return p.stdout.decode("utf-8")


def main() -> None:
    if not (PYFIGLET_DIR / "pyfiglet" / "fonts" / "standard.flf").is_file():
        sys.exit(f"pyfiglet with fonts set up not found at {PYFIGLET_DIR} (see NOTES.md, Step 1)")

    rng = random.Random(7)
    random.seed(7)
    seen = set()
    files = {"visible": [], "hidden": []}
    order = ["visible", "hidden"]
    dropped = 0

    with ThreadPoolExecutor(WORKERS) as pool:
        while len(files["hidden"]) < N_PER_FILE:
            batch = []
            while len(batch) < CHUNK:
                args = make_case(rng)
                key = json.dumps(args, ensure_ascii=False)
                if key in seen:
                    continue
                seen.add(key)
                batch.append(args)
            for args, out in zip(batch, pool.map(run_original, batch)):
                if out is None:
                    dropped += 1
                    continue
                target = next(name for name in order if len(files[name]) < N_PER_FILE)
                if target == "hidden" and len(files["hidden"]) >= N_PER_FILE:
                    break
                files[target].append({"args": args, "out": out})
                if len(files["hidden"]) >= N_PER_FILE:
                    break
            print(f"visible {len(files['visible'])}  hidden {len(files['hidden'])}  dropped {dropped}", flush=True)

    DATA_DIR.mkdir(exist_ok=True)
    for name, cases in files.items():
        with open(DATA_DIR / f"{name}.jsonl", "w", encoding="utf-8", newline="\n") as fh:
            for c in cases:
                fh.write(json.dumps(c, ensure_ascii=False) + "\n")

    for name, cases in files.items():
        def count(pred):
            return sum(1 for c in cases if pred(c["args"]))
        def val(a, flag):
            return a[a.index(flag) + 1] if flag in a[:-1] else None
        print(f"\n{name}: {len(cases)} cases")
        print("  -D right-to-left:", count(lambda a: val(a, "-D") == "right-to-left"))
        print("  width 10/20     :", count(lambda a: val(a, "-w") in ("10", "20")))
        print("  RTL and narrow  :", count(lambda a: val(a, "-D") == "right-to-left" and val(a, "-w") in ("10", "20")))
        for flag in ("-r", "-F", "-s", "-n"):
            print(f"  {flag:16}:", count(lambda a, f=flag: f in a))
        print("  fonts           :", {f: count(lambda a, f=f: val(a, "-f") == f) for f in FONTS})
        print("  empty output    :", sum(1 for c in cases if c["out"] == ""))
    print(f"\ndropped (pyfiglet crashed or hung): {dropped}")


if __name__ == "__main__":
    main()

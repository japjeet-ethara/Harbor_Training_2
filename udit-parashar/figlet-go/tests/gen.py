#!/usr/bin/env python3
"""Generate the frozen test cases for the figlet-go Harbor task.

Runs on the AUTHOR's laptop (never in Docker). Needs the original pyfiglet checkout
(Step 1) next to the task folder:

    <dir>/pyfiglet/      <- git clone + checkout 255827ef..., fonts/ filled from fonts-standard/
    <dir>/figlet-go/     <- this task

Every "out" is produced by running the ORIGINAL pyfiglet main() with the case's args and
capturing stdout bytes. No output is ever written by hand.

    python3 tests/gen.py            # writes tests/data/visible.jsonl and hidden.jsonl
"""
import io
import json
import os
import random
import shutil
import signal
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
PYFIGLET_REPO = Path(os.environ.get("PYFIGLET_DIR", HERE.parent.parent / "pyfiglet")).resolve()
DATA = HERE / "data"
N_PER_FILE = 2000

# ---- make sure the original is usable (Makefile "minimal" step) -------------
pkg = PYFIGLET_REPO / "pyfiglet"
if not (pkg / "fonts-standard").is_dir():
    sys.exit(f"pyfiglet checkout not found at {PYFIGLET_REPO}")
if not (pkg / "fonts").is_dir():
    (pkg / "fonts").mkdir()
    for f in (pkg / "fonts-standard").iterdir():
        shutil.copy(f, pkg / "fonts" / f.name)
sys.path.insert(0, str(PYFIGLET_REPO))
import pyfiglet  # noqa: E402  (the ORIGINAL)

assert Path(pyfiglet.__file__).resolve().parent == pkg, pyfiglet.__file__

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = ("hello world figlet go port smush kern pyfiglet Harbor task agent Shout giant letters "
         "Alpha beta GAMMA delta 42 2026 x y z A I O the quick brown fox jumps over lazy dog "
         "Wizard Quartz Jinx Vex mad boxing pack my box with five dozen liquor jugs").split()
PUNCT_WORDS = ["wow!", "a.b", "$5", "@home", "#1", "50%", "~tilde~", "a&b", "x=y", "^caret^",
               "`tick`", "'quote'", '"dq"', "a,b;c:d", "what?", "1+1=2", "*star*", "a-b", "under_score"]
TRICKY = [
    # smushing-rule traps
    "|_|", "_/\\_", "/\\/\\", "\\/", "/\\", "><", "<>", ">< <>", "[]{}()", ")(", "][", "}{", "|||",
    "___", "/|\\", "\\|/", "_|_", "(_)", "{_}", "[_]", "<_>", "|-|", "\\_/", "/_\\", "||_||",
    # spaces and blanks
    "  ", " ", "   ", "a  b", "  lead", "trail  ", "a   b   c", " x ", "ab  cd  ef",
    # newlines
    "x\ny", "\nstart", "end\n", "a\n\nb",
    # unicode
    "café", "ßtraße", "ñandú", "ÄÖÜäöü", "naïve", "Æon", "\u00a0nbsp",
    "日本", "😀", "€uro", "a→b", "tab\there", "zero\u200bwidth",
    # hardblank / endmark characters of the fonts
    "$", "$$", "@", "@@", "#", "##", "%", "!", "~", "a$b@c#d",
]
PUNCT_CHARS = list("|/\\_[]{}()<>")


def make_text(rng):
    r = rng.random()
    if r < 0.30:
        return " ".join(rng.choice(WORDS) for _ in range(rng.randint(1, 3)))
    if r < 0.45:
        return rng.choice(TRICKY)
    if r < 0.62:
        n = rng.randint(2, 10)
        s = "".join(rng.choice(PUNCT_CHARS) for _ in range(n))
        if rng.random() < 0.3:
            i = rng.randint(1, len(s))
            s = s[:i] + " " + s[i:]
        return s
    if r < 0.72:
        return rng.choice(WORDS) + " " + rng.choice(TRICKY) + " " + rng.choice(WORDS)
    if r < 0.82:
        return " ".join(rng.choice(WORDS + PUNCT_WORDS) for _ in range(rng.randint(5, 14)))
    if r < 0.88:
        return " ".join(rng.choice(PUNCT_WORDS) for _ in range(rng.randint(1, 3)))
    if r < 0.94:
        return rng.choice(["é", "ß", "ñ", "ü", "Ö", "ä", "É", "ç"]) + rng.choice(WORDS) + rng.choice(["é", "ß", "ñ"])
    # words glued to tricky chars
    return rng.choice(WORDS) + "".join(rng.choice(PUNCT_CHARS) for _ in range(rng.randint(1, 4))) + rng.choice(WORDS)


def make_args(rng):
    args = []
    if rng.random() < 0.85:
        args += ["-f", rng.choice(FONTS)]
    # narrow widths and right-to-left are the hard cases: weight them up
    if rng.random() < 0.9:
        args += ["-w", str(rng.choices(WIDTHS, weights=[3, 3, 3, 2, 2, 1])[0])]
    if rng.random() < 0.9:
        args += ["-j", rng.choice(JUSTIFY)]
    if rng.random() < 0.9:
        args += ["-D", rng.choices(DIRECTION, weights=[2, 2, 3])[0]]
    for flag in ("-r", "-F", "-s", "-n"):
        if rng.random() < 0.25:
            args.append(flag)
    text = make_text(rng)
    pieces = text.split(" ")
    can_split = (len(pieces) > 1 and all(p and not p.startswith("-") for p in pieces)
                 and rng.random() < 0.4)
    if text.startswith("-"):
        text = "x" + text
        can_split = False
    args += pieces if can_split else [text]
    return args


class CaseTimeout(BaseException):
    pass


def _on_alarm(signum, frame):
    raise CaseTimeout()


signal.signal(signal.SIGALRM, _on_alarm)


def run_original(args):
    """Run the ORIGINAL pyfiglet.main() exactly like the command line; return stdout bytes or None."""
    old_argv, old_out = sys.argv, sys.stdout
    buf = io.BytesIO()
    sys.argv = ["pyfiglet"] + args
    sys.stdout = io.TextIOWrapper(buf, encoding="utf-8")  # main() does sys.stdout.detach()
    try:
        # The original loops forever when a glyph is exactly as wide as -w; drop such cases.
        signal.setitimer(signal.ITIMER_REAL, 0.5)
        rc = pyfiglet.main()
        out = sys.stdout.getvalue() if isinstance(sys.stdout, io.BytesIO) else None
    except BaseException:  # CharNotPrinted (glyph wider than -w), optparse errors, ...
        return None
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        sys.argv, sys.stdout = old_argv, old_out
    if rc != 0 or out is None:
        return None
    return out


def main():
    rng = random.Random(7)  # == random.seed(7) but isolated
    random.seed(7)
    seen = set()
    cases = []
    skipped = 0
    while len(cases) < 2 * N_PER_FILE:
        args = make_args(rng)
        key = json.dumps(args)
        if key in seen:
            continue
        out = run_original(args)
        if out is None:
            skipped += 1
            continue
        seen.add(key)
        cases.append({"args": args, "out": out.decode("utf-8")})

    DATA.mkdir(parents=True, exist_ok=True)
    for name, chunk in (("visible", cases[:N_PER_FILE]), ("hidden", cases[N_PER_FILE:])):
        with open(DATA / f"{name}.jsonl", "w", encoding="utf-8", newline="\n") as f:
            for c in chunk:
                f.write(json.dumps(c, ensure_ascii=True) + "\n")

    # ---- coverage report -------------------------------------------------
    def stat(chunk):
        d = {"rtl": 0, "narrow<=20": 0, "rtl&narrow": 0, "-r": 0, "-F": 0, "-s": 0, "-n": 0}
        for c in chunk:
            a = c["args"]
            rtl = "right-to-left" in a
            nar = "-w" in a and int(a[a.index("-w") + 1]) <= 20
            d["rtl"] += rtl
            d["narrow<=20"] += nar
            d["rtl&narrow"] += rtl and nar
            for fl in ("-r", "-F", "-s", "-n"):
                d[fl] += fl in a
        return d
    print(f"skipped (original raised): {skipped}")
    for name, chunk in (("visible", cases[:N_PER_FILE]), ("hidden", cases[N_PER_FILE:])):
        print(name, len(chunk), stat(chunk))


if __name__ == "__main__":
    main()

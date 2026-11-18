"""Generate the frozen test cases for the figlet-go task.

Runs on the task author's laptop, NOT in Docker. Expects the pyfiglet checkout from Step 1
(with its font setup done) next to the task folder:

    harbor-practice-2/
        pyfiglet/      <- git checkout 255827efbc09d662875dad50fa0edc1e1263bf45 + fonts copied
        figlet-go/     <- this task

Usage (from the figlet-go folder):

    uv run --no-project --python 3.12 python tests/gen.py

Every "out" comes from running the ORIGINAL pyfiglet main() with the case's args.
No "out" is ever written by hand.
"""

import io
import json
import os
import random
import sys
from collections import Counter

HERE = os.path.dirname(os.path.abspath(__file__))
PYFIGLET_DIR = os.path.abspath(os.path.join(HERE, "..", "..", "pyfiglet"))
DATA_DIR = os.path.join(HERE, "data")
sys.path.insert(0, PYFIGLET_DIR)

import pyfiglet  # noqa: E402  (the original, from the checkout above)

N_PER_FILE = 2000
FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]

WORDS = [
    "hello", "world", "go", "figlet", "giant", "letters", "shout", "port", "twin", "exam",
    "harbor", "docker", "agent", "nice", "smush", "kern", "Hi", "OK", "Yes", "NO", "test",
    "ABC", "xyz", "Mixed", "Case", "123", "42", "2026", "v1.0", "a+b=c", "x*y", "50%", "#1",
    "e-mail", "it's", "\"quoted\"", "@home", "$5", "a&b", "Q?", "wow!", "~tilde", "`tick`",
    "semi;colon", "co:lon", "c,d", "end.",
]
# Rare cases from NOTES.md: places where a reader of the code is likely to get it wrong.
TRICKY = [
    "/\\", "\\/", "><", "<>", "|_|", "||", "__", "_/", "_|", "|/", "/[", "[{", "{(", "(<",
    "[]", "{}", "()", "/\\/\\", "\\/\\/", ">< ><", "[_]", "{_}", "(_)", "<_>", "|\\|", "/|\\",
    "\\_/", "/_\\", "_\\", "\\_", "]|[", "}{", ")(", "VW", "MW", "AV", "TT", "LT", "ll", "Il1",
    "mm", "WW", "ww", "y_y", "^v^", "o_O", "x/y\\z", "a|b|c",
]
PUNCT = "|/\\_[]{}()<>"
LATIN1 = ["café", "straße", "niño", "Ärger", "Öl", "Über", "äöü", "ÄÖÜß", "éàèç", "señor", "ñ", "ß",
          "Größe", "naïve", "façade", "Çà", "øå", "¿qué?", "¡olé!"]
MISSING = ["5€", "日本", "a★b", "☺", "→", "Ω", "😀", "ŁŻ", "∑x", "ĳ", "©", "°C", "±1"]


def choose_text(rng: random.Random) -> str:
    kind = rng.random()
    if kind < 0.20:
        text = " ".join(rng.choice(WORDS) for _ in range(rng.randint(1, 3)))
    elif kind < 0.40:
        text = " ".join(rng.choice(TRICKY) for _ in range(rng.randint(1, 3)))
    elif kind < 0.52:
        text = "".join(rng.choice(PUNCT) for _ in range(rng.randint(1, 8)))
    elif kind < 0.60:
        a, b = rng.choice(WORDS), rng.choice(WORDS + TRICKY)
        text = a + " " * rng.randint(2, 3) + b
    elif kind < 0.75:
        # long sentences that must wrap
        pool = WORDS + TRICKY + LATIN1
        text = " ".join(rng.choice(pool) for _ in range(rng.randint(6, 14)))
    elif kind < 0.87:
        text = " ".join(rng.choice(LATIN1 + WORDS) for _ in range(rng.randint(1, 3)))
    else:
        text = " ".join(rng.choice(MISSING + WORDS + TRICKY) for _ in range(rng.randint(1, 3)))
    if rng.random() < 0.05:
        text = " " + text
    if rng.random() < 0.05:
        text = text + " "
    return text


def text_to_args(rng: random.Random, text: str) -> list:
    # pyfiglet joins the text arguments with single spaces, so these are all the same text.
    if rng.random() < 0.5:
        return [text]
    return text.split(" ")


def choose_args(rng: random.Random) -> list:
    opts = []
    if rng.random() < 0.9:
        opts.append(["-f", rng.choice(FONTS)])
    if rng.random() < 0.9:
        opts.append(["-w", str(rng.choice(WIDTHS))])
    if rng.random() < 0.8:
        opts.append(["-j", rng.choice(JUSTIFY)])
    if rng.random() < 0.8:
        opts.append(["-D", rng.choice(DIRECTIONS)])
    for flag in ["-r", "-F", "-s", "-n"]:
        if rng.random() < 0.25:
            opts.append([flag])
    rng.shuffle(opts)
    args = [a for opt in opts for a in opt]
    words = text_to_args(rng, choose_text(rng))
    if any(w.startswith("-") for w in words):
        return None
    return args + words


class LoopGuard(Exception):
    pass


_steps = [0]
_orig_next = pyfiglet.FigletBuilder.goToNextChar


def _guarded_next(self):
    # pyfiglet never finishes when a glyph is exactly as wide as -w; stop it after many steps.
    _steps[0] += 1
    if _steps[0] > 200000:
        raise LoopGuard()
    return _orig_next(self)


pyfiglet.FigletBuilder.goToNextChar = _guarded_next


def run_original(args: list):
    """Run the original pyfiglet main() with these args. Return stdout as str, or None."""
    real_stdout, real_argv = sys.stdout, sys.argv
    buf = io.BytesIO()
    sys.stdout = io.TextIOWrapper(buf, encoding="utf-8", newline="")
    sys.argv = ["pyfiglet"] + args
    _steps[0] = 0
    try:
        code = pyfiglet.main()
    except (Exception, SystemExit):
        code = 1
    finally:
        sys.stdout = real_stdout
        sys.argv = real_argv
    if code != 0:
        return None
    return buf.getvalue().decode("utf-8")


def build(rng: random.Random, n: int, seen: set, banned_outs: set = frozenset()) -> list:
    cases = []
    while len(cases) < n:
        args = choose_args(rng)
        if args is None:
            continue
        key = json.dumps(args, ensure_ascii=False)
        if key in seen:
            continue
        out = run_original(args)
        if out is None:  # pyfiglet itself failed or never finished: not graded
            continue
        if out.strip() == "":  # blank drawing (e.g. only characters the font lacks):
            continue           # a cheat that prints just newlines would match it
        if out in banned_outs:  # hidden drawing identical to a visible one: a cheat could
            continue            # replay it under slightly different options

        seen.add(key)
        cases.append({"args": args, "out": out})
    return cases


def stats(name: str, cases: list) -> None:
    c = Counter()
    for case in cases:
        a = case["args"]
        for i, x in enumerate(a[:-1]):
            if x in ("-D", "-j", "-w", "-f"):
                c[x + " " + a[i + 1]] += 1
        for flag in ("-r", "-F", "-s", "-n"):
            if flag in a:
                c[flag] += 1
        if "-D" in a and a[a.index("-D") + 1] == "right-to-left" and "-w" in a and int(a[a.index("-w") + 1]) <= 40:
            c["rtl+narrow(<=40)"] += 1
    print(name, dict(sorted(c.items())))


def write(path: str, cases: list) -> None:
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        for case in cases:
            f.write(json.dumps(case, ensure_ascii=False) + "\n")


def main() -> None:
    random.seed(7)
    rng = random.Random(7)
    seen = set()
    visible = build(rng, N_PER_FILE, seen)
    hidden = build(rng, N_PER_FILE, seen, {c["out"] for c in visible})
    os.makedirs(DATA_DIR, exist_ok=True)
    write(os.path.join(DATA_DIR, "visible.jsonl"), visible)
    write(os.path.join(DATA_DIR, "hidden.jsonl"), hidden)
    stats("visible", visible)
    stats("hidden", hidden)


if __name__ == "__main__":
    main()

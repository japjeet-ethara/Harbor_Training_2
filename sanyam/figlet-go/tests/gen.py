#!/usr/bin/env python3
"""Build the answer sheets by running the ORIGINAL pyfiglet.

    python tests/gen.py

Runs on the author's machine (there is no Python in the task image) with the pyfiglet
checkout from Step 1 next to the task folder (../pyfiglet, fonts copied into
pyfiglet/fonts). Writes tests/data/visible.jsonl and tests/data/hidden.jsonl, 2000
distinct cases each, one JSON object per line: {"args": [...], "out": "..."}.
"out" is pyfiglet's stdout for those arguments, captured in-process; never hand-written.
Deterministic: random.seed(7), so two runs give identical files.
"""

import io
import json
import random
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
PYFIGLET = HERE.parent.parent / "pyfiglet"
sys.path.insert(0, str(PYFIGLET))
import pyfiglet  # noqa: E402

assert pyfiglet.__file__.startswith(str(PYFIGLET)), pyfiglet.__file__

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = """hello world figlet go port smush kern wrap wide narrow big small the quick brown fox
jumps over lazy dog Harbor task agent player room secret banner shout giant letters slash
underscore pipe bracket brace paren angle mirror flip strip normal left right center auto
one two three four five six seven eight nine ten zero Alpha Bravo Charlie Delta Echo Foxtrot
Zulu WWW MMM iii lll OK ok Hi HI a I x""".split()

# From NOTES.md "Rare cases": pieces where a single smushing rule decides the output.
TRICKY = ["||", "|_|", "/\\", "\\/", "[]", "][", "{}", "()", ")(", "<>", "><", "_/", "/_",
          "|/", "[|]", "(|)", "[]{}()<>", "__", "$$", "|", "/", "\\", "_", "[", "]", "{", "}",
          "(", ")", "<", ">", "!!", "..", "''", "\"\"", "``", "a  b", "a   b", " x", "x ",
          " ", "  ", "W", "WWWW", "MMMMMMMM", "@@", "##", "%%", "&&", "**", "++", ",,", "--",
          "::", ";;", "==", "??", "~~", "^^", "0_0", "|_|_|", "\\_/", "/_\\", "(_)", "[_]",
          "ÄÖÜ", "äöü", "ß", "é", "ñ", "ßé", "Ñandú", "café", "→", "日本", "a→b", "x日y",
          "", "e"]

SPLIT_ARGS = 0.2  # share of cases whose text is passed as several arguments


def text():
    kind = random.random()
    if kind < 0.40:
        return " ".join(random.sample(WORDS, random.randint(1, 5)))
    if kind < 0.70:
        return random.choice(TRICKY)
    if kind < 0.85:
        parts = [random.choice(TRICKY) if random.random() < 0.5 else random.choice(WORDS)
                 for _ in range(random.randint(2, 4))]
        return random.choice(["", " ", "  "]).join(parts)
    return " ".join(random.choice(WORDS) for _ in range(random.randint(8, 20)))


def case_args():
    args = ["-f", random.choice(FONTS), "-w", str(random.choice(WIDTHS))]
    if random.random() < 0.75:
        args += ["-j", random.choice(JUSTIFY)]
    if random.random() < 0.75:
        args += ["-D", random.choice(DIRECTION)]
    for flag in ("-r", "-F", "-s", "-n"):
        if random.random() < 0.25:
            args.append(flag)
    t = text()
    if t.startswith("-"):  # optparse would read it as an option
        t = "x" + t
    parts = t.split(" ")
    # a split token starting with "-" would be an option (or "--", dropped) for optparse
    if " " in t.strip() and random.random() < SPLIT_ARGS and not any(p.startswith("-") for p in parts):
        args += parts
    else:
        args.append(t)
    return args


class Hang(Exception):
    pass


MAX_STEPS = 100_000  # a real render takes a few hundred steps; the known infinite loop never ends


def guarded(self, _orig=pyfiglet.FigletBuilder.isNotFinished):
    self._steps = getattr(self, "_steps", 0) + 1
    if self._steps > MAX_STEPS:
        raise Hang()
    return _orig(self)


pyfiglet.FigletBuilder.isNotFinished = guarded


def run_original(args):
    """pyfiglet.main() with these argv, stdout captured as bytes. None if it never finishes."""
    buf = io.BytesIO()
    wrapper = io.TextIOWrapper(buf, encoding="utf-8")  # main() detaches it and writes bytes
    old_argv, old_stdout = sys.argv, sys.stdout
    sys.argv, sys.stdout = ["pyfiglet"] + args, wrapper
    try:
        pyfiglet.main()
    except Hang:
        return None
    except BaseException:  # CharNotPrinted, IndexError, SystemExit: nothing reached stdout
        pass
    finally:
        sys.argv, sys.stdout = old_argv, old_stdout
    out = buf.getvalue()
    del wrapper
    return out


def main():
    random.seed(7)
    seen = set()
    cases = []
    dropped = 0
    empty = 0
    while len(cases) < 4000:
        args = case_args()
        key = json.dumps(args, ensure_ascii=False)
        if key in seen:
            continue
        out = run_original(args)
        if out is None:
            dropped += 1
            continue
        if out == b"":  # pyfiglet crashed (glyph wider than -w): keep 1%, a silent binary gets these free
            empty += 1
            if empty > 40:
                continue
        seen.add(key)
        cases.append({"args": args, "out": out.decode("utf-8")})

    data = HERE / "data"
    data.mkdir(exist_ok=True)
    for name, chunk in (("visible", cases[:2000]), ("hidden", cases[2000:])):
        with open(data / f"{name}.jsonl", "w", encoding="utf-8", newline="\n") as f:
            for c in chunk:
                f.write(json.dumps(c, ensure_ascii=False) + "\n")

    rtl = sum(1 for c in cases if "right-to-left" in c["args"])
    narrow = sum(1 for c in cases if c["args"][3] in ("10", "20"))
    kept_empty = sum(1 for c in cases if c["out"] == "")
    print(f"{len(cases)} cases, {dropped} dropped (pyfiglet loops forever), {rtl} right-to-left, "
          f"{narrow} with -w 10/20, {kept_empty} with empty output kept of {empty}")


if __name__ == "__main__":
    main()

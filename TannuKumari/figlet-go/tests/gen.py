#!/usr/bin/env python3
"""Freeze pyfiglet's answers into tests/data/{visible,hidden}.jsonl.

Runs on the HOST (not in the task image), with the pinned pyfiglet checkout
sitting next to the task folder:

    <somewhere>/pyfiglet/          <- git checkout 255827ef..., fonts copied in
    <somewhere>/figlet-go/         <- this task
                tests/gen.py

    python3 tests/gen.py

Every "out" comes from the original implementation; nothing is written by hand.
The run is deterministic: same seed -> byte-identical files.
"""

from __future__ import annotations

import io
import json
import os
import random
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
TASK_DIR = HERE.parent
PYFIGLET_DIR = (TASK_DIR.parent / "pyfiglet").resolve()
DATA_DIR = HERE / "data"

N_VISIBLE = 2000
N_HIDDEN = 2000
SEED = 7

if not (PYFIGLET_DIR / "pyfiglet" / "__init__.py").is_file():
    sys.exit(f"pyfiglet checkout not found at {PYFIGLET_DIR}")
if not (PYFIGLET_DIR / "pyfiglet" / "fonts" / "standard.flf").is_file():
    sys.exit(
        f"{PYFIGLET_DIR}/pyfiglet/fonts/standard.flf missing — run the Makefile's\n"
        f"  mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/"
    )

sys.path.insert(0, str(PYFIGLET_DIR))
import pyfiglet  # noqa: E402  (the original, unmodified)

# Upstream bug: with a very narrow -w, handleNewLine falls through to
# cutBufferAtLastChar, which does `iterator -= 1` before it is re-incremented,
# so the same character is retried for ever. The guard counts STEPS, not
# seconds, so the generated files never depend on machine speed.
_MAX_BUILDER_STEPS = 5000
_orig_add_char = pyfiglet.FigletBuilder.addCharToProduct


def _counted_add_char(self):
    n = getattr(self, "_gen_steps", 0) + 1
    self._gen_steps = n
    if n > _MAX_BUILDER_STEPS:
        raise RuntimeError("pyfiglet builder did not terminate")
    return _orig_add_char(self)


pyfiglet.FigletBuilder.addCharToProduct = _counted_add_char

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = [
    "hello", "world", "figlet", "go", "port", "harbor", "agent", "docker",
    "smush", "kern", "width", "font", "banner", "ascii", "unicode", "test",
    "a", "ab", "abc", "Zz", "OK", "no", "yes", "Hi",
]

# every one of these exercises a branch a naive port gets wrong (see NOTES.md)
TRICKY = [
    "|_|", "_|_", "|||", "___", "_|", "|_",
    "/\\", "\\/", "><", "<>", "/\\/\\", "\\//\\",
    "[]", "{}", "()", ")(", "}{", "][", "[{()}]",
    "|/\\[]{}()<>", "_/\\_", "<|>", "|-|", "+-+",
    "iiii", "llll", "!!!!", "....", "''''", "::::", ",,,,", "````",
    "a  b", "a   b", " lead", "trail ", "  both  ",
    "O_o", "o_O", "^_^", "T_T", "=_=",
    "#@#@", "%%%", "&&&", "$$$", "***", "~~~",
    "AV", "WA", "Ty", "fj", "gq", "Wq",
    "é", "ß", "ñ", "ü", "Ä", "Ö", "Üß", "café", "jalapeño", "straße",
    "€", "☃", "中文", "日本", "→←", "αβγ",
    "the quick brown fox jumps over the lazy dog",
    "pack my box with five dozen liquor jugs",
    "a very long sentence that definitely has to wrap somewhere sensible",
    "supercalifragilisticexpialidocious",
    "WWWWWWWWWW", "iiiiiiiiii", "mmmmmmm",
    "1234567890", "0O0O0O", "1l1l1l",
    "", " ", "  ",
    "line1\nline2", "a\nb\nc", "x\n\ny",
    "tab\there", "semi;colon", "quote\"d", "back\\slash", "pipe|pipe",
]


class _StdoutShim(io.TextIOBase):
    """pyfiglet.main() calls sys.stdout.detach() and then writes bytes."""

    def __init__(self) -> None:
        self.buffer = io.BytesIO()

    def detach(self):  # type: ignore[override]
        return self.buffer

    def write(self, s):  # the --help path, never used here
        self.buffer.write(s.encode("UTF-8"))
        return len(s)


def run_original(args: list[str]) -> bytes | None:
    """Run the original pyfiglet main() and capture its stdout byte for byte.

    Returns None when the original itself fails (e.g. CharNotPrinted because the
    width cannot hold a single character) — those cases are simply not graded.
    """
    shim = _StdoutShim()
    old_stdout, old_argv = sys.stdout, sys.argv
    sys.stdout = shim  # type: ignore[assignment]
    sys.argv = ["pyfiglet"] + args
    try:
        rc = pyfiglet.main()
    except Exception:
        return None
    finally:
        sys.stdout = old_stdout
        sys.argv = old_argv
    if rc:
        return None
    return shim.buffer.getvalue()


def make_args(rng: random.Random) -> list[str]:
    args: list[str] = ["-f", rng.choice(FONTS)]

    if rng.random() < 0.75:
        args += ["-w", str(rng.choice(WIDTHS))]
    if rng.random() < 0.75:
        args += ["-j", rng.choice(JUSTIFY)]
    if rng.random() < 0.75:
        args += ["-D", rng.choice(DIRECTION)]

    for flag in ("-r", "-F", "-s", "-n"):
        if rng.random() < 0.25:
            args.append(flag)

    if rng.random() < 0.55:
        text = rng.choice(TRICKY)
    else:
        n = rng.randint(1, 3)
        text = " ".join(rng.choice(WORDS) for _ in range(n))

    # optparse would read a leading "-" as an option; never generate that
    if text.startswith("-"):
        text = "x" + text
    args.append(text)
    return args


def generate(
    rng: random.Random, n: int, seen: set[str]
) -> list[dict[str, object]]:
    cases: list[dict[str, object]] = []
    attempts = 0
    while len(cases) < n:
        attempts += 1
        if attempts > n * 50:
            sys.exit("generator gave up: too many failing combinations")
        args = make_args(rng)
        key = json.dumps(args)
        # a hidden case that also appears in the visible file would hand a
        # lookup-table cheat a free hidden match
        if key in seen:
            continue
        out = run_original(args)
        if out is None:
            continue
        seen.add(key)
        cases.append({"args": args, "out": out.decode("UTF-8")})
    return cases


def write(path: Path, cases: list[dict[str, object]]) -> None:
    with path.open("w", encoding="UTF-8", newline="\n") as fh:
        for case in cases:
            fh.write(json.dumps(case, ensure_ascii=False, sort_keys=True) + "\n")


def summarise(name: str, cases: list[dict[str, object]]) -> None:
    def count(pred) -> int:
        return sum(1 for c in cases if pred(c["args"]))

    rtl = count(lambda a: "right-to-left" in a)
    narrow = count(lambda a: "-w" in a and a[a.index("-w") + 1] in ("10", "20"))
    both = count(
        lambda a: "right-to-left" in a
        and "-w" in a
        and a[a.index("-w") + 1] in ("10", "20")
    )
    flags = {f: count(lambda a, f=f: f in a) for f in ("-r", "-F", "-s", "-n")}
    print(
        f"{name}: {len(cases)} cases | right-to-left {rtl} | narrow {narrow} | "
        f"narrow+rtl {both} | flags {flags}"
    )


def main() -> int:
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    os.chdir(PYFIGLET_DIR)  # pyfiglet resolves fonts relative to its package

    rng = random.Random(SEED)
    seen: set[str] = set()
    visible = generate(rng, N_VISIBLE, seen)
    hidden = generate(rng, N_HIDDEN, seen)

    write(DATA_DIR / "visible.jsonl", visible)
    write(DATA_DIR / "hidden.jsonl", hidden)

    summarise("visible", visible)
    summarise("hidden", hidden)

    overlap = len({json.dumps(c["args"]) for c in visible}
                  & {json.dumps(c["args"]) for c in hidden})
    print(f"identical arg lists in both files: {overlap}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

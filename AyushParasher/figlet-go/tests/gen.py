"""Generate the frozen answer files for figlet-go.

Runs on the task author's laptop (NOT in Docker), with the pyfiglet clone from
Step 1 next to the task folder:

    harbor-practice-2/
    ├── pyfiglet/     git clone at 255827ef, with pyfiglet/fonts set up
    └── figlet-go/    this task (tests/gen.py is this file)

Usage, from the task folder:
    uv run --no-project --python 3.12 python tests/gen.py

Writes tests/data/visible.jsonl and tests/data/hidden.jsonl, one JSON object per
line: {"args": [...], "out": "..."}. Every "out" is the exact stdout of the
ORIGINAL pyfiglet main(); nothing is ever written by hand. Cases where the
original raises (e.g. width too narrow) or hangs are skipped.
"""

from __future__ import annotations

import io
import json
import queue
import random
import subprocess
import sys
import threading
from pathlib import Path

TASK = Path(__file__).resolve().parent.parent
PYFIGLET = TASK.parent / "pyfiglet"
DATA = TASK / "tests" / "data"
PINNED_SHA = "255827efbc09d662875dad50fa0edc1e1263bf45"

N_VISIBLE = 2000
N_HIDDEN = 2000
CASE_TIMEOUT = 2.0  # seconds; real cases take milliseconds, so longer = a hang (skipped)

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTION = ["auto", "left-to-right", "right-to-left"]

WORDS = [
    "hello", "world", "go", "figlet", "port", "test", "giant", "letters", "shout",
    "chat", "python", "twin", "smush", "kern", "wrap", "mirror", "flip", "the", "a",
    "of", "Harbor", "OK", "Zebra", "quiz", "jump", "MiXeD", "UPPER", "lower", "123",
    "4.2", "v1.2", "x", "I", "WWW", "mm", "lil", "Hi", "yes", "no", "Q&A",
]

# Rare cases from NOTES.md, plus every smushing pair the fonts can hit.
TRICKY = [
    "|_|", "||", "__", "/\\", "\\/", "><", "<>", "[]", "][", "{}", "}{", "()", ")(",
    "|/", "_/", "\\_", "_|", "|_", "[{(<>)}]", "/\\/\\", "_|_", "|-|", "(_)", "{_}",
    "a  b", "hi  there", "x   y",                          # double/triple spaces
    "é", "ß", "ñ", "Ä Ö Ü", "äöü", "Straße", "café", "niño", "Ünïcödé", "ÄÖÜäöüß",
    "€", "中文", "✓", "→", "¿qué?", "çà",                    # chars many fonts lack
    "~", "`^'", "@#$%&*", "\"quotes\"", "!?.,;:", "0123456789", "+=*",
    "hello world", "W", "M", "x", "Wm", "ab", "ba", "$", "$$", "a$b",
]

SYMBOLS = "|/\\_[]{}()<>"


def make_text(rng: random.Random) -> str:
    kind = rng.random()
    if kind < 0.25:                      # a few normal words
        text = " ".join(rng.choice(WORDS) for _ in range(rng.randint(1, 3)))
    elif kind < 0.50:                    # a rare case on its own
        text = rng.choice(TRICKY)
    elif kind < 0.65:                    # a run of smushable symbols
        text = "".join(rng.choice(SYMBOLS) for _ in range(rng.randint(1, 8)))
    elif kind < 0.80:                    # a long sentence that must wrap
        text = " ".join(rng.choice(WORDS) for _ in range(rng.randint(5, 14)))
    else:                                # words mixed with rare cases
        parts = [rng.choice(WORDS), rng.choice(TRICKY), rng.choice(WORDS)]
        rng.shuffle(parts)
        text = rng.choice([" ", "  "]).join(parts)

    casing = rng.random()
    if casing < 0.15:
        text = text.upper()
    elif casing < 0.30:
        text = text.lower()
    return text


def text_args(rng: random.Random, text: str) -> list[str]:
    """Usually one argument; sometimes split into several (joined back with single spaces)."""
    if "  " not in text and " " in text and rng.random() < 0.25:
        parts = text.split(" ")
    else:
        parts = [text]
    # The instruction promises text args never start with "-" and are never empty.
    return [p if not p.startswith("-") else "x" + p for p in parts if p != ""] or ["x"]


def make_case(rng: random.Random) -> list[str]:
    opts: list[list[str]] = []
    if rng.random() < 7 / 8:
        opts.append(["-f", rng.choice(FONTS)])
    if rng.random() < 6 / 7:
        opts.append(["-w", str(rng.choice(WIDTHS))])
    if rng.random() < 4 / 5:
        opts.append(["-j", rng.choice(JUSTIFY)])
    if rng.random() < 4 / 5:
        opts.append(["-D", rng.choice(DIRECTION)])
    for flag in ("-r", "-F", "-s", "-n"):
        if rng.random() < 1 / 4:
            opts.append([flag])
    rng.shuffle(opts)
    args = [a for o in opts for a in o]
    return args + text_args(rng, make_text(rng))


# --------------------------------------------------------------------------
# Worker: runs the ORIGINAL pyfiglet.main() in-process, one case per line.
# Lives in a child process so a hanging case can be killed.
# --------------------------------------------------------------------------

def worker() -> None:
    sys.path.insert(0, str(PYFIGLET))
    import pyfiglet

    proto_in = sys.stdin.buffer
    proto_out = sys.stdout.buffer
    real_stdout = sys.stdout
    for line in proto_in:
        args = json.loads(line)
        buf = io.BytesIO()
        sys.stdout = io.TextIOWrapper(buf, encoding="utf-8")  # main() detaches this
        sys.argv = ["pyfiglet"] + args
        try:
            rc = pyfiglet.main()
            result = {"ok": rc == 0, "out": buf.getvalue().decode("utf-8")}
        except Exception as e:  # CharNotPrinted, FontError, ... -> skipped by parent
            result = {"ok": False, "err": f"{type(e).__name__}: {e}"}
        finally:
            sys.stdout = real_stdout
        proto_out.write(json.dumps(result).encode("utf-8") + b"\n")
        proto_out.flush()


class Runner:
    def __init__(self) -> None:
        self._start()

    def _start(self) -> None:
        self.proc = subprocess.Popen(
            [sys.executable, "-I", str(Path(__file__).resolve()), "--worker"],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE,
        )
        self.q: queue.Queue = queue.Queue()
        threading.Thread(target=self._read, args=(self.proc, self.q), daemon=True).start()

    @staticmethod
    def _read(proc, q) -> None:
        for line in proc.stdout:
            q.put(line)
        q.put(None)

    def run(self, args: list[str]) -> dict:
        self.proc.stdin.write(json.dumps(args).encode("utf-8") + b"\n")
        self.proc.stdin.flush()
        try:
            line = self.q.get(timeout=CASE_TIMEOUT)
        except queue.Empty:
            line = None
        if line is None:  # hang or crash: kill and restart the worker
            self.proc.kill()
            self.proc.wait()
            self._start()
            return {"ok": False, "err": "timeout"}
        return json.loads(line)

    def close(self) -> None:
        self.proc.stdin.close()
        self.proc.wait()


# --------------------------------------------------------------------------

def check_reference() -> None:
    if sys.version_info[:2] != (3, 12):
        sys.exit(f"gen.py must run on Python 3.12, not {sys.version.split()[0]}")
    sha = subprocess.run(["git", "-C", str(PYFIGLET), "rev-parse", "HEAD"],
                         capture_output=True, text=True).stdout.strip()
    if sha != PINNED_SHA:
        sys.exit(f"{PYFIGLET} is at {sha or '???'}, expected {PINNED_SHA}")
    fonts = PYFIGLET / "pyfiglet" / "fonts"
    std = PYFIGLET / "pyfiglet" / "fonts-standard"
    if not fonts.is_dir():
        sys.exit("pyfiglet/fonts is missing: run the Makefile font step from NOTES.md")
    names = lambda d: {p.name for p in d.iterdir() if p.is_file()}
    if names(fonts) != names(std) or any(
        (fonts / n).read_bytes() != (std / n).read_bytes() for n in names(std)
    ):
        sys.exit("pyfiglet/fonts differs from fonts-standard: redo the font step")


def write_jsonl(path: Path, cases: list[dict]) -> None:
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        for c in cases:
            f.write(json.dumps(c, ensure_ascii=False) + "\n")


def main() -> None:
    check_reference()
    random.seed(7)
    rng = random  # the seeded module-level generator: same cases on every run

    runner = Runner()
    seen: set[tuple[str, ...]] = set()
    visible: list[dict] = []
    hidden: list[dict] = []
    skipped = 0
    while len(hidden) < N_HIDDEN:
        args = make_case(rng)
        key = tuple(args)
        if key in seen:
            continue
        seen.add(key)  # visible and hidden never share the same args
        res = runner.run(args)
        if not res["ok"]:
            skipped += 1
            if res.get("err") == "timeout":
                print(f"  skipped a hang: {args}", file=sys.stderr, flush=True)
            continue
        case = {"args": args, "out": res["out"]}
        (visible if len(visible) < N_VISIBLE else hidden).append(case)
        done = len(visible) + len(hidden)
        if done % 500 == 0:
            print(f"progress: {done}/{N_VISIBLE + N_HIDDEN} cases", file=sys.stderr, flush=True)
    runner.close()

    DATA.mkdir(parents=True, exist_ok=True)
    write_jsonl(DATA / "visible.jsonl", visible)
    write_jsonl(DATA / "hidden.jsonl", hidden)

    def count(cases, pred):
        return sum(1 for c in cases if pred(c["args"]))
    rtl = lambda a: "right-to-left" in a
    narrow = lambda a: "-w" in a and int(a[a.index("-w") + 1]) <= 20
    print(f"visible={len(visible)} hidden={len(hidden)} skipped(error/timeout)={skipped}")
    for name, cases in (("visible", visible), ("hidden", hidden)):
        print(f"  {name}: right-to-left={count(cases, rtl)} narrow(<=20)={count(cases, narrow)} "
              f"rtl+narrow={count(cases, lambda a: rtl(a) and narrow(a))} "
              f"wrapped={sum(1 for c in cases if c['out'].count(chr(10)) > 12)}")


if __name__ == "__main__":
    if sys.argv[1:] == ["--worker"]:
        worker()
    else:
        main()

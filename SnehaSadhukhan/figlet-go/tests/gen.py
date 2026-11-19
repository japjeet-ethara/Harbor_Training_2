#!/usr/bin/env python3
"""Generate the visible and hidden FIGlet cases for the figlet-go task.

Runs on the author's machine (not inside Docker). The reference implementation
is the original pyfiglet repository located at ../pyfiglet relative to the
Harbor task folder (i.e. a sibling of figlet-go/).

Every expected output is produced by calling the original pyfiglet ``main()``
with the case's arguments and capturing exactly the bytes it writes to stdout.
Nothing is constructed or edited by hand.

Output (one JSON object per line, exactly two keys):
    tests/data/visible.jsonl   2000 cases
    tests/data/hidden.jsonl    2000 different cases
    {"args": [...], "out": "..."}
"""

import json
import queue
import random
import subprocess
import sys
import threading
from collections import Counter
from pathlib import Path

N_PER_FILE = 2000
PINNED_COMMIT = "255827efbc09d662875dad50fa0edc1e1263bf45"

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]
FLAGS = ["-r", "-F", "-s", "-n"]
FLAG_PROBABILITY = 0.25  # each flag enabled in 1 out of 4 cases

TASK_DIR = Path(__file__).resolve().parent.parent      # figlet-go/
REPO_DIR = TASK_DIR.parent / "pyfiglet"                 # ../pyfiglet
DATA_DIR = TASK_DIR / "tests" / "data"

# pyfiglet can loop forever on some glyphs exactly as wide as -w. A normal case
# finishes in milliseconds, so a candidate still running after this many seconds
# is treated as hanging: its worker process is killed and the candidate skipped.
CASE_TIMEOUT_SEC = 3.0

# ---------------------------------------------------------------------------
# Text material
# ---------------------------------------------------------------------------
WORDS = (
    "the quick brown fox jumps over lazy dog hello world go figlet ascii art "
    "font text banner small big shadow script mini slant code port python "
    "terminal width center right left harbor test case line wrap letter "
    "smush kern rule pipe slash bracket brace angle under score space"
).split()

SPECIAL = "|/\\_[]{}()<>"                       # smushing-rule characters
LATIN1 = "éßñÄÖÜäöüçàèìòùâêîôûëïÿøåæÆØÅ¡¿£©®°±µ¶»«"
NOT_IN_FONT_CANDIDATES = "€™←→↑↓★☃♥中文字ΩπЖλ✓"  # filtered per font at runtime

# Rare cases taken from NOTES.md, section 7 ("Things to keep in mind for the
# Go port") plus the pipeline notes in section 2 that it refers to.
NARROW = "il|.:!;,'`"          # "The smush 'width < 2' guard uses the previous/current letter widths."
GERMAN = "ÄÖÜäöüß"             # Section 2: German letters loaded after ASCII 126.


def rand_words(rng, lo, hi):
    words = [rng.choice(WORDS) for _ in range(rng.randint(lo, hi))]
    if rng.random() < 0.3:
        words = [w.capitalize() for w in words]
    return words


def text_normal(rng, font):
    text = " ".join(rand_words(rng, 1, 4))
    if rng.random() < 0.2:
        text += rng.choice(".!?")
    return text


def text_special(rng, font):
    parts = []
    for _ in range(rng.randint(1, 4)):
        if rng.random() < 0.6:
            parts.append("".join(rng.choice(SPECIAL) for _ in range(rng.randint(1, 6))))
        else:
            parts.append(rng.choice(WORDS))
    return rng.choice(["", " "]).join(parts)


def text_double_space(rng, font):
    words = rand_words(rng, 2, 5)
    out = words[0]
    for w in words[1:]:
        out += rng.choice(["  ", "  ", " ", "   "]) + w
    if "  " not in out:
        out = words[0] + "  " + " ".join(words[1:])
    return out


def text_long(rng, font):
    return " ".join(rand_words(rng, 10, 22))


def text_latin1(rng, font):
    words = rand_words(rng, 1, 3)
    out = []
    for w in words:
        pos = rng.randint(0, len(w))
        out.append(w[:pos] + "".join(rng.choice(LATIN1) for _ in range(rng.randint(1, 3))) + w[pos:])
    return " ".join(out)


def text_missing(rng, font, missing_by_font):
    candidates = missing_by_font[font]
    words = rand_words(rng, 1, 3)
    i = rng.randrange(len(words))
    w = words[i]
    pos = rng.randint(0, len(w))
    words[i] = w[:pos] + "".join(rng.choice(candidates) for _ in range(rng.randint(1, 3))) + w[pos:]
    return " ".join(words)


def text_rare(rng, font):
    kind = rng.choice(["space_glyph", "narrow", "newline", "german", "hardblank_justify"])
    if kind == "space_glyph":
        # NOTES: "keep the space glyph always" — text that is or starts/ends with spaces.
        return rng.choice([" ", "   ", " " + rng.choice(WORDS), rng.choice(WORDS) + " ",
                           " " + rng.choice(WORDS) + "  "])
    if kind == "narrow":
        # NOTES: the "width < 2" smush guard — narrow glyphs next to each other / to letters.
        s = "".join(rng.choice(NARROW) for _ in range(rng.randint(2, 6)))
        if rng.random() < 0.5:
            s = rng.choice(WORDS) + s + rng.choice(WORDS)
        return s
    if kind == "newline":
        # NOTES section 2: a "\n" in the text finishes the current line.
        return "\n".join(rand_words(rng, 2, 3))
    if kind == "german":
        # NOTES section 2 + "width is measured in characters (code points)".
        return "".join(rng.choice(GERMAN) for _ in range(rng.randint(1, 5)))
    # NOTES: "Hardblanks are replaced with spaces only at the very end" and
    # "right-justify uses width - len - 1" — multi-word text exercising both.
    return " ".join(rand_words(rng, 2, 4))


CATEGORIES = [
    ("normal", 0.25, text_normal),
    ("rare_notes", 0.15, text_rare),
    ("special_chars", 0.15, text_special),
    ("double_space", 0.10, text_double_space),
    ("long_wrap", 0.15, text_long),
    ("latin1", 0.10, text_latin1),
    ("not_in_font", 0.10, None),  # needs per-font data, handled below
]


# ---------------------------------------------------------------------------
# Reference runner: original pyfiglet main() in a worker process
# ---------------------------------------------------------------------------
WORKER_CODE = r"""
import io, json, sys
sys.dont_write_bytecode = True
sys.path.insert(0, sys.argv[1])
import pyfiglet

class _Capture:
    # pyfiglet.main() does `sys.stdout = sys.stdout.detach()` and then writes
    # raw bytes; detach() hands it this buffer so we capture the exact bytes.
    def __init__(self):
        self.buf = io.BytesIO()
    def detach(self):
        return self.buf
    def write(self, s):            # only used by non-rendering paths (help etc.)
        self.buf.write(s.encode("utf-8"))
    def flush(self):
        pass

proto_in = io.TextIOWrapper(sys.stdin.buffer, encoding="utf-8")
proto_out = sys.stdout
for line in proto_in:
    args = json.loads(line)
    cap = _Capture()
    sys.argv = ["pyfiglet"] + args
    sys.stdout = cap
    try:
        rc = pyfiglet.main()
        reply = {"status": "ok", "rc": rc, "out_b": cap.buf.getvalue().decode("latin-1")}
    except SystemExit as e:
        reply = {"status": "exit", "rc": e.code}
    except Exception as e:
        reply = {"status": "error", "type": type(e).__name__}
    finally:
        sys.stdout = proto_out
    proto_out.write(json.dumps(reply) + "\n")
    proto_out.flush()
"""


class Reference:
    """Runs pyfiglet.main() for each case in a long-lived worker process."""

    def __init__(self):
        self.proc = None
        self.replies = None
        self._start()

    def _start(self):
        self.proc = subprocess.Popen(
            [sys.executable, "-B", "-c", WORKER_CODE, str(REPO_DIR)],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            cwd=str(REPO_DIR),
        )
        self.replies = queue.Queue()
        threading.Thread(target=self._reader, args=(self.proc, self.replies), daemon=True).start()

    @staticmethod
    def _reader(proc, q):
        for line in proc.stdout:
            q.put(line)

    def run(self, args):
        self.proc.stdin.write((json.dumps(args) + "\n").encode("utf-8"))
        self.proc.stdin.flush()
        try:
            reply = json.loads(self.replies.get(timeout=CASE_TIMEOUT_SEC))
        except queue.Empty:
            # pyfiglet is hanging on this candidate: kill and reap the worker,
            # then start a fresh one for the next candidate.
            self._kill()
            self._start()
            return {"status": "timeout"}
        if reply.get("status") == "ok":
            reply["out"] = reply.pop("out_b").encode("latin-1").decode("utf-8")
        return reply

    def _kill(self):
        if self.proc is None:
            return
        if self.proc.poll() is None:
            self.proc.kill()
        self.proc.wait()
        # stdout is left to the reader thread, which ends on EOF now that the
        # process is gone.
        try:
            self.proc.stdin.close()
        except OSError:
            pass
        self.proc = None

    def close(self):
        if self.proc is None:
            return
        try:
            self.proc.stdin.close()
            self.proc.wait(timeout=30)
        except Exception:
            pass
        finally:
            self._kill()


# ---------------------------------------------------------------------------
# Case generation
# ---------------------------------------------------------------------------
def check_repo():
    head = subprocess.run(["git", "-C", str(REPO_DIR), "rev-parse", "HEAD"],
                          capture_output=True, text=True).stdout.strip()
    if head != PINNED_COMMIT:
        sys.exit(f"error: {REPO_DIR} is at {head or '?'}, expected {PINNED_COMMIT}")
    if not (REPO_DIR / "pyfiglet" / "fonts" / "standard.flf").is_file():
        sys.exit(f"error: {REPO_DIR / 'pyfiglet' / 'fonts'} is missing; copy fonts-standard into it first")


def load_fonts():
    sys.dont_write_bytecode = True
    sys.path.insert(0, str(REPO_DIR))
    import pyfiglet
    fonts = {name: pyfiglet.FigletFont(name) for name in FONTS}
    missing = {name: [ch for ch in NOT_IN_FONT_CANDIDATES if ord(ch) not in f.chars]
               for name, f in fonts.items()}
    return fonts, missing


def fits_width(font, text, width):
    """pyfiglet raises CharNotPrinted (no stdout, exit 1) for a glyph wider than
    the width, so such cases are not used. Glyphs exactly as wide as the width
    are allowed: some render normally and some make pyfiglet loop forever, and
    the reference run (with its timeout) decides which. This only selects
    inputs; it never produces expected output."""
    return all(font.width.get(ord(ch), 0) <= width for ch in text)


def has_equal_width_glyph(font, text, width):
    return any(font.width.get(ord(ch), 0) == width for ch in text)


def build_args(rng, missing_by_font):
    font = rng.choice(FONTS)
    width = rng.choice(WIDTHS)
    justify = rng.choice(JUSTIFY)
    direction = rng.choice(DIRECTIONS)
    flags = [f for f in FLAGS if rng.random() < FLAG_PROBABILITY]

    r, acc = rng.random(), 0.0
    for name, weight, fn in CATEGORIES:
        acc += weight
        if r < acc:
            break
    if name == "not_in_font":
        text = text_missing(rng, font, missing_by_font)
    else:
        text = fn(rng, font)

    # Sometimes pass the words as separate arguments (pyfiglet joins them with
    # single spaces); only when that does not change the text.
    if (rng.random() < 0.3 and "  " not in text and "\n" not in text
            and text == text.strip() and " " in text):
        text_args = text.split(" ")
    else:
        text_args = [text]

    args = ["-f", font, "-w", str(width), "-j", justify, "-D", direction] + flags + text_args
    return args, font, width, text, name


def generate():
    check_repo()
    fonts, missing_by_font = load_fonts()
    random.seed(7)
    rng = random  # module-level generator seeded with random.seed(7)

    ref = Reference()
    seen = set()
    files = {"visible": [], "hidden": []}
    stats = {k: Counter() for k in files}
    skipped = Counter()

    try:
        for split in ("visible", "hidden"):
            while len(files[split]) < N_PER_FILE:
                args, font, width, text, category = build_args(rng, missing_by_font)
                key = json.dumps(args, ensure_ascii=False)
                if key in seen:
                    skipped["duplicate"] += 1
                    continue
                if not fits_width(fonts[font], text, width):
                    skipped["glyph_wider_than_width"] += 1
                    continue
                equal = has_equal_width_glyph(fonts[font], text, width)
                reply = ref.run(args)
                if reply["status"] != "ok" or reply["rc"] != 0:
                    reason = f"reference_{reply['status']}"
                    skipped[reason + ("_equal_width" if equal else "")] += 1
                    continue
                seen.add(key)
                files[split].append({"args": args, "out": reply["out"]})
                stats[split][category] += 1
                if equal:
                    stats[split]["(equal_width_glyph)"] += 1
    finally:
        ref.close()

    DATA_DIR.mkdir(parents=True, exist_ok=True)
    for split, records in files.items():
        with open(DATA_DIR / f"{split}.jsonl", "w", encoding="utf-8", newline="\n") as fh:
            for rec in records:
                fh.write(json.dumps(rec, ensure_ascii=False) + "\n")

    for split in files:
        print(f"{split}: {len(files[split])} cases  categories={dict(sorted(stats[split].items()))}")
    print(f"skipped candidates: {dict(sorted(skipped.items()))}")


if __name__ == "__main__":
    generate()

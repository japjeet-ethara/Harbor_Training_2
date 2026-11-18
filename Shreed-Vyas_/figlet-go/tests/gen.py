#!/usr/bin/env python3
"""Generate visible.jsonl / hidden.jsonl from the REAL pyfiglet (pinned commit).

Run from the root of the cloned pyfiglet repo (fonts already copied to pyfiglet/fonts):
    cd pyfiglet && python3 ../figlet-go/tests/gen.py
(or set PYFIGLET_REPO=/path/to/pyfiglet clone)
"""
import json, os, random, signal, sys
from pathlib import Path

sys.path.insert(0, os.environ.get("PYFIGLET_REPO", os.getcwd()))
from pyfiglet import Figlet  # noqa: E402

random.seed(7)

HERE = Path(__file__).resolve().parent
DATA = HERE / "data"
ENV_DIR = HERE.parent / "environment"
N = 2000

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]
JUSTIFIES = ["auto", "left", "center", "right"]

WORDS = ["hello", "world", "figlet", "smush", "kerning", "Harbor", "Docker", "oracle", "Go",
         "pyfiglet", "Alpine", "banner", "slant", "test", "Zebra", "quick", "brown", "fox",
         "jumps", "over", "lazy", "dog", "MIXED", "Case", "123", "4567", "It's", "x=y+z"]
SPECIAL = "!@#$%^&*()_+-=[]{}|;:'\",.<>/?\\~`"
SMUSH_TRIGGERS = ["/\\", "\\/", "|/", "[]", "][", "{}", "()", "<>", "><", "_|", "||", "__",
                  "|_|", "/\\/\\", "[]{}()<>", "A/B\\C", "(a)[b]{c}<d>", "-_-", "...", "|-|"]
LATIN1 = ["é", "ß", "ñ", "ü", "ö", "ä", "É", "Ñ", "ç", "å", "¡", "¿", "©", "£"]
MISSING = ["€", "☃", "日", "本", "Ω", "Ж", "😀", "→", "™"]


def words(k):
    return " ".join(random.choice(WORDS) for _ in range(k))


def t_words():
    return words(random.randint(1, 3))


def t_special():
    return "".join(random.choice(SPECIAL) for _ in range(random.randint(2, 10)))


def t_double_space():
    return "  ".join(random.choice(WORDS) for _ in range(random.randint(2, 4)))


def t_long():
    return words(random.randint(8, 16))


def t_latin1():
    parts = [random.choice(WORDS)[:4] + random.choice(LATIN1) for _ in range(random.randint(1, 3))]
    return " ".join(parts)


def t_missing():
    parts = [random.choice(WORDS)[:3] + random.choice(MISSING) + random.choice(WORDS)[:2]
             for _ in range(random.randint(1, 3))]
    return " ".join(parts)


def t_smush():
    return " ".join(random.choice(SMUSH_TRIGGERS) for _ in range(random.randint(1, 3)))


def t_edge_spaces():
    return " " * random.randint(0, 2) + words(random.randint(1, 3)) + " " * random.randint(0, 2)


def make_text():
    kinds = [t_words, t_special, t_double_space, t_long, t_latin1, t_missing, t_smush, t_edge_spaces]
    weights = [20, 10, 10, 20, 10, 8, 14, 8]
    text = random.choices(kinds, weights)[0]()
    return text if text.strip() else "x"


def make_args():
    groups = []
    if random.random() < 0.92: groups.append(["-f", random.choice(FONTS)])
    if random.random() < 0.92: groups.append(["-w", str(random.choice(WIDTHS))])
    if random.random() < 0.70: groups.append(["-j", random.choice(JUSTIFIES)])
    if random.random() < 0.70: groups.append(["-D", random.choice(DIRECTIONS)])
    if random.random() < 0.25: groups.append(["-r"])
    if random.random() < 0.25: groups.append(["-F"])
    r = random.random()
    if r < 0.20: groups.append(["-s"])
    elif r < 0.45: groups.append(["-n"])
    elif r < 0.55: groups += [["-s"], ["-n"]]
    random.shuffle(groups)
    return [a for g in groups for a in g]


def parse(args):
    o = dict(font="standard", width=80, justify="auto", direction="auto",
             reverse=False, flip=False, nosmush=False)
    i = 0
    while i < len(args):
        a = args[i]
        if a == "-f": o["font"] = args[i + 1]; i += 1
        elif a == "-w": o["width"] = int(args[i + 1]); i += 1
        elif a == "-j": o["justify"] = args[i + 1]; i += 1
        elif a == "-D": o["direction"] = args[i + 1]; i += 1
        elif a == "-r": o["reverse"] = True
        elif a == "-F": o["flip"] = True
        elif a == "-s": o["nosmush"] = False      # last of -s / -n wins
        elif a == "-n": o["nosmush"] = True
        i += 1
    return o


def render(args, text):
    o = parse(args)
    fig = Figlet(font=o["font"], direction=o["direction"], justify=o["justify"], width=o["width"])
    widths = getattr(fig.Font, "width", None)
    if widths is not None:  # avoid pyfiglet's endless loop when a glyph is as wide as the line
        if max((widths.get(ord(c), 0) for c in text), default=0) >= o["width"]:
            raise ValueError("glyph too wide")
    if o["nosmush"]:
        fig.Font.smushMode = 0
    out = fig.renderText(text)
    if o["reverse"]:
        out = out.reverse()
    if o["flip"]:
        out = out.flip()
    return str(out) + "\n"          # CLI prints the result, adding one newline


class Timeout(Exception):
    pass


def _alarm(signum, frame):
    raise Timeout()


if hasattr(signal, "SIGALRM"):
    signal.signal(signal.SIGALRM, _alarm)


def try_render(args, text):
    if hasattr(signal, "alarm"):
        signal.alarm(5)
    try:
        return render(args, text)
    except Exception:
        return None
    finally:
        if hasattr(signal, "alarm"):
            signal.alarm(0)


def build(n, seen):
    cases, attempts = [], 0
    while len(cases) < n:
        attempts += 1
        if attempts > n * 60:
            sys.exit("too many rejected cases - is pyfiglet importable and are the fonts installed?")
        args, text = make_args(), make_text()
        key = (tuple(args), text)
        if key in seen:
            continue
        exp = try_render(args, text)
        if exp is None:
            continue
        seen.add(key)
        cases.append({"args": args, "text": text, "expected": exp})
    return cases


def write(path, prefix, cases):
    with open(path, "w", encoding="utf-8") as f:
        for i, c in enumerate(cases):
            f.write(json.dumps({"id": "%s-%04d" % (prefix, i), **c}) + "\n")
    print("wrote", len(cases), "cases ->", path)


if __name__ == "__main__":
    DATA.mkdir(parents=True, exist_ok=True)
    seen = set()
    visible = build(N, seen)
    hidden = build(N, seen)
    write(DATA / "visible.jsonl", "vis", visible)
    write(DATA / "hidden.jsonl", "hid", hidden)
    if ENV_DIR.is_dir():
        write(ENV_DIR / "visible.jsonl", "vis", visible)   # copied into the image for debugging

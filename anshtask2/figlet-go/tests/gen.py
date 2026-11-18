#!/usr/bin/env python3
"""Make cases and FREEZE answers by running the ORIGINAL pyfiglet main().
Runs on your laptop. Usage: python3 tests/gen.py [path-to-pyfiglet-repo]
(default: ../../pyfiglet i.e. the clone from Step 1, next to the task folder; fonts/ must exist there)
Deterministic: random.seed(7). 2000 visible + 2000 different hidden cases.
"""
import hashlib, io, json, os, random, signal, sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(sys.argv[1] if len(sys.argv) > 1 else os.path.join(HERE, "..", "..", "pyfiglet"))
sys.path.insert(0, REPO)
import pyfiglet  # the original = the truth
assert os.path.abspath(pyfiglet.__file__).startswith(REPO), "imported wrong pyfiglet: " + pyfiglet.__file__
assert os.path.isdir(os.path.join(REPO, "pyfiglet", "fonts")), \
    "run the font setup first: mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/"

random.seed(7)
N = 2000
FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUST = ["auto", "left", "center", "right"]
DIRS = ["auto", "left-to-right", "right-to-left"]
WORDS = ("hello world figlet shout giant letters smush kern slash quick brown fox jumps over lazy dog "
         "Harbor Go Python Zebra mix MAX min Title text ABC xyz 0123 42 !? ., ;: '\" ~ # @ & %").split()
SYMS = ["|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">", "|_|", "/\\", "\\/", "[]", "{}", "()", "<>",
        "/\\/\\", "_|_", "<<>>", "|||", "[[]]", "(){}", "_/", "\\_", "|/", ">|<", "[}", "{]", "(>", "<)"]
RARE = ["|_|", "/\\", "\\/", "[]{}()", "<>"]          # <- add your rare-cases list from NOTES.md here
LATIN = ["é", "ß", "ñ", "Ünï", "café", "Straße", "señor", "ÄÖÜäöü", "ÿ", "¿qué?"]
MISSING = ["€", "日本", "😀", "Ω", "→", "™", "a€b"]
LONG = ["the quick brown fox jumps over the lazy dog again and again",
        "pack my box with five dozen liquor jugs please", "a b c d e f g h i j k l m n o p"]


def text():
    r = random.random()
    if r < .35: t = random.choice(WORDS)
    elif r < .55: t = " ".join(random.choice(WORDS) for _ in range(random.randint(2, 4)))
    elif r < .70: t = "".join(random.choice(SYMS + RARE) for _ in range(random.randint(1, 4)))
    elif r < .78: t = random.choice(LATIN + MISSING) + random.choice(["", " " + random.choice(WORDS)])
    elif r < .86: t = random.choice(LONG)
    elif r < .92: t = random.choice(WORDS) + "  " + random.choice(WORDS)
    elif r < .96: t = random.choice(["", "\n"]) + random.choice(WORDS) + random.choice(["", "\n"])
    else: t = random.choice(RARE) + " " + random.choice(WORDS)
    return t


def case_args():
    font, width = random.choice(FONTS), random.choice(WIDTHS)
    just, direction = random.choice(JUST), random.choice(DIRS)
    groups = []
    if font != "standard" or random.random() < .5: groups.append(["-f", font])
    if width != 80 or random.random() < .3: groups.append(["-w", str(width)])
    if just != "auto" or random.random() < .2: groups.append(["-j", just])
    if direction != "auto" or random.random() < .2: groups.append(["-D", direction])
    for fl in ("-r", "-F", "-s", "-n"):
        if random.random() < .25: groups.append([fl])
    random.shuffle(groups)
    t = text()
    if not t.strip("\n ") or t.startswith("-"): return None
    words = t.split(" ") if random.random() < .15 and "\n" not in t and "  " not in t else [t]
    return [a for g in groups for a in g] + words


class Timeout(Exception): pass
def _alarm(*_): raise Timeout()
signal.signal(signal.SIGALRM, _alarm)


def original(args):
    """Run pyfiglet.main() exactly as the command line would; return stdout (str) or None."""
    old_argv, old_out = sys.argv, sys.stdout
    buf = io.BytesIO()
    w = io.TextIOWrapper(buf, encoding="utf-8", newline="")
    sys.argv, sys.stdout = ["pyfiglet"] + args, w
    signal.alarm(10)
    try:
        ret = pyfiglet.main()
        w.flush()
        data = buf.getvalue()
        return data.decode("utf-8") if ret in (None, 0) and data else None
    except BaseException:   # crashes, SystemExit, timeouts -> skip case
        return None
    finally:
        signal.alarm(0)
        sys.argv, sys.stdout = old_argv, old_out


cases, seen, skipped = [], set(), 0
while len(cases) < 2 * N:
    a = case_args()
    if a is None or tuple(a) in seen: continue
    seen.add(tuple(a))
    out = original(a)
    if out is None:
        skipped += 1
        continue
    cases.append({"args": a, "out": out})

os.makedirs(os.path.join(HERE, "data"), exist_ok=True)
h = hashlib.sha256()
for name, part in (("visible.jsonl", cases[:N]), ("hidden.jsonl", cases[N:])):
    with open(os.path.join(HERE, "data", name), "w", encoding="utf-8", newline="\n") as f:
        for c in part:
            line = json.dumps(c, ensure_ascii=False)
            f.write(line + "\n"); h.update(line.encode())
rtl = sum(1 for c in cases if "right-to-left" in c["args"])
narrow = sum(1 for c in cases if any(c["args"][i] == "-w" and int(c["args"][i + 1]) <= 20 for i in range(len(c["args"]) - 1)))
print(f"ok: {N} visible + {N} hidden | skipped(original crashed/empty)={skipped} | rtl={rtl} narrow(-w<=20)={narrow} | sha={h.hexdigest()[:16]}")

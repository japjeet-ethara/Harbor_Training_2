#!/usr/bin/env python3
import json
import random
import sys
from pathlib import Path

base_dir = Path(__file__).resolve().parent.parent
pyfiglet_dir = base_dir.parent / "pyfiglet"
if not pyfiglet_dir.exists():
    raise FileNotFoundError(f"pyfiglet repo not found next to task: {pyfiglet_dir}")

sys.path.insert(0, str(pyfiglet_dir))
sys.dont_write_bytecode = True
from pyfiglet import CharNotPrinted, Figlet

FONTS = ["standard", "slant", "small", "big", "mini", "banner", "shadow", "script"]
WIDTHS = [10, 20, 40, 60, 80, 120]
JUSTIFY = ["auto", "left", "center", "right"]
DIRECTIONS = ["auto", "left-to-right", "right-to-left"]
WORDS = [
    "hello", "world", "harbor", "agent", "twin", "machine", "shout", "letters",
    "golang", "python", "docker", "benchmark", "practice", "code", "runner",
]
TRICKY = [
    "|", "/", "\\", "_", "[", "]", "{", "}", "(", ")", "<", ">",
    "| / \\ _ [ ] { } ( ) < >", "/\\", "\\/", "><", "<>", "[]", "][",
    "{}", "}{", "()", ")(", "|_|", "/_\\", "[_]", "{_}", "<_>",
    "||", "//", "\\\\", "double  spaces  here", "   leading and trailing spaces   ",
    "a very long sentence that is guaranteed to wrap across multiple columns",
    "narrow wrap test for small widths", "café", "straße", "niño", "é ß ñ",
    "Ä Ö Ü ä ö ü ß", "emoji 😊 and greek α β γ", "special #$%^&*~`+=:;'\",.?!",
]


def render(args):
    options = {}
    toggles = set()
    text = None
    i = 0
    while i < len(args):
        arg = args[i]
        if arg in ("-f", "-w", "-j", "-D"):
            options[arg] = args[i + 1]
            i += 2
        elif arg in ("-r", "-F", "-s", "-n"):
            toggles.add(arg)
            i += 1
        else:
            text = " ".join(args[i:])
            break
    font = options.get("-f", "standard")
    width = int(options.get("-w", "80"))
    justify = options.get("-j", "auto")
    direction = options.get("-D", "auto")
    fig = Figlet(font=font, direction=direction, justify=justify, width=width)
    rendered = fig.renderText(text)
    if "-r" in toggles:
        rendered = rendered.reverse()
    if "-F" in toggles:
        rendered = rendered.flip()
    if "-s" in toggles:
        rendered = rendered.strip_surrounding_newlines()
    elif "-n" in toggles:
        rendered = rendered.normalize_surrounding_newlines()
    return str(rendered) + "\n"


def make_case(rng):
    font = rng.choice(FONTS)
    narrow_roll = rng.randrange(20)
    if narrow_roll == 0:
        width = 10
    elif narrow_roll == 1:
        width = 20
    else:
        width = rng.choice([40, 60, 80, 120])
    justify = rng.choice(JUSTIFY)
    direction = rng.choice(DIRECTIONS)
    text = rng.choice(WORDS + TRICKY)
    if rng.random() < 0.3:
        text = f"{rng.choice(WORDS)} {rng.choice(TRICKY)}"
    if width <= 20:
        font = "mini"
        text = "hi"
    elif width < 80 and len(text) > 24:
        text = rng.choice(WORDS)

    flags = ["-f", font, "-w", str(width), "-j", justify, "-D", direction]
    for flag in ("-r", "-F", "-s", "-n"):
        if rng.randrange(4) == 0:
            flags.append(flag)
    args = flags + [text]

    try:
        output = render(args)
    except CharNotPrinted:
        # A few long glyphs cannot fit the narrowest widths; retry with a
        # short word while keeping the selected font and options.
        text = "hi"
        args[-1] = text
        output = render(args)
    return {"args": args, "out": output}


def make_dataset(count, rng, excluded_args=None):
    excluded = excluded_args or set()
    cases = []
    while len(cases) < count:
        case = make_case(rng)
        signature = json.dumps(case["args"], ensure_ascii=False)
        if signature in excluded:
            continue
        cases.append(case)
    return cases


def main():
    random.seed(7)
    rng = random.Random(7)
    data_dir = Path(__file__).resolve().parent / "data"
    data_dir.mkdir(parents=True, exist_ok=True)

    visible = make_dataset(2000, rng)
    visible_args = {
        json.dumps(case["args"], ensure_ascii=False)
        for case in visible
    }
    hidden = make_dataset(2000, rng, visible_args)

    for name, cases in (("visible", visible), ("hidden", hidden)):
        path = data_dir / f"{name}.jsonl"
        with path.open("w", encoding="utf-8", newline="\n") as f:
            for case in cases:
                f.write(json.dumps(case, ensure_ascii=False) + "\n")
        print(f"Wrote {len(cases)} cases to {path}")


if __name__ == "__main__":
    main()

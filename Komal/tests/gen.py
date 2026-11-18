import argparse
from collections import Counter
from contextlib import redirect_stdout
from hashlib import sha256
from io import BytesIO, TextIOWrapper
from itertools import product
import json
from pathlib import Path
import random
import subprocess
import sys

FONTS = ("standard", "slant", "small", "big", "mini", "banner", "shadow", "script")
WIDTHS = (10, 20, 40, 60, 80, 120)
JUST = ("auto", "left", "center", "right")
DIRECTIONS = ("auto", "left-to-right", "right-to-left")
FLAGS = ("-r", "-F", "-s", "-n")
ORDINARY = (
    "A ferry to the island", "an early train", "walk across the bridge",
    "old harbor road", "tea with lime", "fresh bread from the bakery",
    "pack lunch for the trail", "roasted corn by the station",
    "rain on the hillside", "follow the river bend", "pine needles on the path",
    "shells on the beach", "a quiet lake at dawn", "bakery alley",
)
WORDS = ("ferry", "sunrise", "bus", "trail", "mango", "chai", "tide", "bread",
         "island", "forest", "heron", "mist", "cabin", "shore", "cloud", "lake")
MARKS = "|/\\_[]{}()<>"
LATIN = ("café", "crème brûlée", "piñata", "cañón", "Straße", "weiß", "jalapeño")
UNSUPPORTED = ("🫖", "🦜", "🪨", "界", "🚃")
SPACES = ("\u00a0", "\t", "\n", " ", "  ", "   ")


def capture(module, args):
    old_argv = sys.argv[:]
    buffer = BytesIO()
    stdout = TextIOWrapper(buffer, encoding="utf-8", newline="\n")
    try:
        with redirect_stdout(stdout):
            sys.argv = ["pyfiglet", *args]
            status = module.main()
        return status, buffer.getvalue().decode("utf-8")
    finally:
        sys.argv = old_argv


def edge_pairs(font):
    candidates = []
    symbols = MARKS + "mAVTWOjikyfgIYl$"
    for a, b in product(symbols, repeat=2):
        if ord(a) not in font.chars or ord(b) not in font.chars:
            continue
        for left, right in zip(font.chars[ord(a)], font.chars[ord(b)]):
            left, right = left.rstrip(" "), right.lstrip(" ")
            if left and right and (
                font.hardBlank in (left[-1], right[0])
                or left[-1] + right[0] in ("/\\", "\\/", "><", "[]", "{}", "()")
                or left[-1] == right[0]
            ):
                candidates.append(a + b)
                break
    return candidates


def phrase(family, geometry):
    word = random.choice(WORDS)
    mark = random.choice(MARKS)
    marks = "".join(random.choices(MARKS, k=random.randint(2, 10)))
    if family == "travel-food-nature":
        return random.choice(ORDINARY) + random.choice(("", "", " " + word))
    if family == "punctuation":
        return random.choice((marks, mark + word + marks, marks + " " + word))
    if family == "glyph-edges":
        a, b = random.choice(geometry)
        return random.choice((a + b, a + b + b + a, a * 2 + b * 2, word + " " + a + b))
    if family == "repetition":
        return random.choice("imVWlYTAf") * random.randint(2, 15) + random.choice(("", " " + word))
    if family == "spacing":
        return random.choice(SPACES) + word + random.choice(SPACES) + random.choice(WORDS)
    if family == "latin-1":
        return random.choice(LATIN) + " " + word
    if family == "unsupported":
        return word + random.choice(UNSUPPORTED) + random.choice(WORDS)
    return random.choice(ORDINARY) + "  " + random.choice(ORDINARY).lower() + " " + word


def freeze(module, directory):
    random.seed(7)
    directory.mkdir(parents=True, exist_ok=True)
    fonts = {name: module.FigletFont(name) for name in FONTS}
    geometry = {name: edge_pairs(font) or ["/\\"] for name, font in fonts.items()}
    families = ("travel-food-nature", "punctuation", "glyph-edges", "repetition",
                "spacing", "latin-1", "unsupported", "long-wrap")
    seen = set()
    manifest = {"seed": 7, "python": sys.version.split()[0], "splits": {}}
    for split in ("visible", "hidden"):
        combinations = list(product(FONTS, WIDTHS, JUST, DIRECTIONS))
        tail = random.sample(combinations, 2000 % len(combinations))
        slots = combinations * (2000 // len(combinations)) + tail
        random.shuffle(slots)
        flag_columns = []
        for _ in FLAGS:
            values = [True] * 500 + [False] * 1500
            random.shuffle(values)
            flag_columns.append(values)
        counts = {field: Counter() for field in
                  ("font", "width", "justify", "direction", "flags", "flag_combinations",
                   "narrow_rtl_by_width", "family", "rejections")}
        path = directory / (split + ".jsonl")
        with path.open("w", encoding="utf-8", newline="\n") as handle:
            for index, (name, width, justify, direction) in enumerate(slots):
                enabled = [flag for flag, values in zip(FLAGS, flag_columns) if values[index]]
                bool_args = enabled[:]
                random.shuffle(bool_args)
                groups = [["-f", name], ["-w", str(width)], ["-j", justify], ["-D", direction]]
                random.shuffle(groups)
                prefix = [item for group in groups for item in group] + bool_args
                font = fonts[name]
                for _ in range(300):
                    family = random.choice(families)
                    text = phrase(family, geometry[name])
                    if any(font.width.get(ord(ch), 0) >= width for ch in text if ch != "\n"):
                        counts["rejections"]["glyph_at_or_over_width"] += 1
                        continue
                    key = (name, width, justify, direction, tuple(enabled), text)
                    if key in seen:
                        counts["rejections"]["duplicate_semantic_arguments"] += 1
                        continue
                    args = [*prefix, "--", text]
                    try:
                        status, out = capture(module, args)
                    except (module.FigletError, IndexError, TypeError) as error:
                        counts["rejections"][type(error).__name__] += 1
                        continue
                    if status or not out.strip():
                        counts["rejections"]["nonzero_status_or_whitespace_only"] += 1
                        continue
                    seen.add(key)
                    break
                else:
                    raise RuntimeError(f"exhausted proposals for {name}, {width}, {justify}, {direction}")
                handle.write(json.dumps({"args": args, "out": out}, ensure_ascii=False) + "\n")
                for field, value in zip(("font", "width", "justify", "direction"),
                                        (name, width, justify, direction)):
                    counts[field][str(value)] += 1
                counts["flags"].update(enabled)
                counts["flag_combinations"][" ".join(enabled) or "none"] += 1
                counts["family"][family] += 1
                if width <= 20 and direction == "right-to-left":
                    counts["narrow_rtl_by_width"][str(width)] += 1
        manifest["splits"][split] = {"records": 2000, "sha256": sha256(path.read_bytes()).hexdigest(),
                                     **counts}
    manifest["disjoint_argument_vectors"] = len(seen) == 4000
    (directory / "coverage.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8", newline="\n")
    print(json.dumps(manifest, indent=2, sort_keys=True))


def main():
    parser = argparse.ArgumentParser()
    task = Path(__file__).resolve().parents[1]
    parser.add_argument("--upstream", type=Path, default=task.parent / "pyfiglet")
    parser.add_argument("--output", type=Path, default=task / "tests" / "data")
    args = parser.parse_args()
    upstream = args.upstream.resolve()
    sha = subprocess.check_output(["git", "-C", upstream, "rev-parse", "HEAD"], text=True).strip()
    if sha != "255827efbc09d662875dad50fa0edc1e1263bf45":
        parser.error(f"unexpected pyfiglet revision: {sha}")
    sys.dont_write_bytecode = True
    sys.path.insert(0, str(upstream))
    import pyfiglet
    freeze(pyfiglet, args.output)


if __name__ == "__main__":
    main()

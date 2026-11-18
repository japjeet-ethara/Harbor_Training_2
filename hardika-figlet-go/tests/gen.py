import argparse
from collections import Counter
from hashlib import sha256
from itertools import product
import io
import json
from pathlib import Path
import random
import subprocess
import sys

TASK = Path(__file__).resolve().parents[1]
COMMIT = "255827efbc09d662875dad50fa0edc1e1263bf45"
PYTHON = (3, 12, 13)
FONTS = ("standard", "slant", "small", "big", "mini", "banner", "shadow", "script")
WIDTHS = (10, 20, 40, 60, 80, 120)
JUSTIFICATIONS = ("auto", "left", "center", "right")
DIRECTIONS = ("auto", "left-to-right", "right-to-left")
FLAGS = ("-r", "-F", "-s", "-n")
LATIN = ("Café", "Straße", "Peña", "Noël", "Römer", "Dürer")
UNSUPPORTED = ("雪", "🦉", "জ", "λ", "𝄞")
PUNCTUATION = tuple("|/\\_[]{}()<>")
FAMILIES = (
    ("catalog", ("Atlas", "Index", "Folio", "Letters", "Studies", "Gazette"),
     ("plate", "volume", "colophon", "facsimile", "leaf", "edition")),
    ("exhibition", ("Portraits", "Ceramics", "Prints", "Textiles", "Lanterns", "Vessels"),
     ("gallery", "loan", "case", "wing", "label", "archive")),
    ("music", ("Nocturnes", "Sonatas", "Preludes", "Ballads", "Fugues", "Etudes"),
     ("recital", "quartet", "solo", "encore", "interlude", "score")),
    ("bibliography", ("Pamphlets", "Manuscripts", "Periodicals", "Treatises", "Journals", "Letters"),
     ("cited in", "revised by", "edited by", "appendix", "supplement", "concordance")),
    ("historical", ("Hypatia", "Ibn Sina", "Ada Lovelace", "Sappho", "Kepler", "Emmy Noether"),
     ("papers", "lectures", "fragments", "correspondence", "notebook", "chronicle")),
)
SCENARIOS = ("short", "paired", "edge", "wrap", "newlines", "omitted", "latin", "multiarg")


def load_reference(repo):
    head = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()
    if head != COMMIT:
        raise SystemExit(f"Expected upstream {COMMIT}, got {head}")
    subprocess.run(["git", "-C", str(repo), "diff", "--quiet", "HEAD", "--"], check=True)
    sys.path.insert(0, str(repo))
    import pyfiglet

    pkg = repo / "pyfiglet"
    if Path(pyfiglet.__file__).resolve().parent != pkg:
        raise SystemExit("pyfiglet was imported from outside the pinned checkout")
    for name in FONTS:
        prepared = pkg / "fonts" / f"{name}.flf"
        original = pkg / "fonts-standard" / prepared.name
        if not prepared.is_file() or prepared.read_bytes() != original.read_bytes():
            raise SystemExit(f"Prepare an exact copy of {original} under {pkg / 'fonts'}")
    return pyfiglet


def reference_output(reference, args):
    original_argv, original_stdout = sys.argv, sys.stdout
    data = io.BytesIO()
    stdout = io.TextIOWrapper(data, encoding="utf-8", newline="\n")
    try:
        sys.argv = ["pyfiglet", *args]
        sys.stdout = stdout
        status = reference.main()
        if sys.stdout is stdout:
            stdout.flush()
        if status != 0:
            raise ValueError(f"reference exited with {status}")
        return data.getvalue().decode("utf-8")
    finally:
        sys.argv, sys.stdout = original_argv, original_stdout


def scheduled(values, count):
    items = [values[index % len(values)] for index in range(count)]
    random.shuffle(items)
    return items


def designs():
    grid = list(product(FONTS, WIDTHS, JUSTIFICATIONS, DIRECTIONS))
    repeats, extra = divmod(2000, len(grid))
    remaining = zip(*(scheduled(levels, extra) for levels in (FONTS, WIDTHS, JUSTIFICATIONS, DIRECTIONS)))
    settings = grid * repeats + list(remaining)
    random.shuffle(settings)
    switches = [scheduled((True, False, False, False), 2000) for _ in FLAGS]
    return zip(settings, zip(*switches))


def text_parts(family, scenario):
    _, subjects, descriptors = family
    subject, descriptor = random.choice(subjects), random.choice(descriptors)
    latin, omitted = random.choice(LATIN), random.choice(UNSUPPORTED)
    separator = random.choice(PUNCTUATION)
    if scenario == "short":
        return [random.choice((subject, descriptor, latin, "A", "a", "III", "Xi"))]
    if scenario == "paired":
        pair = random.choice(("[]", "{}", "()", "><", "/\\", "\\/", "|_|", "_/"))
        return [f"{subject} {pair} {descriptor}"]
    if scenario == "edge":
        return [f"  {separator}{subject}  {descriptor}{separator}  "]
    if scenario == "wrap":
        return [f"{subject} and {descriptor} for the {latin} set {separator} a note on art"]
    if scenario == "newlines":
        return [f"\n{subject}{separator}{latin}\n\n {descriptor} {omitted}\n"]
    if scenario == "omitted":
        return [f"{subject}{omitted}{separator}{descriptor}"]
    if scenario == "latin":
        return [f"{latin} {separator} {subject}  {descriptor}"]
    return [subject, f"{descriptor}{separator}", latin]


def args_for(setting, switches, text):
    font, width, justify, direction = setting
    options = [["-f", font], ["-w", str(width)], ["-j", justify], ["-D", direction]]
    options.extend([flag] for flag, enabled in zip(FLAGS, switches) if enabled)
    random.shuffle(options)
    return [word for option in options for word in option] + ["--", *text]


def generate_split(name, reference, font_widths, used, directory, rejection_file):
    data_path = directory / f"{name}.jsonl"
    coverage = {key: Counter() for key in (
        "font", "width", "direction", "justification", "flags", "flag_combinations",
        "family", "scenario", "family_scenario", "features", "width_direction", "rejections",
    )}
    distinct_settings = set()
    mixes = scheduled(FAMILIES, 2000)
    scenarios = scheduled(SCENARIOS, 2000)
    with data_path.open("w", encoding="utf-8", newline="\n") as file:
        for (setting, switches), family, scenario in zip(designs(), mixes, scenarios):
            font, width, justify, direction = setting
            for _ in range(200):
                parts = text_parts(family, scenario)
                joined = " ".join(parts)
                args = args_for(setting, switches, parts)
                identity = (setting, switches, joined)
                reason = None
                if identity in used:
                    reason = "duplicate effective options and text across splits"
                elif any(font_widths[font].get(ord(ch), 0) >= width for ch in joined if ch != " "):
                    reason = "glyph meets or exceeds width and may fail or retry indefinitely"
                else:
                    try:
                        out = reference_output(reference, args)
                        if not out.strip():
                            reason = "reference output has no ink"
                    except (reference.FigletError, IndexError, TypeError, ValueError) as exc:
                        reason = f"reference execution failed: {type(exc).__name__}: {exc}"
                if reason is None:
                    used.add(identity)
                    file.write(json.dumps({"args": args, "out": out}, ensure_ascii=False) + "\n")
                    break
                coverage["rejections"][reason] += 1
                rejection_file.write(json.dumps({"split": name, "args": args, "reason": reason}, ensure_ascii=False) + "\n")
            else:
                raise RuntimeError(f"Unable to fill {name} stratum {setting}, {family[0]}, {scenario}")

            flags = [flag for flag, enabled in zip(FLAGS, switches) if enabled]
            coverage["font"][font] += 1
            coverage["width"][str(width)] += 1
            coverage["direction"][direction] += 1
            coverage["justification"][justify] += 1
            coverage["flags"].update(flags)
            coverage["flag_combinations"][" ".join(flags) or "(none)"] += 1
            coverage["family"][family[0]] += 1
            coverage["scenario"][scenario] += 1
            coverage["family_scenario"][f"{family[0]}/{scenario}"] += 1
            coverage["width_direction"][f"{width}/{direction}"] += 1
            coverage["features"].update(label for label, present in {
                "punctuation": any(ch in joined for ch in PUNCTUATION),
                "double_spaces": "  " in joined,
                "explicit_newlines": "\n" in joined,
                "latin_1": any(ch in joined for ch in "éßñëöü"),
                "unsupported_alongside_text": any(ch in joined for ch in UNSUPPORTED),
                "multiple_text_arguments": len(parts) > 1,
                "wrap_scenario": scenario == "wrap",
                "narrow_rtl": width <= 20 and direction == "right-to-left",
            }.items() if present)
            distinct_settings.add(setting)
    return {
        "cases": 2000,
        "sha256": sha256(data_path.read_bytes()).hexdigest(),
        "distinct_font_width_justification_direction": len(distinct_settings),
        **coverage,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--reference", type=Path, default=TASK.parent / "pyfiglet")
    parser.add_argument("--out", type=Path, default=TASK / "tests" / "data")
    parser.add_argument("--report", type=Path, default=TASK / "evidence" / "coverage.json")
    parser.add_argument("--rejections", type=Path, default=TASK / "evidence" / "generation-rejections.jsonl")
    args = parser.parse_args()
    if sys.version_info[:3] != PYTHON:
        raise SystemExit("Use CPython 3.12.13 for reproducible generation")
    reference = load_reference(args.reference.resolve())
    widths = {font: reference.FigletFont(font).width for font in FONTS}
    random.seed(7)
    used = set()
    for folder in (args.out, args.report.parent, args.rejections.parent):
        folder.mkdir(parents=True, exist_ok=True)
    report = {"seed": 7, "python": ".".join(map(str, PYTHON)), "upstream_commit": COMMIT, "splits": {}}
    with args.rejections.open("w", encoding="utf-8", newline="\n") as rejected:
        for name in ("visible", "hidden"):
            report["splits"][name] = generate_split(name, reference, widths, used, args.out, rejected)
    with args.report.open("w", encoding="utf-8", newline="\n") as report_file:
        json.dump(report, report_file, ensure_ascii=False, indent=2, sort_keys=True)
        report_file.write("\n")
    assert len(used) == 4000


if __name__ == "__main__":
    main()

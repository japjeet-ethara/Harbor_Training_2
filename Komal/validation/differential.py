import argparse
from collections import Counter
from difflib import unified_diff
from hashlib import sha256
from itertools import product
import json
from pathlib import Path
import random
import subprocess
import sys
import tempfile

TASK = Path(__file__).resolve().parents[1]
sys.dont_write_bytecode = True
sys.path.insert(0, str(TASK / "tests"))
from gen import MARKS, capture, edge_pairs


def read_split(split):
    path = TASK / "tests" / "data" / (split + ".jsonl")
    with path.open(encoding="utf-8") as stream:
        return [json.loads(line) for line in stream]


def targeted(module, upstream):
    rng = random.Random(516)
    for path in sorted((upstream / "pyfiglet" / "fonts-standard").glob("*.flf")):
        try:
            font = module.FigletFont(path.stem)
        except module.FigletError:
            yield path.stem, None
            continue
        geometry = edge_pairs(font)
        contacts = rng.sample(geometry, min(4, len(geometry)))
        examples = ("harbor tide", "Àéßñ🪨", " \n\nabc\n x ", "-bus [/]{}_<>", "WWa hii")
        for i, text in enumerate((*examples, *contacts)):
            directions = ("auto", "left-to-right", "right-to-left")
            alignments = ("auto", "center", "left", "right")
            flags = ([], ["-F", "-r"], ["-r", "-F", "-n"], ["-n", "-s"], ["-s", "-n"])
            max_glyph = max((font.width.get(ord(ch), 0) for ch in text), default=0)
            width = max(10 + i % 4, max_glyph + 2)
            args = ["-f", path.stem, "-D", directions[i % 3], "-j", alignments[i % 4],
                    f"-w{width}", *flags[i % 5], "--", text]
            yield path.stem, args
    for direction, suffix in product(("left-to-right", "right-to-left"), ("W", "T", "  a")):
        for text_width in (10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20):
            yield "standard", [
                "-f", "standard", "-w", str(text_width), "-j", "right", "-D", direction,
                "--", "jAA m ii" + suffix + "\n" + MARKS,
            ]
    for args in (["hello"], ["hello", "world"], ["-rFsn", "world"], [""],
                 ["-F", ""], ["-n", ""], ["-s", "界"], ["--", "-en-route"]):
        yield "standard", args


def compare(binary, cases, label, cwd):
    passed = 0
    for entry in cases:
        proc = subprocess.run([binary, *entry["args"]], capture_output=True, cwd=cwd, timeout=5)
        expected = entry["out"].encode("utf-8")
        if proc.returncode == 0 and proc.stdout == expected:
            passed += 1
            continue
        if compare.errors < 5:
            print("MISMATCH", label, repr(entry["args"]), "exit", proc.returncode)
            if proc.stderr:
                print(proc.stderr.decode(errors="replace"))
            print("\n".join(unified_diff(
                [repr(line) for line in expected.splitlines(keepends=True)],
                [repr(line) for line in proc.stdout.splitlines(keepends=True)],
                fromfile="upstream", tofile="Go", lineterm="",
            )))
        compare.errors += 1
    print(f"{label}: {passed}/{len(cases)} byte-for-byte matches")
    return passed == len(cases)


compare.errors = 0


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("binary", type=Path)
    parser.add_argument("--upstream", type=Path, default=TASK.parent / "pyfiglet")
    args = parser.parse_args()
    upstream = args.upstream.resolve()
    sys.path.insert(0, str(upstream))
    import pyfiglet
    binary = args.binary.resolve()
    splits = {name: read_split(name) for name in ("visible", "hidden")}
    visible_keys, hidden_keys = (set(tuple(row["args"]) for row in records)
                                 for records in splits.values())
    assert len(visible_keys) == len(hidden_keys) == 2000
    assert visible_keys.isdisjoint(hidden_keys)
    assert all(row["out"].strip() for records in splits.values() for row in records)
    print("Corpus audit: 4000 distinct vectors, zero whitespace-only answers")
    for split, index in (("visible", 17), ("hidden", 803), ("hidden", 1949)):
        entry = splits[split][index]
        result = subprocess.run([sys.executable, "-m", "pyfiglet", *entry["args"]],
                                cwd=upstream, capture_output=True, check=True, timeout=5)
        expected = entry["out"].encode("utf-8")
        assert result.stdout == expected
        print(f"Independent CLI {split}:{index + 1} matched SHA256 {sha256(expected).hexdigest()}")
    independent, rejects, supported_fonts = [], Counter(), set()
    for name, vector in targeted(pyfiglet, upstream):
        if vector is None:
            rejects[f"font parse error ({name})"] += 1
            continue
        font = pyfiglet.FigletFont(name)
        if any(font.width.get(ord(ch), 0) >= int(vector[6][2:] if vector[6].startswith("-w")
                                                else vector[3])
               for ch in vector[-1] if ch != "\n") if len(vector) >= 9 else False:
            rejects["glyph at or over width"] += 1
            continue
        try:
            status, out = capture(pyfiglet, vector)
        except (pyfiglet.FigletError, IndexError, TypeError) as error:
            rejects[type(error).__name__] += 1
            continue
        assert status == 0
        independent.append({"args": vector, "out": out})
        supported_fonts.add(name)
    print(f"Independent coverage: {len(supported_fonts)} standard fonts")
    print(f"Independent skips: {dict(rejects)}")
    with tempfile.TemporaryDirectory() as cwd:
        checks = [compare(binary, cases, split, cwd) for split, cases in splits.items()]
        checks.append(compare(binary, independent, "independently generated", cwd))
    return 0 if all(checks) else 1


if __name__ == "__main__":
    sys.exit(main())

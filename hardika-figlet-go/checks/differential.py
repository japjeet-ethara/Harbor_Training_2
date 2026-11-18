import argparse
import importlib.util
from itertools import product
import json
from pathlib import Path
import subprocess
import sys
import tempfile

TASK = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("generator", TASK / "tests" / "gen.py")
GENERATOR = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(GENERATOR)


def targeted(reference, repo):
    names = sorted(path.stem for path in (repo / "pyfiglet" / "fonts-standard").glob("*.flf"))
    for name in names:
        for direction, (width, text, flags) in product(
            ("auto", "left-to-right", "right-to-left"),
            (
                (30, "][ /\\ \\/ >< |_|", ("-r",)),
                (20, "\nÄéß   {ab}\nñ()雪  \n", ("-F", "-n")),
                (24, "AB CD \n \neta Écho", ("-s", "-n")),
                (40, "PmwA /^\\_!  1", ("-r", "-F")),
            ),
        ):
            args = ["-f", name, "-w", str(width), "-D", direction, "-j", "auto", *flags, "--", text]
            try:
                yield args, GENERATOR.reference_output(reference, args)
            except (reference.FigletError, IndexError, TypeError, ValueError) as exc:
                print(f"target upstream error {args!r} {type(exc).__name__}: {exc}")
    for args in (
        ["hello"],
        ["-fterm", "-w010", "ABCDEFGH"],
        ["-fterm", "-w0x8", "ABCDEFGH"],
        ["-fterm", "-w0b1000", "ABCDEFGH"],
        ["-fterm", "-w", " 010", "ABCDEFGH"],
        ["first", "-fterm", "second", "--", "-third"],
        ["-fterm", "-rFns", "A/\n_"],
        ["-fterm", "-r", ""],
        ["-fterm", "-n", "雪"],
    ):
        yield args, GENERATOR.reference_output(reference, args)


def compare(label, entries, candidate, cwd):
    cases, failures = 0, 0
    for args, expected in entries:
        run = subprocess.run([candidate, *args], cwd=cwd, capture_output=True, timeout=5)
        actual = run.stdout.decode("utf-8", "replace")
        cases += 1
        if run.returncode != 0 or run.stdout != expected.encode("utf-8"):
            failures += 1
            if failures <= 5:
                print(f"args: {args!r} exit: {run.returncode} stderr: {run.stderr!r}")
                print(f"expected: {expected!r}\nactual:   {actual!r}")
    print(f"{label}: {cases - failures}/{cases} byte-exact matches")
    return cases > 0 and failures == 0


def active_cases(path):
    with path.open(encoding="utf-8") as file:
        for line in file:
            case = json.loads(line)
            yield case["args"], case["out"]


def sample_reference(repo):
    locations = (("visible", 123), ("hidden", 902), ("hidden", 1548))
    for split, index in locations:
        args, expected = list(active_cases(TASK / "tests" / "data" / f"{split}.jsonl"))[index - 1]
        result = subprocess.run([sys.executable, "-B", "-m", "pyfiglet", *args],
                                cwd=repo, capture_output=True, check=True, timeout=5)
        if result.stdout != expected.encode("utf-8"):
            raise AssertionError(f"Reference mismatch at {split}:{index}")
        print(f"Independent python -m pyfiglet check matched {split}:{index} ({len(result.stdout)} bytes)")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("candidate", type=Path)
    parser.add_argument("--reference", type=Path, default=TASK.parent / "pyfiglet")
    opts = parser.parse_args()
    repo = opts.reference.resolve()
    reference = GENERATOR.load_reference(repo)
    candidate = str(opts.candidate.resolve())
    with tempfile.TemporaryDirectory() as cwd:
        corpus = [(name, active_cases(TASK / "tests" / "data" / f"{name}.jsonl")) for name in ("visible", "hidden")]
        corpus.append(("targeted across all standard fonts", targeted(reference, repo)))
        results = [compare(label, cases, candidate, cwd) for label, cases in corpus]
    sample_reference(repo)
    return 0 if all(results) else 1


if __name__ == "__main__":
    sys.exit(main())

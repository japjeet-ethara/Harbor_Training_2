import argparse
from collections import Counter
import itertools
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent
REFERENCE = ROOT.parent / "pyfiglet"
sys.path.insert(0, str(ROOT / "tests"))
from gen import SHA


def run_reference(binary, args):
    return subprocess.run([binary, "-m", "pyfiglet", *args], cwd=REFERENCE, capture_output=True,
                          timeout=5, check=True, env={**os.environ, "PYTHONDONTWRITEBYTECODE": "1"}).stdout


def targeted():
    yield from (["-f", font.stem, "-w", "120", "Agent"] for font in
                sorted((REFERENCE / "pyfiglet/fonts-standard").glob("*.flf")))
    for font, direction, punctuation in itertools.product(
        ("standard", "slant", "small", "big", "mini", "banner", "shadow", "script"),
        ("auto", "right-to-left"),
        ("\\/> <", "//\\\\", "||_|", "([{<>}])", "\n \n\n", "A雪B", "éßñ", "🚦"),
    ):
        yield ["-f", font, "-D", direction, "-w", "23", "-j", "center", punctuation, "ops"]
    for specials, switches in itertools.product(
        ("", "🦦", " \n ", "\n  begin\n\nend\n", " café  ñ ß ", " /\\ >< [] {} () _ || ",
         "api runs dry\nedge recovers", "λedge🧪"),
        ((), ("-r",), ("-F",), ("-s",), ("-n",), ("-r", "-F", "-n", "-s")),
    ):
        yield [*switches, specials]
    yield ["agent", "sync", "-rF", "-w0x40", "-fmini", "-Dright-to-left", "--", "-plain"]
    yield ["--font=small", "-w", "80", "--width=32", "-jleft", "--justify=center", "edge  service"]
    yield ["-w", "10", "-f", "mini", "-j", "right", "agent runner cache"]


def compare(program, args, expected):
    command = subprocess.run([program, *args], cwd="/tmp", capture_output=True, timeout=5)
    if command.returncode or command.stdout != expected:
        return {"args": args, "expected": repr(expected), "actual": repr(command.stdout),
                "status": command.returncode, "stderr": command.stderr.decode("utf-8", "replace")}
    return None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("program", type=Path)
    args = parser.parse_args()
    program = args.program.resolve()
    ref = sys.executable
    expected_sha = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=REFERENCE, text=True).strip()
    if expected_sha != SHA:
        raise SystemExit("unexpected reference revision")

    failed = 0
    for split in ("visible", "hidden"):
        cases = [json.loads(line) for line in (ROOT / "tests/data" / f"{split}.jsonl").read_text(
            encoding="utf-8").split("\n") if line]
        mismatches = 0
        for case in cases:
            difference = compare(program, case["args"], case["out"].encode())
            if difference:
                if failed + mismatches < 5:
                    print(json.dumps(difference, ensure_ascii=False))
                mismatches += 1
        print(f"{split}: {len(cases) - mismatches}/{len(cases)} exact", flush=True)
        failed += mismatches

    outcomes = Counter()
    for args in targeted():
        try:
            expected = run_reference(ref, args)
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as error:
            outcomes["reference rejected"] += 1
            print(f"reference rejected targeted input {args!r}: {error}")
            continue
        difference = compare(program, args, expected)
        if difference:
            if failed < 5:
                print(json.dumps(difference, ensure_ascii=False))
            failed += 1
            outcomes["mismatch"] += 1
        else:
            outcomes["exact"] += 1
    print(f"targeted: {outcomes['exact']} exact, {outcomes['mismatch']} mismatches, "
          f"{outcomes['reference rejected']} reference rejected")
    return bool(failed)


if __name__ == "__main__":
    sys.exit(main())

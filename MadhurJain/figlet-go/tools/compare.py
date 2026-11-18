#!/usr/bin/env python3
"""Compare output of the Go figlet port against expected test cases."""

import json
import subprocess
import sys
from pathlib import Path


def mark_line_ends(s: str) -> str:
    """Mark each line end with $ so trailing spaces show."""
    if not s:
        return ""
    # Replace \n with $\n; if s does not end with \n, append $
    marked = s.replace("\n", "$\n")
    if not s.endswith("\n"):
        marked += "$"
    return marked


def main():
    root_dir = Path(__file__).resolve().parent.parent
    test_file = root_dir / "tests" / "data" / "visible.jsonl"
    if len(sys.argv) > 1:
        test_file = Path(sys.argv[1]).resolve()

    if not test_file.exists():
        print(f"Error: test file not found: {test_file}", file=sys.stderr)
        sys.exit(1)

    # 1. Build solution/go to /tmp/figlet
    go_dir = root_dir / "solution" / "go"
    print(f"Building {go_dir} -> /tmp/figlet ...")
    build_res = subprocess.run(
        ["go", "build", "-o", "/tmp/figlet", "."],
        cwd=str(go_dir),
        capture_output=True,
        text=True,
    )
    if build_res.returncode != 0:
        print(f"Build failed:\n{build_res.stderr}", file=sys.stderr)
        sys.exit(1)
    print("Build succeeded.\n")

    # 2. Run cases
    total = 0
    matched = 0
    mismatched = 0
    errors = 0
    timeouts = 0
    diff_cases = []

    with open(test_file, "r", encoding="utf-8") as f:
        for line_no, line in enumerate(f, start=1):
            line = line.strip()
            if not line:
                continue
            case = json.loads(line)
            args = case["args"]
            expected = case["out"]
            total += 1

            try:
                proc = subprocess.run(
                    ["/tmp/figlet"] + args,
                    cwd="/tmp",
                    capture_output=True,
                    timeout=5,
                )
            except subprocess.TimeoutExpired:
                timeouts += 1
                if len(diff_cases) < 5:
                    diff_cases.append((args, expected, "<TIMEOUT (5s)>"))
                continue

            if proc.returncode != 0:
                errors += 1
                if len(diff_cases) < 5:
                    err_msg = f"<EXIT CODE {proc.returncode}>\nStderr: {proc.stderr.decode('utf-8', 'replace')}"
                    diff_cases.append((args, expected, err_msg))
                continue

            actual = proc.stdout.decode("utf-8")
            if actual == expected:
                matched += 1
            else:
                mismatched += 1
                if len(diff_cases) < 5:
                    diff_cases.append((args, expected, actual))

    print(f"Results for {test_file.name}:")
    print(f"  Total cases:  {total}")
    print(f"  Matched:      {matched} ({matched/total*100:.2f}%)" if total else "  Matched: 0")
    print(f"  Mismatched:   {mismatched}")
    print(f"  Errors:       {errors}")
    print(f"  Timeouts:     {timeouts}")

    if diff_cases:
        print(f"\nFirst {len(diff_cases)} differing cases:")
        for idx, (args, exp, act) in enumerate(diff_cases, start=1):
            print(f"\n--- Differing Case #{idx} ---")
            print(f"Args: {args}")
            print("Expected:")
            print(mark_line_ends(exp), end="" if exp.endswith("\n") else "\n")
            print("Actual:")
            print(mark_line_ends(act), end="" if act.endswith("\n") else "\n")


if __name__ == "__main__":
    main()

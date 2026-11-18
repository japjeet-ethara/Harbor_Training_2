#!/usr/bin/env python3
"""
Verification script for FIGlet dataset and spot checks against the pinned pyfiglet reference.
"""

import hashlib
import json
import os
import pathlib
import subprocess
import sys

EXPECTED_COMMIT = "255827efbc09d662875dad50fa0edc1e1263bf45"
EXPECTED_LINE_COUNT = 2000


def validate_file_format_and_lines(path: pathlib.Path) -> tuple[bytes, list[bytes]]:
    if not path.is_file():
        print(f"ERROR: Dataset file not found: {path}", file=sys.stderr)
        sys.exit(1)

    raw_bytes = path.read_bytes()

    # Strict UTF-8 check
    try:
        raw_bytes.decode("utf-8", errors="strict")
    except UnicodeDecodeError as e:
        print(f"ERROR: {path.name} is not valid UTF-8: {e}", file=sys.stderr)
        sys.exit(1)

    # Strict no BOM check
    if raw_bytes.startswith(b"\xef\xbb\xbf"):
        print(f"ERROR: {path.name} contains UTF-8 BOM", file=sys.stderr)
        sys.exit(1)

    # No physical CR bytes check
    if b"\r" in raw_bytes:
        print(f"ERROR: {path.name} contains physical CR (\\r) bytes", file=sys.stderr)
        sys.exit(1)

    # Final LF check
    if not raw_bytes.endswith(b"\n"):
        print(f"ERROR: {path.name} does not terminate with LF (\\n)", file=sys.stderr)
        sys.exit(1)

    # Exactly 2000 nonempty physical lines
    raw_lines = raw_bytes[:-1].split(b"\n")
    if len(raw_lines) != EXPECTED_LINE_COUNT:
        print(
            f"ERROR: {path.name} has {len(raw_lines)} lines, expected exactly {EXPECTED_LINE_COUNT}",
            file=sys.stderr,
        )
        sys.exit(1)

    for idx, line in enumerate(raw_lines, 1):
        if len(line) == 0:
            print(f"ERROR: {path.name} physical line {idx} is empty", file=sys.stderr)
            sys.exit(1)

    return raw_bytes, raw_lines


def parse_dataset_cases(raw_lines: list[bytes], filename: str, is_hidden: bool) -> list[dict]:
    cases = []
    seen_args = set()

    for idx, line_bytes in enumerate(raw_lines, 1):
        line_str = line_bytes.decode("utf-8")
        try:
            obj = json.loads(line_str)
        except Exception as e:
            msg = f"ERROR: {filename} line {idx} failed JSON parsing"
            if not is_hidden:
                msg += f": {e}"
            print(msg, file=sys.stderr)
            sys.exit(1)

        if not isinstance(obj, dict):
            print(f"ERROR: {filename} line {idx} is not a JSON object", file=sys.stderr)
            sys.exit(1)

        if set(obj.keys()) != {"args", "out"}:
            print(
                f"ERROR: {filename} line {idx} has invalid keys {sorted(obj.keys())}, expected exactly ['args', 'out']",
                file=sys.stderr,
            )
            sys.exit(1)

        args = obj["args"]
        out = obj["out"]

        if not isinstance(args, list) or not all(isinstance(a, str) for a in args):
            print(f"ERROR: {filename} line {idx} 'args' must be a list of strings", file=sys.stderr)
            sys.exit(1)

        if not isinstance(out, str):
            print(f"ERROR: {filename} line {idx} 'out' must be a string", file=sys.stderr)
            sys.exit(1)

        args_tuple = tuple(args)
        if args_tuple in seen_args:
            if is_hidden:
                print(
                    f"ERROR: {filename} line {idx} has duplicate args tuple within file",
                    file=sys.stderr,
                )
            else:
                print(
                    f"ERROR: {filename} line {idx} has duplicate args tuple: {json.dumps(args)}",
                    file=sys.stderr,
                )
            sys.exit(1)
        seen_args.add(args_tuple)

        cases.append({
            "line_no": idx,
            "args": args,
            "out": out,
            "args_tuple": args_tuple,
        })

    return cases


def parse_case_options(args: list[str]) -> tuple[dict, str]:
    if "--" in args:
        sep_idx = args.index("--")
        opt_tokens = args[:sep_idx]
        text_tokens = args[sep_idx + 1:]
    else:
        opt_tokens = []
        text_tokens = []
        for i, a in enumerate(args):
            if not a.startswith("-"):
                opt_tokens = args[:i]
                text_tokens = args[i:]
                break
        else:
            opt_tokens = args
            text_tokens = []

    text = " ".join(text_tokens)

    parsed = {
        "direction": None,
        "width": None,
        "strip": False,
        "normalize": False,
    }

    i = 0
    while i < len(opt_tokens):
        tok = opt_tokens[i]
        if tok == "--":
            break
        elif tok.startswith("--"):
            if "=" in tok:
                name, val = tok[2:].split("=", 1)
            else:
                name = tok[2:]
                val = None
            if name == "direction":
                if val is None and i + 1 < len(opt_tokens):
                    i += 1
                    val = opt_tokens[i]
                parsed["direction"] = val
            elif name == "width":
                if val is None and i + 1 < len(opt_tokens):
                    i += 1
                    val = opt_tokens[i]
                try:
                    parsed["width"] = int(val)
                except (ValueError, TypeError):
                    pass
            elif name == "strip-surrounding-newlines":
                parsed["strip"] = True
            elif name == "normalize-surrounding-newlines":
                parsed["normalize"] = True
        elif tok.startswith("-") and len(tok) > 1:
            opt_char = tok[1]
            if opt_char in ("D", "w", "f", "j", "c", "L"):
                if len(tok) > 2:
                    val = tok[2:]
                elif i + 1 < len(opt_tokens):
                    i += 1
                    val = opt_tokens[i]
                else:
                    val = None
                if opt_char == "D":
                    parsed["direction"] = val
                elif opt_char == "w":
                    try:
                        parsed["width"] = int(val)
                    except (ValueError, TypeError):
                        pass
            else:
                for c in tok[1:]:
                    if c == "s":
                        parsed["strip"] = True
                    elif c == "n":
                        parsed["normalize"] = True
        i += 1

    return parsed, text


def matches_cat_1(parsed: dict, text: str) -> bool:
    return parsed["direction"] == "right-to-left" and parsed["width"] in (10, 20)


def matches_cat_2(parsed: dict, text: str) -> bool:
    return parsed["strip"] and parsed["normalize"]


def matches_cat_3(parsed: dict, text: str) -> bool:
    return any(ord(c) > 127 for c in text)


def main() -> None:
    # Anchor all paths to __file__
    script_dir = pathlib.Path(__file__).resolve().parent
    clone_root = (script_dir / "pyfiglet").resolve()
    if not clone_root.is_dir() and (script_dir.parent / "pyfiglet").is_dir():
        clone_root = (script_dir.parent / "pyfiglet").resolve()

    data_dir = (script_dir / "figlet-go" / "tests" / "data").resolve()
    if not data_dir.is_dir() and (script_dir.parent / "figlet-go" / "tests" / "data").is_dir():
        data_dir = (script_dir.parent / "figlet-go" / "tests" / "data").resolve()

    if not clone_root.is_dir():
        print(f"ERROR: pyfiglet clone directory not found at {clone_root}", file=sys.stderr)
        sys.exit(1)

    if not data_dir.is_dir():
        print(f"ERROR: data directory not found at {data_dir}", file=sys.stderr)
        sys.exit(1)

    visible_path = data_dir / "visible.jsonl"
    hidden_path = data_dir / "hidden.jsonl"

    # Validate file format and physical lines for both files
    visible_raw, visible_lines = validate_file_format_and_lines(visible_path)
    hidden_raw, hidden_lines = validate_file_format_and_lines(hidden_path)

    # Parse and validate cases
    visible_cases = parse_dataset_cases(visible_lines, "visible.jsonl", is_hidden=False)
    hidden_cases = parse_dataset_cases(hidden_lines, "hidden.jsonl", is_hidden=True)

    # Check cross-file overlap
    visible_args_set = {c["args_tuple"] for c in visible_cases}
    hidden_args_set = {c["args_tuple"] for c in hidden_cases}
    overlap = visible_args_set.intersection(hidden_args_set)
    overlap_count = len(overlap)

    if overlap_count > 0:
        # Never print hidden arguments
        print(
            f"ERROR: Found {overlap_count} overlapping args tuples between visible and hidden datasets",
            file=sys.stderr,
        )
        sys.exit(1)

    # Print counts, overlap count and SHA-256 hashes
    visible_sha = hashlib.sha256(visible_raw).hexdigest()
    hidden_sha = hashlib.sha256(hidden_raw).hexdigest()

    print(f"Visible cases count: {len(visible_cases)}")
    print(f"Hidden cases count: {len(hidden_cases)}")
    print(f"Overlap count: {overlap_count}")
    print(f"visible.jsonl SHA-256: {visible_sha}")
    print(f"hidden.jsonl SHA-256: {hidden_sha}")

    # Spot checks preparation
    # 1. Require successful git rev-parse HEAD in pyfiglet clone and exact commit SHA
    try:
        proc = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=str(clone_root),
            capture_output=True,
            text=True,
            timeout=5,
        )
    except Exception as e:
        print(f"ERROR: git rev-parse HEAD failed: {e}", file=sys.stderr)
        sys.exit(1)

    if proc.returncode != 0:
        print(
            f"ERROR: git rev-parse HEAD exited with code {proc.returncode}: {proc.stderr.strip()}",
            file=sys.stderr,
        )
        sys.exit(1)

    actual_commit = proc.stdout.strip()
    if actual_commit != EXPECTED_COMMIT:
        print(
            f"ERROR: Git HEAD mismatch in pyfiglet: expected {EXPECTED_COMMIT}, got {actual_commit}",
            file=sys.stderr,
        )
        sys.exit(1)

    print(f"Verified pyfiglet commit SHA: {actual_commit}")

    # 2. Verify original import path in child process resolves to clone's pyfiglet/__init__.py
    child_env = os.environ.copy()
    child_env["PYTHONUTF8"] = "1"
    child_env["PYTHONDONTWRITEBYTECODE"] = "1"

    expected_init = (clone_root / "pyfiglet" / "__init__.py").resolve()
    init_cmd = [
        sys.executable,
        "-B",
        "-c",
        "import pathlib, pyfiglet; print(pathlib.Path(pyfiglet.__file__).resolve())",
    ]

    try:
        proc = subprocess.run(
            init_cmd,
            cwd=str(clone_root),
            env=child_env,
            capture_output=True,
            text=True,
            timeout=5,
        )
    except Exception as e:
        print(f"ERROR: pyfiglet import check failed: {e}", file=sys.stderr)
        sys.exit(1)

    if proc.returncode != 0:
        print(
            f"ERROR: pyfiglet import check exited with code {proc.returncode}: {proc.stderr.strip()}",
            file=sys.stderr,
        )
        sys.exit(1)

    if proc.stderr:
        print(
            f"ERROR: pyfiglet import check produced stderr: {proc.stderr.strip()}",
            file=sys.stderr,
        )
        sys.exit(1)

    actual_init = pathlib.Path(proc.stdout.strip()).resolve()
    if actual_init != expected_init:
        print(
            f"ERROR: pyfiglet import path mismatch: expected {expected_init}, got {actual_init}",
            file=sys.stderr,
        )
        sys.exit(1)

    print(f"Verified pyfiglet import path: {actual_init}")

    # Spot check selection
    # Exactly three DISTINCT visible cases:
    # 1. One explicit -D right-to-left case with -w 10 or 20.
    # 2. One case with both -s and -n enabled.
    # 3. One case containing a non-ASCII character in its text.
    # Select first matching unused case for each category.
    categories = [
        ("explicit -D right-to-left with -w 10 or 20", matches_cat_1),
        ("both -s and -n enabled", matches_cat_2),
        ("non-ASCII character in text", matches_cat_3),
    ]

    selected_cases = []
    used_indices = set()

    for cat_name, matcher in categories:
        found_case = None
        for idx, case in enumerate(visible_cases):
            if idx in used_indices:
                continue
            parsed, text = parse_case_options(case["args"])
            if matcher(parsed, text):
                used_indices.add(idx)
                found_case = case
                selected_cases.append((cat_name, case))
                break
        if found_case is None:
            print(
                f"ERROR: Required category '{cat_name}' is absent in visible cases",
                file=sys.stderr,
            )
            sys.exit(1)

    assert len(selected_cases) == 3, "Expected exactly 3 selected cases"
    assert len({c["line_no"] for _, c in selected_cases}) == 3, "Selected cases must be distinct"

    # Spot checks execution
    for cat_name, case in selected_cases:
        cmd = [sys.executable, "-B", "-m", "pyfiglet"] + case["args"]
        expected_bytes = case["out"].encode("utf-8")

        try:
            proc = subprocess.run(
                cmd,
                cwd=str(clone_root),
                env=child_env,
                capture_output=True,
                shell=False,
                timeout=5,
            )
        except subprocess.TimeoutExpired:
            print(
                f"Line: {case['line_no']}, Category: {cat_name}, Args: {json.dumps(case['args'])}, "
                f"Expected: {len(expected_bytes)} bytes, Actual: 0 bytes, Status: FAIL",
                file=sys.stderr,
            )
            sys.exit(1)
        except Exception as e:
            print(
                f"Line: {case['line_no']}, Category: {cat_name}, Args: {json.dumps(case['args'])}, "
                f"Expected: {len(expected_bytes)} bytes, Actual: 0 bytes, Status: FAIL",
                file=sys.stderr,
            )
            sys.exit(1)

        actual_bytes = proc.stdout
        expected_len = len(expected_bytes)
        actual_len = len(actual_bytes)

        if proc.returncode != 0:
            print(
                f"Line: {case['line_no']}, Category: {cat_name}, Args: {json.dumps(case['args'])}, "
                f"Expected: {expected_len} bytes, Actual: {actual_len} bytes, Status: FAIL",
                file=sys.stderr,
            )
            if proc.stderr:
                print(
                    f"Stderr: {proc.stderr.decode('utf-8', errors='replace').strip()}",
                    file=sys.stderr,
                )
            sys.exit(1)

        if len(proc.stderr) > 0:
            print(
                f"Line: {case['line_no']}, Category: {cat_name}, Args: {json.dumps(case['args'])}, "
                f"Expected: {expected_len} bytes, Actual: {actual_len} bytes, Status: FAIL",
                file=sys.stderr,
            )
            print(
                f"Stderr: {proc.stderr.decode('utf-8', errors='replace').strip()}",
                file=sys.stderr,
            )
            sys.exit(1)

        if actual_bytes != expected_bytes:
            print(
                f"Line: {case['line_no']}, Category: {cat_name}, Args: {json.dumps(case['args'])}, "
                f"Expected: {expected_len} bytes, Actual: {actual_len} bytes, Status: FAIL",
                file=sys.stderr,
            )
            sys.exit(1)

        print(
            f"Line: {case['line_no']}, Category: {cat_name}, Args: {json.dumps(case['args'])}, "
            f"Expected: {expected_len} bytes, Actual: {actual_len} bytes, Status: PASS"
        )

    # Print success banner
    print("PASS: dataset validation and 3/3 spot checks")


if __name__ == "__main__":
    main()

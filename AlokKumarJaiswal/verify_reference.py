#!/usr/bin/env python3
"""
Verify exact subprocess capture of the pinned original pyfiglet CLI.
"""

import os
import pathlib
import subprocess
import sys

EXPECTED_COMMIT = "255827efbc09d662875dad50fa0edc1e1263bf45"

FONTS = [
    "standard",
    "slant",
    "small",
    "big",
    "mini",
    "banner",
    "shadow",
    "script",
]

FLAG_CASES = [
    ("none", []),
    ("-s", ["-s"]),
    ("-n", ["-n"]),
    ("both -s -n", ["-s", "-n"]),
]


def get_git_head(repo_dir: pathlib.Path) -> str:
    try:
        proc = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=str(repo_dir),
            capture_output=True,
            timeout=5,
        )
        if proc.returncode == 0 and proc.stdout:
            commit = proc.stdout.decode("utf-8").strip()
            if commit:
                return commit
    except Exception:
        pass

    head_file = repo_dir / ".git" / "HEAD"
    if head_file.is_file():
        content = head_file.read_text(encoding="utf-8").strip()
        if not content.startswith("ref:"):
            return content
        ref_rel = content[4:].strip()
        ref_file = repo_dir / ".git" / ref_rel
        if ref_file.is_file():
            return ref_file.read_text(encoding="utf-8").strip()
        packed_refs = repo_dir / ".git" / "packed-refs"
        if packed_refs.is_file():
            for line in packed_refs.read_text(encoding="utf-8").splitlines():
                line = line.strip()
                if not line or line.startswith("#") or line.startswith("^"):
                    continue
                parts = line.split()
                if len(parts) == 2 and parts[1] == ref_rel:
                    return parts[0]
    raise RuntimeError(f"Could not determine git HEAD for {repo_dir}")


def main() -> None:
    # Locate sibling pyfiglet clone relative to __file__
    script_dir = pathlib.Path(__file__).resolve().parent
    clone_root = (script_dir / "pyfiglet").resolve()
    if not clone_root.is_dir() and (script_dir.parent / "pyfiglet").is_dir():
        clone_root = (script_dir.parent / "pyfiglet").resolve()

    if not clone_root.is_dir():
        print(f"ERROR: pyfiglet clone not found at {clone_root}", file=sys.stderr)
        sys.exit(1)

    # Verify git HEAD equals expected commit
    try:
        actual_head = get_git_head(clone_root)
        assert (
            actual_head == EXPECTED_COMMIT
        ), f"git HEAD mismatch: expected {EXPECTED_COMMIT}, got {actual_head}"
    except AssertionError as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"ERROR: Failed checking git HEAD: {e}", file=sys.stderr)
        sys.exit(1)

    # Set child process environment
    child_env = os.environ.copy()
    child_env["PYTHONDONTWRITEBYTECODE"] = "1"
    child_env["PYTHONUTF8"] = "1"

    # First launch a child process from the clone root that imports pyfiglet
    # and prints its resolved __file__. Assert it is the expected local
    # pyfiglet/__init__.py, then print the verified path.
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
            timeout=5,
        )
    except subprocess.TimeoutExpired as e:
        stderr_str = e.stderr.decode("utf-8", errors="replace") if e.stderr else "(none)"
        print(
            f"ERROR: Initial import check timed out after 5 seconds.\nStderr: {stderr_str}",
            file=sys.stderr,
        )
        sys.exit(1)

    if proc.returncode != 0:
        stderr_str = proc.stderr.decode("utf-8", errors="replace") if proc.stderr else "(none)"
        print(
            f"ERROR: Initial import check exited with code {proc.returncode}.\nStderr: {stderr_str}",
            file=sys.stderr,
        )
        sys.exit(1)

    if proc.stderr:
        stderr_str = proc.stderr.decode("utf-8", errors="replace")
        print(
            f"ERROR: Initial import check had non-empty stderr.\nStderr: {stderr_str}",
            file=sys.stderr,
        )
        sys.exit(1)

    try:
        raw_path = proc.stdout.decode("utf-8", errors="strict").strip()
        resolved_init = pathlib.Path(raw_path).resolve()
        assert (
            resolved_init == expected_init
        ), f"Imported pyfiglet path {resolved_init} does not match expected {expected_init}"
    except AssertionError as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"ERROR: Could not verify pyfiglet import path: {e}", file=sys.stderr)
        sys.exit(1)

    print(f"Verified pyfiglet path: {resolved_init}")

    # Test eight fonts with four invocations each
    total_passed = 0

    for font in FONTS:
        outputs = {}
        for flag_label, flags in FLAG_CASES:
            cmd = [
                sys.executable,
                "-B",
                "-m",
                "pyfiglet",
                "-f",
                font,
                "-w",
                "80",
            ] + flags + ["hello"]

            flag_desc = " ".join(flags) if flags else "none"

            try:
                proc = subprocess.run(
                    cmd,
                    cwd=str(clone_root),
                    env=child_env,
                    capture_output=True,
                    timeout=5,
                )
            except subprocess.TimeoutExpired as e:
                stderr_str = e.stderr.decode("utf-8", errors="replace") if e.stderr else "(none)"
                print(
                    f"ERROR: Timeout after 5s for font={font!r}, flags={flag_desc!r}.\nStderr: {stderr_str}",
                    file=sys.stderr,
                )
                sys.exit(1)

            stderr_str = proc.stderr.decode("utf-8", errors="replace") if proc.stderr else "(empty)"

            if proc.returncode != 0:
                print(
                    f"ERROR: Non-zero exit code {proc.returncode} for font={font!r}, flags={flag_desc!r}.\nStderr: {stderr_str}",
                    file=sys.stderr,
                )
                sys.exit(1)

            if proc.stderr != b"":
                print(
                    f"ERROR: Non-empty stderr for font={font!r}, flags={flag_desc!r}.\nStderr: {stderr_str}",
                    file=sys.stderr,
                )
                sys.exit(1)

            # Strictly decode stdout as UTF-8 and assert re-encoding reproduces
            # captured bytes exactly without stripping or normalizing.
            try:
                decoded_stdout = proc.stdout.decode("utf-8", errors="strict")
                re_encoded = decoded_stdout.encode("utf-8")
                assert (
                    re_encoded == proc.stdout
                ), f"Re-encoded UTF-8 does not match captured raw bytes for font={font!r}, flags={flag_desc!r}"
            except AssertionError as e:
                print(
                    f"ERROR: {e}\nFont: {font}\nFlags: {flag_desc}\nStderr: {stderr_str}",
                    file=sys.stderr,
                )
                sys.exit(1)
            except UnicodeDecodeError as e:
                print(
                    f"ERROR: UTF-8 strict decoding error for font={font!r}, flags={flag_desc!r}: {e}\nStderr: {stderr_str}",
                    file=sys.stderr,
                )
                sys.exit(1)

            outputs[flag_label] = proc.stdout
            total_passed += 1

        # Assert output with both -s and -n equals output with -s alone.
        try:
            assert (
                outputs["both -s -n"] == outputs["-s"]
            ), f"Output with 'both -s -n' does not match output with '-s' alone for font={font!r}"
        except AssertionError as e:
            print(
                f"ERROR: {e}\nFont: {font}\nFlags: both -s -n vs -s\nStderr: (none)",
                file=sys.stderr,
            )
            sys.exit(1)

        # Print one compact summary per font: PASS, and the byte lengths for the four outputs.
        print(
            f"{font}: PASS (byte lengths: none={len(outputs['none'])}, "
            f"-s={len(outputs['-s'])}, -n={len(outputs['-n'])}, both={len(outputs['both -s -n'])})"
        )

    # Finally print PASS: 32/32 CLI captures only if all checks passed.
    try:
        assert total_passed == 32, f"Expected 32 passed captures, got {total_passed}"
    except AssertionError as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)

    print("PASS: 32/32 CLI captures")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""
mutation_check.py

Single mutation experiment checking universal smushing logic in oracle_dev
against the visible test dataset using the figlet-room Docker container.
"""

import hashlib
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

WORKSPACE_ROOT = Path(__file__).resolve().parent

SUMMARY_PATTERN = re.compile(
    r"summary:\s*matches=(\d+)\s+differences=(\d+)\s+timeouts=(\d+)\s+nonzero_exits=(\d+)"
)


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest()


def record_oracle_dev(oracle_dev_dir: Path) -> dict[str, str]:
    if not oracle_dev_dir.is_dir():
        raise FileNotFoundError(f"oracle_dev directory not found at {oracle_dev_dir}")

    regular_files: dict[str, str] = {}
    for p in sorted(oracle_dev_dir.rglob("*")):
        if p.is_symlink():
            raise RuntimeError(f"Safety violation: symlink rejected in oracle_dev: {p}")
        if p.is_file():
            rel_name = p.relative_to(oracle_dev_dir).as_posix()
            regular_files[rel_name] = sha256_file(p)
    return regular_files


def find_visible_dataset(workspace_root: Path) -> Path:
    candidates = [
        workspace_root / "tests" / "data" / "visible.jsonl",
        workspace_root / "figlet-go" / "tests" / "data" / "visible.jsonl",
    ]
    for c in candidates:
        if c.is_file():
            return c
    raise FileNotFoundError(
        "Could not find visible.jsonl in tests/data/ or figlet-go/tests/data/"
    )


def docker_path(p: Path) -> str:
    # Docker on Windows expects clean path separators for volume mounts
    return str(p.resolve()).replace("\\", "/")


def run_docker_comparator(
    module_dir: Path,
    dev_compare_path: Path,
    visible_dataset_path: Path,
    timeout_sec: int = 180,
) -> tuple[int, str, str]:
    host_mod = docker_path(module_dir)
    host_cmp = docker_path(dev_compare_path)
    host_vis = docker_path(visible_dataset_path)

    inner_cmd = (
        "mkdir -p /tmp/gocache /tmp/gotmp && "
        "cd /src/oracle_dev && "
        "go build -o /tmp/figlet . && "
        "go build -o /tmp/dev_compare /tools/dev_compare.go && "
        "/tmp/dev_compare"
    )

    docker_args = [
        "docker",
        "run",
        "--rm",
        "--network", "none",
        "-e", "GOCACHE=/tmp/gocache",
        "-e", "GOTMPDIR=/tmp/gotmp",
        "-e", "GOPROXY=off",
        "-e", "GONOSUMDB=*",
        "-e", "CGO_ENABLED=0",
        "-v", f"{host_mod}:/src/oracle_dev:ro",
        "-v", f"{host_cmp}:/tools/dev_compare.go:ro",
        "-v", f"{host_vis}:/cases/visible.jsonl:ro",
        "figlet-room",
        "sh", "-c", inner_cmd,
    ]

    proc = subprocess.run(
        docker_args,
        capture_output=True,
        text=True,
        timeout=timeout_sec,
        shell=False,
    )
    return proc.returncode, proc.stdout, proc.stderr


def parse_comparator_summary(stdout: str) -> dict[str, int]:
    m = SUMMARY_PATTERN.search(stdout)
    if not m:
        raise ValueError("Failed to locate 'summary:' line in comparator output")
    return {
        "matches": int(m.group(1)),
        "differences": int(m.group(2)),
        "timeouts": int(m.group(3)),
        "nonzero_exits": int(m.group(4)),
    }


def verify_safety_state(
    oracle_dev_dir: Path,
    baseline_oracle_files: dict[str, str],
    protected_files: dict[Path, str],
) -> None:
    errors: list[str] = []

    # Check oracle_dev files
    current_oracle_files: dict[str, str] = {}
    for p in sorted(oracle_dev_dir.rglob("*")):
        if p.is_symlink():
            errors.append(f"Safety violation: symlink detected in oracle_dev: {p}")
        if p.is_file():
            rel_name = p.relative_to(oracle_dev_dir).as_posix()
            current_oracle_files[rel_name] = sha256_file(p)

    added = set(current_oracle_files.keys()) - set(baseline_oracle_files.keys())
    removed = set(baseline_oracle_files.keys()) - set(current_oracle_files.keys())

    if added:
        errors.append(f"Safety violation: new file(s) created in oracle_dev: {sorted(added)}")
    if removed:
        errors.append(f"Safety violation: file(s) deleted from oracle_dev: {sorted(removed)}")

    for rel_name, orig_hash in baseline_oracle_files.items():
        if rel_name in current_oracle_files:
            curr_hash = current_oracle_files[rel_name]
            if curr_hash != orig_hash:
                errors.append(
                    f"Safety violation: {rel_name} was mutated (hash changed from {orig_hash} to {curr_hash})"
                )

    # Check external protected files
    for path, orig_hash in protected_files.items():
        if not path.is_file():
            errors.append(f"Safety violation: protected file disappeared: {path}")
        else:
            curr_hash = sha256_file(path)
            if curr_hash != orig_hash:
                errors.append(
                    f"Safety violation: protected file {path} was modified (hash mismatch)"
                )

    if errors:
        for err in errors:
            print(f"ERROR: {err}", file=sys.stderr)
        raise RuntimeError(f"Safety audit failed with {len(errors)} error(s)")


def main() -> int:
    oracle_dev_dir = WORKSPACE_ROOT / "oracle_dev"
    dev_compare_path = WORKSPACE_ROOT / "dev_compare.go"

    if not dev_compare_path.is_file():
        print(f"Error: dev_compare.go not found at {dev_compare_path}", file=sys.stderr)
        return 1

    visible_dataset_path = find_visible_dataset(WORKSPACE_ROOT)

    # Record SHA-256 hashes of protected files
    baseline_oracle_files = record_oracle_dev(oracle_dev_dir)

    protected_files: dict[Path, str] = {}
    solve_sh_path = WORKSPACE_ROOT / "figlet-go" / "solution" / "solve.sh"
    if solve_sh_path.is_file():
        protected_files[solve_sh_path] = sha256_file(solve_sh_path)

    # Protect visible.jsonl
    protected_files[visible_dataset_path] = sha256_file(visible_dataset_path)

    temp_dir_obj = tempfile.TemporaryDirectory(dir=WORKSPACE_ROOT)
    try:
        temp_dir = Path(temp_dir_obj.name)
        temp_oracle_dev = temp_dir / "oracle_dev"
        shutil.copytree(oracle_dev_dir, temp_oracle_dev, symlinks=False)

        # Baseline execution
        base_code, base_stdout, base_stderr = run_docker_comparator(
            temp_oracle_dev, dev_compare_path, visible_dataset_path
        )

        if base_code != 0:
            print(
                f"Baseline error: comparator exited with code {base_code}.\n"
                f"Stderr: {base_stderr.strip()}\nStdout: {base_stdout.strip()}",
                file=sys.stderr,
            )
            print("Baseline failed; aborting experiment.", file=sys.stderr)
            return 1

        try:
            base_summary = parse_comparator_summary(base_stdout)
        except Exception as e:
            print(f"Baseline parsing error: {e}", file=sys.stderr)
            return 1

        if (
            base_summary["matches"] != 2000
            or base_summary["differences"] != 0
            or base_summary["timeouts"] != 0
            or base_summary["nonzero_exits"] != 0
        ):
            print(
                f"Baseline requirement failure: expected matches=2000 differences=0 timeouts=0 nonzero_exits=0, "
                f"got {base_summary}",
                file=sys.stderr,
            )
            print("Baseline failed; aborting experiment.", file=sys.stderr)
            return 1

        # Mutation: mutate copied builder.go
        copied_builder = temp_oracle_dev / "builder.go"
        if not copied_builder.is_file():
            print(f"Error: copied builder.go not found at {copied_builder}", file=sys.stderr)
            return 1

        content = copied_builder.read_text(encoding="utf-8")
        target_needle = "if (b.font.SmushMode & 63) == 0 {"
        replacement_code = (
            "if (b.font.SmushMode & 63) == 0 { return nil }; if (b.font.SmushMode & 63) == 0 {"
        )

        occurrences = content.count(target_needle)
        if occurrences != 1:
            print(
                f"Mutation setup error: expected exactly 1 occurrence of '{target_needle}', found {occurrences}",
                file=sys.stderr,
            )
            return 1

        mutated_content = content.replace(target_needle, replacement_code, 1)
        copied_builder.write_text(mutated_content, encoding="utf-8")

        # Mutated execution in disposable container
        mut_code, mut_stdout, mut_stderr = run_docker_comparator(
            temp_oracle_dev, dev_compare_path, visible_dataset_path
        )

        try:
            mut_summary = parse_comparator_summary(mut_stdout)
        except Exception as e:
            print(
                f"Experiment Failure: Could not parse comparator summary for mutant: {e}\n"
                f"Return code: {mut_code}\nStderr: {mut_stderr.strip()}",
                file=sys.stderr,
            )
            return 1

        total_cases = mut_summary["matches"] + mut_summary["differences"]
        if total_cases != 2000:
            print(
                f"Experiment Failure: matches + differences = {total_cases} (expected 2000)",
                file=sys.stderr,
            )
            return 1

        if mut_summary["timeouts"] > 0 or mut_summary["nonzero_exits"] > 0:
            print(
                f"Experiment Failure: Mutant run experienced setup/runtime errors: "
                f"timeouts={mut_summary['timeouts']}, nonzero_exits={mut_summary['nonzero_exits']}",
                file=sys.stderr,
            )
            return 1

        # A valid mutated run should exit 1 if differences exist, or 0 if none
        if mut_summary["differences"] > 0 and mut_code != 1:
            print(
                f"Experiment Failure: Expected comparator exit code 1 when differences > 0, got {mut_code}",
                file=sys.stderr,
            )
            return 1
        elif mut_summary["differences"] == 0 and mut_code != 0:
            print(
                f"Experiment Failure: Expected comparator exit code 0 when differences == 0, got {mut_code}",
                file=sys.stderr,
            )
            return 1

        diff_percent = (mut_summary["differences"] / 2000.0) * 100.0

        # Print aggregate results only (no full rendered outputs)
        print("=== Mutation Experiment Aggregate Results ===")
        print(
            f"Baseline: matches={base_summary['matches']} "
            f"differences={base_summary['differences']} "
            f"timeouts={base_summary['timeouts']} "
            f"nonzero_exits={base_summary['nonzero_exits']}"
        )
        print(
            f"Mutant:   matches={mut_summary['matches']} "
            f"differences={mut_summary['differences']} "
            f"timeouts={mut_summary['timeouts']} "
            f"nonzero_exits={mut_summary['nonzero_exits']}"
        )
        print(f"Differing Cases: {mut_summary['differences']}/2000 ({diff_percent:.2f}%)")

        if mut_summary["differences"] > 0:
            print("Outcome: UNIVERSAL_MUTATION_DETECTED")
        else:
            print("Outcome: UNIVERSAL_MUTATION_SURVIVED")
            print(
                "Note: The universal smushing mutation survived across the 2,000 visible cases. "
                "This does not imply that universal smushing is universally untested or unused in FIGlet."
            )

        print("\n=== Scope and Runtime-Unverified Notice ===")
        print("- Hidden dataset (hidden.jsonl, secrets, dataset backups) was not mounted, inspected, or tested.")
        print("- Controlled smush modes (equal, lowline, hierarchy, pair, bigx, hardblank) and flags were not mutated in this run and remain unverified here.")
        print("- Font behaviors and edge cases outside the 2,000 visible dataset cases remain runtime-unverified.")

        return 0

    finally:
        try:
            temp_dir_obj.cleanup()
        except Exception:
            pass
        verify_safety_state(oracle_dev_dir, baseline_oracle_files, protected_files)


if __name__ == "__main__":
    sys.exit(main())

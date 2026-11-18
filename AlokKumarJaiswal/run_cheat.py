#!/usr/bin/env python3
"""
run_cheat.py

Standard-library Python helper anchored to __file__ to run a cheat evaluation
with Harbor, securely restoring the real solution afterwards.

Safety Notice:
While this script uses exclusive backup creation, atomic replacement via os.replace,
and try/finally blocks to guarantee restoration during standard execution, errors,
or KeyboardInterrupt, recovery cannot be guaranteed in the event of abrupt process
termination (e.g. SIGKILL / Windows TerminateProcess) or sudden hardware/power loss
while the file is swapped.
"""

import hashlib
import json
import math
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile

EXPECTED_REAL_SHA256 = (
    "63fb1050569e8b33c3f08a87b844848a1791e674da22bd97269d706fdd429cec"
)
EXPECTED_CHEAT_SHA256 = (
    "c9fc5e62259913708a7b750da2724d971d22b0dc4baa53f5000a5be7b6c4ff98"
)


def validate_regular_non_symlink(path: pathlib.Path, description: str) -> None:
    """Validate that path exists, is a regular file, and is not a symlink."""
    if not path.exists():
        sys.exit(f"ERROR: {description} does not exist: {path}")
    if path.is_symlink():
        sys.exit(f"ERROR: {description} ({path}) must not be a symlink.")
    if not path.is_file():
        sys.exit(f"ERROR: {description} ({path}) must be a regular file.")


def compute_sha256(path: pathlib.Path) -> str:
    """Compute hex-encoded SHA-256 of file contents."""
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main() -> None:
    # Anchor all paths to __file__
    base_dir = pathlib.Path(__file__).resolve().parent
    task_dir = base_dir / "figlet-go"
    real_solution_path = task_dir / "solution" / "solve.sh"
    cheat_solution_path = task_dir / "cheat" / "solve.sh"
    job_dir = task_dir / "jobs" / "figlet-equivalence-cheat-01"
    backup_path = base_dir / "solution_backup" / "figlet-oracle-before-equivalence-cheat.sh"

    # --- Pre-run verifications before changing anything ---

    # 1. Verify both files are regular, non-symlink files with exact hashes
    validate_regular_non_symlink(real_solution_path, "Real solution file")
    real_bytes = real_solution_path.read_bytes()
    actual_real_hash = hashlib.sha256(real_bytes).hexdigest()
    if actual_real_hash != EXPECTED_REAL_SHA256:
        sys.exit(
            f"ERROR: Real solution SHA-256 mismatch:\n"
            f"  expected: {EXPECTED_REAL_SHA256}\n"
            f"  actual:   {actual_real_hash}"
        )

    validate_regular_non_symlink(cheat_solution_path, "Cheat solution file")
    cheat_bytes = cheat_solution_path.read_bytes()
    actual_cheat_hash = hashlib.sha256(cheat_bytes).hexdigest()
    if actual_cheat_hash != EXPECTED_CHEAT_SHA256:
        sys.exit(
            f"ERROR: Cheat solution SHA-256 mismatch:\n"
            f"  expected: {EXPECTED_CHEAT_SHA256}\n"
            f"  actual:   {actual_cheat_hash}"
        )

    # 2. Refuse if the job directory or backup file already exists
    if job_dir.exists():
        sys.exit(f"ERROR: Job directory already exists: {job_dir}")
    if backup_path.exists():
        sys.exit(f"ERROR: Backup file already exists: {backup_path}")

    # 3. Ensure harbor is available
    harbor_bin = shutil.which("harbor")
    if not harbor_bin:
        sys.exit("ERROR: 'harbor' executable not found in PATH.")

    # --- Create backup with exclusive creation and verify ---
    backup_path.parent.mkdir(parents=True, exist_ok=True)
    try:
        with open(backup_path, "xb") as bf:
            bf.write(real_bytes)
            bf.flush()
            os.fsync(bf.fileno())
    except FileExistsError:
        sys.exit(f"ERROR: Backup file already exists: {backup_path}")

    backup_bytes = backup_path.read_bytes()
    backup_hash = hashlib.sha256(backup_bytes).hexdigest()
    if backup_hash != EXPECTED_REAL_SHA256:
        sys.exit(
            f"ERROR: Backup verification failed (hash mismatch):\n"
            f"  expected: {EXPECTED_REAL_SHA256}\n"
            f"  actual:   {backup_hash}"
        )
    if backup_bytes != real_bytes:
        sys.exit("ERROR: Backup verification failed (byte mismatch with real solution).")

    # --- Swap solution and run Harbor inside try/finally ---
    harbor_rc = None
    try:
        # Replace solution/solve.sh with cheat bytes using temp file + os.replace
        tmp_fd, tmp_swap_path = tempfile.mkstemp(
            prefix="solve_swap_", dir=real_solution_path.parent
        )
        try:
            with os.fdopen(tmp_fd, "wb") as f:
                f.write(cheat_bytes)
                f.flush()
                os.fsync(f.fileno())
            try:
                os.chmod(tmp_swap_path, 0o755)
            except Exception:
                pass
            os.replace(tmp_swap_path, real_solution_path)
        except BaseException:
            if os.path.exists(tmp_swap_path):
                try:
                    os.remove(tmp_swap_path)
                except OSError:
                    pass
            raise

        # Verify swapped solution hash
        swapped_hash = compute_sha256(real_solution_path)
        if swapped_hash != EXPECTED_CHEAT_SHA256:
            raise RuntimeError(
                f"Swapped solution SHA-256 mismatch:\n"
                f"  expected: {EXPECTED_CHEAT_SHA256}\n"
                f"  actual:   {swapped_hash}"
            )

        # Run Harbor command with inherited terminal stdout/stderr
        # harbor run -p . -a oracle --job-name figlet-equivalence-cheat-01
        cmd = [
            harbor_bin,
            "run",
            "-p",
            ".",
            "-a",
            "oracle",
            "--job-name",
            "figlet-equivalence-cheat-01",
        ]
        proc = subprocess.run(cmd, cwd=task_dir)
        harbor_rc = proc.returncode

    finally:
        # In finally, restore original bytes from verified backup using temp file and os.replace
        tmp_fd, tmp_restore_path = tempfile.mkstemp(
            prefix="solve_restore_", dir=real_solution_path.parent
        )
        try:
            b_bytes = backup_path.read_bytes()
            b_hash = hashlib.sha256(b_bytes).hexdigest()
            if b_hash != EXPECTED_REAL_SHA256:
                raise RuntimeError(
                    f"CRITICAL: Backup file hash mismatch during restoration:\n"
                    f"  expected: {EXPECTED_REAL_SHA256}\n"
                    f"  actual:   {b_hash}"
                )
            with os.fdopen(tmp_fd, "wb") as f:
                f.write(b_bytes)
                f.flush()
                os.fsync(f.fileno())
            try:
                os.chmod(tmp_restore_path, 0o755)
            except Exception:
                pass
            os.replace(tmp_restore_path, real_solution_path)
        except BaseException:
            if os.path.exists(tmp_restore_path):
                try:
                    os.remove(tmp_restore_path)
                except OSError:
                    pass
            raise

        restored_hash = compute_sha256(real_solution_path)
        if restored_hash != EXPECTED_REAL_SHA256:
            raise RuntimeError(
                f"CRITICAL: Restored solution hash mismatch:\n"
                f"  expected: {EXPECTED_REAL_SHA256}\n"
                f"  actual:   {restored_hash}"
            )
        print(f"ORACLE_RESTORED {restored_hash}")

    # --- Only after successful restoration and Harbor exit 0 ---
    if harbor_rc is None or harbor_rc != 0:
        sys.exit(f"ERROR: Harbor failed with exit code: {harbor_rc}")

    if not job_dir.exists():
        sys.exit(f"ERROR: Expected fresh job directory does not exist: {job_dir}")

    # Require exactly one trial verifier/reward.json under this fresh job
    reward_candidates = [
        p for p in job_dir.rglob("reward.json") if p.parent.name == "verifier"
    ]
    if len(reward_candidates) != 1:
        sys.exit(
            f"ERROR: Expected exactly one trial verifier/reward.json under {job_dir}, "
            f"found {len(reward_candidates)}: {reward_candidates}"
        )

    reward_path = reward_candidates[0]
    verifier_dir = reward_path.parent
    trial_dir = verifier_dir.parent

    # Validate numeric reward/build/no_python/behaviour fields
    try:
        reward_obj = json.loads(reward_path.read_text(encoding="utf-8"))
    except Exception as exc:
        sys.exit(f"ERROR: Could not parse {reward_path} as JSON: {exc}")

    if not isinstance(reward_obj, dict):
        sys.exit(
            f"ERROR: Expected JSON object in {reward_path}, got {type(reward_obj).__name__}"
        )

    for field in ("reward", "build", "no_python", "behaviour"):
        if field not in reward_obj:
            sys.exit(f"ERROR: Missing field '{field}' in {reward_path}")
        val = reward_obj[field]
        if isinstance(val, bool) or not isinstance(val, (int, float)):
            sys.exit(
                f"ERROR: Field '{field}' in {reward_path} must be numeric, got {type(val).__name__} ({val!r})"
            )
        if not math.isfinite(val):
            sys.exit(f"ERROR: Field '{field}' in {reward_path} is not finite: {val}")

    build_val = reward_obj["build"]
    no_python_val = reward_obj["no_python"]
    behaviour_val = reward_obj["behaviour"]
    reward_val = reward_obj["reward"]

    # Print trial path and component scores
    print(f"Trial path: {trial_dir}")
    print(
        f"Component scores: build={build_val}, no_python={no_python_val}, "
        f"behaviour={behaviour_val}, reward={reward_val}"
    )

    # Assert build=1, no_python=0, behaviour=0 and reward equals 1/6
    # within a small floating-point tolerance; also assert reward < 0.2.
    assert build_val == 1, f"Expected build=1, got {build_val}"
    assert no_python_val == 0, f"Expected no_python=0, got {no_python_val}"
    assert behaviour_val == 0, f"Expected behaviour=0, got {behaviour_val}"
    assert math.isclose(reward_val, 1.0 / 6.0, rel_tol=1e-5, abs_tol=1e-5), (
        f"Expected reward == 1/6 (~0.166667), got {reward_val}"
    )
    assert reward_val < 0.2, f"Expected reward < 0.2, got {reward_val}"

    # Check verifier/test-stdout.txt contains "No secret for you. Yet."
    # and does not contain "*** SECRET UNLOCKED ***".
    test_stdout_path = verifier_dir / "test-stdout.txt"
    if not test_stdout_path.is_file():
        sys.exit(f"ERROR: Verifier stdout log missing: {test_stdout_path}")

    test_stdout_content = test_stdout_path.read_text(encoding="utf-8", errors="replace")
    has_no_secret = "No secret for you. Yet." in test_stdout_content
    has_secret_unlocked = "*** SECRET UNLOCKED ***" in test_stdout_content

    # Print only those log checks, not the full verifier log
    print(f"Log check ['No secret for you. Yet.' present]: {has_no_secret}")
    print(f"Log check ['*** SECRET UNLOCKED ***' present]: {has_secret_unlocked}")

    assert has_no_secret, "Verifier log does not contain 'No secret for you. Yet.'"
    assert (
        not has_secret_unlocked
    ), "Verifier log unexpectedly contains '*** SECRET UNLOCKED ***'"

    # Print CHEAT_CHECK_PASSED only after all assertions pass
    print("CHEAT_CHECK_PASSED")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""
build_solve.py

Standard-library Python helper to generate figlet-go/solution/solve.sh
from the existing oracle_dev implementation.
"""

import base64
import gzip
import hashlib
import io
import os
import pathlib
import re
import stat
import sys
import tarfile
import tempfile


def validate_not_symlink_and_regular(path: pathlib.Path, description: str) -> None:
    if path.is_symlink():
        sys.exit(f"ERROR: {description} ({path}) must not be a symlink.")
    if not path.is_file():
        sys.exit(f"ERROR: {description} ({path}) must be a regular file.")


def main() -> None:
    # 1. Anchor all paths to __file__
    base_dir = pathlib.Path(__file__).resolve().parent
    oracle_dir = base_dir / "oracle_dev"
    pyfiglet_dir = base_dir / "pyfiglet"
    solution_dir = base_dir / "figlet-go" / "solution"
    target_solve_sh = solution_dir / "solve.sh"

    # 2. Check oracle_dev/go.mod exists and is a regular file
    go_mod_path = oracle_dir / "go.mod"
    if not go_mod_path.exists():
        sys.exit(f"ERROR: Missing required file: {go_mod_path}")
    validate_not_symlink_and_regular(go_mod_path, "oracle_dev/go.mod")

    # Read go.mod and check for third-party require/replace directives
    go_mod_bytes = go_mod_path.read_bytes()
    try:
        go_mod_text = go_mod_bytes.decode("utf-8")
    except UnicodeDecodeError as exc:
        sys.exit(f"ERROR: oracle_dev/go.mod is not valid UTF-8: {exc}")

    for line_no, raw_line in enumerate(go_mod_text.splitlines(), start=1):
        clean_line = raw_line.split("//")[0].strip()
        if not clean_line:
            continue
        if re.match(r"^(require|replace)\b", clean_line):
            sys.exit(
                f"ERROR: Forbidden directive found in go.mod at line {line_no}: {raw_line}"
            )

    # 3. Require existing Go source files in oracle_dev (excluding *_test.go)
    if not oracle_dir.exists() or not oracle_dir.is_dir() or oracle_dir.is_symlink():
        sys.exit(f"ERROR: {oracle_dir} must exist as a directory and not a symlink.")

    go_files = []
    for item in oracle_dir.iterdir():
        if item.name.endswith(".go") and not item.name.endswith("_test.go"):
            validate_not_symlink_and_regular(item, f"Go source file {item.name}")
            go_files.append(item)

    if not go_files:
        sys.exit("ERROR: No Go source files (*.go excluding *_test.go) found in oracle_dev.")
    go_files.sort(key=lambda p: p.name)

    # 4. Verify oracle_dev/LICENSE byte-for-byte against pyfiglet/LICENSE
    oracle_lic = oracle_dir / "LICENSE"
    pyfiglet_lic = pyfiglet_dir / "LICENSE"

    if not oracle_lic.exists():
        sys.exit(f"ERROR: Missing license file: {oracle_lic}")
    validate_not_symlink_and_regular(oracle_lic, "oracle_dev/LICENSE")

    if not pyfiglet_lic.exists():
        sys.exit(f"ERROR: Missing reference license file: {pyfiglet_lic}")
    validate_not_symlink_and_regular(pyfiglet_lic, "pyfiglet/LICENSE")

    if oracle_lic.read_bytes() != pyfiglet_lic.read_bytes():
        sys.exit("ERROR: oracle_dev/LICENSE does not match pyfiglet/LICENSE byte-for-byte.")

    # 5. Verify font sets and byte equality
    oracle_fonts_dir = oracle_dir / "fonts"
    pyfiglet_fonts_dir = pyfiglet_dir / "pyfiglet" / "fonts-standard"

    if (
        not oracle_fonts_dir.exists()
        or not oracle_fonts_dir.is_dir()
        or oracle_fonts_dir.is_symlink()
    ):
        sys.exit(f"ERROR: {oracle_fonts_dir} must exist as a directory and not a symlink.")

    if (
        not pyfiglet_fonts_dir.exists()
        or not pyfiglet_fonts_dir.is_dir()
        or pyfiglet_fonts_dir.is_symlink()
    ):
        sys.exit(f"ERROR: {pyfiglet_fonts_dir} must exist as a directory and not a symlink.")

    oracle_fonts = {}
    for item in oracle_fonts_dir.iterdir():
        if item.name.endswith(".flf"):
            validate_not_symlink_and_regular(item, f"oracle font {item.name}")
            oracle_fonts[item.name] = item

    pyfiglet_fonts = {}
    for item in pyfiglet_fonts_dir.iterdir():
        if item.name.endswith(".flf"):
            validate_not_symlink_and_regular(item, f"pyfiglet font {item.name}")
            pyfiglet_fonts[item.name] = item

    if not oracle_fonts:
        sys.exit("ERROR: No .flf font files found in oracle_dev/fonts.")

    oracle_names = set(oracle_fonts.keys())
    pyfiglet_names = set(pyfiglet_fonts.keys())
    if oracle_names != pyfiglet_names:
        extra = sorted(oracle_names - pyfiglet_names)
        missing = sorted(pyfiglet_names - oracle_names)
        sys.exit(
            f"ERROR: Font name set mismatch. Extra in oracle_dev: {extra}, Missing in oracle_dev: {missing}"
        )

    for font_name in sorted(oracle_fonts.keys()):
        if oracle_fonts[font_name].read_bytes() != pyfiglet_fonts[font_name].read_bytes():
            sys.exit(f"ERROR: Font byte mismatch between oracle_dev and pyfiglet for: {font_name}")

    # 6. Assemble package entries (archive-relative paths rooted at oracle_dev)
    # Package only:
    # - oracle_dev/go.mod
    # - oracle_dev/*.go (exclude *_test.go)
    # - oracle_dev/LICENSE
    # - oracle_dev/fonts/*.flf
    package_items = []
    package_items.append(("go.mod", go_mod_path))
    package_items.append(("LICENSE", oracle_lic))
    for gf in go_files:
        package_items.append((gf.name, gf))
    for fn in sorted(oracle_fonts.keys()):
        package_items.append((f"fonts/{fn}", oracle_fonts[fn]))

    # Validate archive-relative paths
    for rel_path, abs_path in package_items:
        if "\\" in rel_path:
            sys.exit(f"ERROR: Archive path must use forward slashes: {rel_path}")
        if rel_path.startswith("/") or os.path.isabs(rel_path):
            sys.exit(f"ERROR: Archive path must not be absolute: {rel_path}")
        parts = rel_path.split("/")
        if any(p in ("..", ".", "") for p in parts):
            sys.exit(f"ERROR: Archive path contains illegal traversal component: {rel_path}")
        validate_not_symlink_and_regular(abs_path, f"Packaged file {rel_path}")

    # Stable entry order (alphabetical by archive-relative path)
    package_items.sort(key=lambda x: x[0])

    # 7. Produce deterministic tar.gz payload:
    # - stable entry order
    # - fixed timestamps (mtime = 0)
    # - fixed ownership (uid = 0, gid = 0, uname = "", gname = "")
    # - regular-file modes (0o644, type = REGTYPE)
    # - fixed gzip timestamp (mtime = 0, filename = "")
    tar_buf = io.BytesIO()
    with tarfile.open(fileobj=tar_buf, mode="w", format=tarfile.GNU_FORMAT) as tar:
        for rel_path, abs_path in package_items:
            file_size = abs_path.stat().st_size
            ti = tarfile.TarInfo(name=rel_path)
            ti.size = file_size
            ti.mtime = 0
            ti.mode = 0o644
            ti.type = tarfile.REGTYPE
            ti.uid = 0
            ti.gid = 0
            ti.uname = ""
            ti.gname = ""
            with open(abs_path, "rb") as f:
                tar.addfile(ti, f)

    uncompressed_tar_bytes = tar_buf.getvalue()

    gz_buf = io.BytesIO()
    with gzip.GzipFile(filename="", mode="wb", fileobj=gz_buf, mtime=0) as gz:
        gz.write(uncompressed_tar_bytes)

    tar_gz_bytes = gz_buf.getvalue()
    b64_payload = base64.b64encode(tar_gz_bytes).decode("ascii")

    # Format base64 payload into standard 76-character lines
    b64_lines = [b64_payload[i : i + 76] for i in range(0, len(b64_payload), 76)]
    b64_payload_formatted = "\n".join(b64_lines)

    # 8. Construct complete solve.sh content
    solve_sh_lines = [
        "#!/bin/bash",
        "set -euo pipefail",
        "",
        "cd /workspace/repo",
        "",
        "base64 -d << 'EOF' | tar -xzf -",
        b64_payload_formatted,
        "EOF",
        "",
        "find /workspace/repo -type f -iname '*.py' -delete",
        "",
        "remaining_py=$(find /workspace/repo -iname '*.py' | wc -l | tr -d ' ')",
        'if [ "${remaining_py}" -ne 0 ]; then',
        '    echo "ERROR: ${remaining_py} .py file(s) or entry/entries still found under /workspace/repo" >&2',
        "    exit 1",
        "fi",
        "",
        "go build ./...",
        "",
        'echo "FIGlet Go solution deployed and verified successfully."',
    ]
    solve_sh_text = "\n".join(solve_sh_lines) + "\n"
    solve_sh_bytes = solve_sh_text.encode("utf-8")
    script_size = len(solve_sh_bytes)
    script_sha256 = hashlib.sha256(solve_sh_bytes).hexdigest()

    # 9. Write via temporary file and os.replace
    solution_dir.mkdir(parents=True, exist_ok=True)
    tmp_fd, tmp_path_str = tempfile.mkstemp(
        dir=str(solution_dir),
        prefix="solve_",
        suffix=".tmp",
    )
    tmp_path = pathlib.Path(tmp_path_str)
    try:
        with os.fdopen(tmp_fd, "wb") as f:
            f.write(solve_sh_bytes)
        try:
            os.chmod(tmp_path_str, 0o755)
        except OSError:
            pass
        os.replace(tmp_path_str, target_solve_sh)
        try:
            os.chmod(target_solve_sh, 0o755)
        except OSError:
            pass
    except Exception:
        if tmp_path.exists():
            tmp_path.unlink(missing_ok=True)
        raise

    # 10. Print required outputs (without base64 payload)
    print(f"Packaged Go source files count: {len(go_files)}")
    print(f"Packaged font files count: {len(oracle_fonts)}")
    print("License verification: SUCCESS (oracle_dev/LICENSE matches pyfiglet/LICENSE byte-for-byte)")
    print(f"Font verification: SUCCESS (all {len(oracle_fonts)} fonts match pyfiglet/pyfiglet/fonts-standard/*.flf byte-for-byte)")
    print(f"Final script size: {script_size} bytes")
    print(f"Final script SHA-256: {script_sha256}")


if __name__ == "__main__":
    main()

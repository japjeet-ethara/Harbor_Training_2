#!/usr/bin/env python3
"""
build_cheat.py

Standard-library Python helper to generate figlet-go/cheat/solve.sh
containing a deterministic cheating Go solution based on visible.jsonl.
"""

import base64
import gzip
import hashlib
import io
import json
import os
import pathlib
import stat
import sys
import tarfile
import tempfile


EXPECTED_VISIBLE_SHA256 = (
    "f38c234706244f4b08e6ea0cde272d0976dd47932130c3c72200f1c0d937790c"
)
EXPECTED_RECORD_COUNT = 2000


def validate_not_symlink_and_regular(path: pathlib.Path, description: str) -> None:
    if path.is_symlink():
        sys.exit(f"ERROR: {description} ({path}) must not be a symlink.")
    if not path.is_file():
        sys.exit(f"ERROR: {description} ({path}) must be a regular file.")


def main() -> None:
    # 1. Anchor all paths to __file__
    base_dir = pathlib.Path(__file__).resolve().parent
    tests_data_dir = base_dir / "figlet-go" / "tests" / "data"
    visible_jsonl_path = tests_data_dir / "visible.jsonl"
    cheat_dir = base_dir / "figlet-go" / "cheat"
    target_solve_sh = cheat_dir / "solve.sh"
    solution_solve_sh = base_dir / "figlet-go" / "solution" / "solve.sh"

    # Enforce safety against modifying solution/solve.sh
    if target_solve_sh.resolve() == solution_solve_sh.resolve():
        sys.exit("ERROR: Target path conflicts with protected solution/solve.sh.")

    # Refuse to overwrite an existing cheat/solve.sh
    if target_solve_sh.exists():
        sys.exit(f"ERROR: Refusing to overwrite existing cheat/solve.sh: {target_solve_sh}")

    # 2. Read only figlet-go/tests/data/visible.jsonl as answer data
    if not visible_jsonl_path.exists():
        sys.exit(f"ERROR: Missing answer data file: {visible_jsonl_path}")
    validate_not_symlink_and_regular(visible_jsonl_path, "visible.jsonl")

    visible_bytes = visible_jsonl_path.read_bytes()

    # 3. Require SHA-256 match
    actual_visible_sha256 = hashlib.sha256(visible_bytes).hexdigest()
    if actual_visible_sha256 != EXPECTED_VISIBLE_SHA256:
        sys.exit(
            f"ERROR: SHA-256 mismatch for visible.jsonl:\n"
            f"  expected: {EXPECTED_VISIBLE_SHA256}\n"
            f"  actual:   {actual_visible_sha256}"
        )

    # 4. Validate 2000 records and unique args
    try:
        visible_text = visible_bytes.decode("utf-8")
    except UnicodeDecodeError as exc:
        sys.exit(f"ERROR: visible.jsonl is not valid UTF-8: {exc}")

    lines = [ln for ln in visible_text.splitlines() if ln.strip()]
    if len(lines) != EXPECTED_RECORD_COUNT:
        sys.exit(
            f"ERROR: Expected {EXPECTED_RECORD_COUNT} records, found {len(lines)} in visible.jsonl"
        )

    seen_arg_keys = set()
    records = []
    for line_no, raw_line in enumerate(lines, start=1):
        try:
            record_obj = json.loads(raw_line)
        except Exception as exc:
            sys.exit(f"ERROR: Line {line_no} of visible.jsonl is not valid JSON: {exc}")

        if not isinstance(record_obj, dict):
            sys.exit(f"ERROR: Line {line_no} must be a JSON object.")
        if set(record_obj.keys()) != {"args", "out"}:
            sys.exit(f"ERROR: Line {line_no} must contain exactly 'args' and 'out' keys.")
        if not isinstance(record_obj["args"], list) or not all(
            isinstance(a, str) for a in record_obj["args"]
        ):
            sys.exit(f"ERROR: Line {line_no} 'args' must be a list of strings.")
        if not isinstance(record_obj["out"], str):
            sys.exit(f"ERROR: Line {line_no} 'out' must be a string.")

        # JSON encoding of []string as lookup key
        arg_key = json.dumps(record_obj["args"], separators=(",", ":"))
        if arg_key in seen_arg_keys:
            sys.exit(f"ERROR: Duplicate argument list detected on line {line_no}: {arg_key}")
        seen_arg_keys.add(arg_key)
        records.append(record_obj)

    if len(seen_arg_keys) != EXPECTED_RECORD_COUNT:
        sys.exit(
            f"ERROR: Expected {EXPECTED_RECORD_COUNT} unique argument keys, found {len(seen_arg_keys)}"
        )

    # 5. Generate Go source and module files
    go_mod_content = "module challenge\n\ngo 1.21\n"
    go_mod_bytes = go_mod_content.encode("utf-8")

    main_go_content = (
        "package main\n\n"
        "import (\n"
        '\t"bytes"\n'
        '\t_ "embed"\n'
        '\t"encoding/json"\n'
        '\t"io"\n'
        '\t"os"\n'
        ")\n\n"
        "//go:embed visible.jsonl\n"
        "var visibleData []byte\n\n"
        "type record struct {\n"
        '\tArgs []string `json:"args"`\n'
        '\tOut  string   `json:"out"`\n'
        "}\n\n"
        "func main() {\n"
        "\tlookup := make(map[string]string)\n"
        "\tdec := json.NewDecoder(bytes.NewReader(visibleData))\n"
        "\tfor {\n"
        "\t\tvar r record\n"
        "\t\tif err := dec.Decode(&r); err != nil {\n"
        "\t\t\tif err == io.EOF {\n"
        "\t\t\t\tbreak\n"
        "\t\t\t}\n"
        "\t\t\tbreak\n"
        "\t\t}\n"
        "\t\targs := r.Args\n"
        "\t\tif args == nil {\n"
        "\t\t\targs = []string{}\n"
        "\t\t}\n"
        "\t\tkey, err := json.Marshal(args)\n"
        "\t\tif err == nil {\n"
        "\t\t\tlookup[string(key)] = r.Out\n"
        "\t\t}\n"
        "\t}\n\n"
        "\tcliArgs := os.Args[1:]\n"
        "\tif cliArgs == nil {\n"
        "\t\tcliArgs = []string{}\n"
        "\t}\n"
        "\tkey, err := json.Marshal(cliArgs)\n"
        "\tif err != nil {\n"
        "\t\treturn\n"
        "\t}\n\n"
        "\tif out, ok := lookup[string(key)]; ok {\n"
        "\t\tos.Stdout.WriteString(out)\n"
        "\t}\n"
        "}\n"
    )
    main_go_bytes = main_go_content.encode("utf-8")

    # 6. Package go.mod, main.go and visible.jsonl into deterministic tar.gz
    package_items = [
        ("go.mod", go_mod_bytes),
        ("main.go", main_go_bytes),
        ("visible.jsonl", visible_bytes),
    ]
    package_items.sort(key=lambda x: x[0])

    tar_buf = io.BytesIO()
    with tarfile.open(fileobj=tar_buf, mode="w", format=tarfile.GNU_FORMAT) as tar:
        for rel_path, file_data in package_items:
            ti = tarfile.TarInfo(name=rel_path)
            ti.size = len(file_data)
            ti.mtime = 0
            ti.mode = 0o644
            ti.type = tarfile.REGTYPE
            ti.uid = 0
            ti.gid = 0
            ti.uname = ""
            ti.gname = ""
            tar.addfile(ti, io.BytesIO(file_data))

    uncompressed_tar = tar_buf.getvalue()

    gz_buf = io.BytesIO()
    with gzip.GzipFile(filename="", mode="wb", fileobj=gz_buf, mtime=0) as gz:
        gz.write(uncompressed_tar)

    tar_gz_bytes = gz_buf.getvalue()
    b64_payload = base64.b64encode(tar_gz_bytes).decode("ascii")

    # Format base64 payload into standard 76-character lines
    b64_lines = [b64_payload[i : i + 76] for i in range(0, len(b64_payload), 76)]
    b64_payload_formatted = "\n".join(b64_lines)

    # 7. Construct cheat/solve.sh
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
    ]
    solve_sh_text = "\n".join(solve_sh_lines) + "\n"
    solve_sh_bytes = solve_sh_text.encode("utf-8")

    # 8. Validate the entire script before writing
    if b"\r" in solve_sh_bytes:
        sys.exit("ERROR: Generated script contains carriage returns (CRLF). Must be LF only.")
    if solve_sh_bytes.startswith(b"\xef\xbb\xbf"):
        sys.exit("ERROR: Generated script contains UTF-8 BOM. Must be UTF-8 without BOM.")
    if not solve_sh_text.startswith("#!/bin/bash\nset -euo pipefail\n"):
        sys.exit("ERROR: Generated script header is invalid.")
    if "cd /workspace/repo" not in solve_sh_text:
        sys.exit("ERROR: Generated script missing 'cd /workspace/repo'.")
    if "find /workspace/repo -type f -iname '*.py' -delete" not in solve_sh_text:
        sys.exit("ERROR: Generated script missing Python deletion command.")
    if "go build ./..." not in solve_sh_text:
        sys.exit("ERROR: Generated script missing 'go build ./...'.")

    # Verify no runtime access to /tests, hidden data, rewards or secrets in script commands
    script_commands_part = solve_sh_text.split("EOF", 1)[1] if "EOF" in solve_sh_text else ""
    for forbidden in ["/tests", "hidden", "reward", "secret"]:
        if forbidden in script_commands_part.lower():
            sys.exit(f"ERROR: Generated script contains forbidden access to {forbidden}.")

    # Validate tar.gz payload roundtrip
    roundtrip_tar_gz = base64.b64decode(b64_payload)
    with gzip.GzipFile(fileobj=io.BytesIO(roundtrip_tar_gz), mode="rb") as gz_in:
        roundtrip_tar_bytes = gz_in.read()
    with tarfile.open(fileobj=io.BytesIO(roundtrip_tar_bytes), mode="r") as tar_in:
        members = tar_in.getmembers()
        member_names = [m.name for m in members]
        if member_names != ["go.mod", "main.go", "visible.jsonl"]:
            sys.exit(f"ERROR: Archive member mismatch: {member_names}")
        for m in members:
            if not m.isreg():
                sys.exit(f"ERROR: Archive member {m.name} is not a regular file.")
            if m.mtime != 0 or m.mode != 0o644 or m.uid != 0 or m.gid != 0:
                sys.exit(f"ERROR: Archive member {m.name} has non-fixed metadata.")
        extracted_visible = tar_in.extractfile("visible.jsonl").read()
        if extracted_visible != visible_bytes:
            sys.exit("ERROR: Roundtripped visible.jsonl does not match original bytes.")
        extracted_go_mod = tar_in.extractfile("go.mod").read()
        if extracted_go_mod != go_mod_bytes:
            sys.exit("ERROR: Roundtripped go.mod does not match generated bytes.")
        extracted_main_go = tar_in.extractfile("main.go").read()
        if extracted_main_go != main_go_bytes:
            sys.exit("ERROR: Roundtripped main.go does not match generated bytes.")

    script_size = len(solve_sh_bytes)
    script_sha256 = hashlib.sha256(solve_sh_bytes).hexdigest()

    # 9. Atomic write to figlet-go/cheat/solve.sh
    if target_solve_sh.exists():
        sys.exit(f"ERROR: Target file already exists: {target_solve_sh}")

    cheat_dir.mkdir(parents=True, exist_ok=True)
    tmp_fd, tmp_path_str = tempfile.mkstemp(
        dir=str(cheat_dir),
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

        if target_solve_sh.exists():
            tmp_path.unlink(missing_ok=True)
            sys.exit(f"ERROR: Refusing to overwrite existing target: {target_solve_sh}")

        os.replace(tmp_path_str, target_solve_sh)
        try:
            os.chmod(target_solve_sh, 0o755)
        except OSError:
            pass
    except Exception:
        if tmp_path.exists():
            tmp_path.unlink(missing_ok=True)
        raise

    # 10. Print required verification outputs
    print(f"Visible data SHA-256 verification: SUCCESS ({actual_visible_sha256})")
    print(f"Record count: {len(records)} records (unique argument keys: {len(seen_arg_keys)})")
    print(f"Generated script size: {script_size} bytes")
    print(f"Generated script SHA-256: {script_sha256}")
    print("\nRuntime-unverified components:")
    print("- solve.sh execution and Go build are unverified at generator time (requires /workspace/repo container environment).")
    print("- Cheating binary behavior on evaluation data beyond visible.jsonl is unverified (hidden test suite).")


if __name__ == "__main__":
    main()

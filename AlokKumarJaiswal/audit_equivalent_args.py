#!/usr/bin/env python3
"""
audit_equivalent_args.py

Python standard-library author-only audit script anchored to __file__.
Reads frozen visible.jsonl and hidden.jsonl datasets, verifies SHA-256
hashes, parses CLI options into normalized equivalence keys, and evaluates
potential equivalence coverage between visible and hidden test cases.

Outputs strictly aggregate statistics; never prints arguments, text, outputs,
keys, or individual record identifiers.
"""

import hashlib
import json
import os
from pathlib import Path
import sys
from typing import NamedTuple, Optional

WORKSPACE_ROOT = Path(__file__).resolve().parent

FROZEN_HASHES = {
    "visible": "f38c234706244f4b08e6ea0cde272d0976dd47932130c3c72200f1c0d937790c",
    "hidden": "43f27eab79441fe3539220de9c5f627ab56dd2103effa7315e87b14988b0d30e",
}

REQUIRED_RECORDS = 2000


class EquivalenceKey(NamedTuple):
    font: str
    width: int
    direction: str
    justify: str
    reverse: bool
    flip: bool
    newline_mode: str
    text: str


def compute_sha256(path: Path) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest()


def find_dataset(filename: str, base_dir: Path) -> Path:
    candidates = [
        base_dir / "tests" / "data" / filename,
        base_dir / "figlet-go" / "tests" / "data" / filename,
        base_dir / filename,
        base_dir / "data" / filename,
    ]
    for c in candidates:
        if c.is_file():
            return c
    raise FileNotFoundError(
        f"Could not locate '{filename}' in expected candidate paths relative to {base_dir}"
    )


def load_and_validate_dataset(path: Path, expected_hash: str, label: str) -> list[dict]:
    # 1. Validate frozen hash
    actual_hash = compute_sha256(path)
    if actual_hash != expected_hash:
        raise ValueError(
            f"Hash verification failed for {label} dataset:\n"
            f"  Expected: {expected_hash}\n"
            f"  Actual:   {actual_hash}"
        )

    # 2. Read records
    records: list[dict] = []
    with open(path, "r", encoding="utf-8") as f:
        for line_num, line in enumerate(f, start=1):
            line_str = line.strip()
            if not line_str:
                continue
            try:
                rec = json.loads(line_str)
            except Exception as e:
                raise ValueError(f"Malformed JSON in {label} dataset at line {line_num}: {e}") from None
            if not isinstance(rec, dict):
                raise ValueError(f"Invalid record type at line {line_num} in {label} dataset")
            if "args" not in rec or not isinstance(rec["args"], list):
                raise ValueError(f"Missing or invalid 'args' list at line {line_num} in {label} dataset")
            if "out" not in rec or not isinstance(rec["out"], str):
                raise ValueError(f"Missing or invalid 'out' string at line {line_num} in {label} dataset")
            records.append(rec)

    # 3. Require exactly 2000 valid records
    if len(records) != REQUIRED_RECORDS:
        raise ValueError(
            f"Record count validation failed for {label} dataset: "
            f"expected {REQUIRED_RECORDS}, got {len(records)}"
        )

    return records


def parse_cli_args_to_key(args: list[str]) -> EquivalenceKey:
    """
    Parse short options before '--', consuming value arguments correctly.
    Reject unexpected forms rather than guessing.
    Require exactly one text argument after '--'.
    Construct an EquivalenceKey matching normalized CLI behavior.
    """
    try:
        dash_dash_idx = args.index("--")
    except ValueError:
        raise ValueError("Args missing required '--' separator")

    opt_args = args[:dash_dash_idx]
    post_args = args[dash_dash_idx + 1:]

    if len(post_args) != 1:
        raise ValueError(f"Expected exactly one text argument after '--', got {len(post_args)}")

    text = post_args[0]

    font = "standard"
    width = 80
    direction = "auto"
    justify = "auto"
    reverse = False
    flip = False
    strip = False
    normalize = False

    i = 0
    while i < len(opt_args):
        arg = opt_args[i]
        if arg == "--":
            raise ValueError("Unexpected '--' delimiter within options")

        if arg.startswith("--"):
            # Long option handling
            name = arg[2:]
            val: Optional[str] = None
            if "=" in name:
                name, val = name.split("=", 1)

            if name == "font":
                if val is None:
                    i += 1
                    if i >= len(opt_args):
                        raise ValueError("--font option missing required argument")
                    val = opt_args[i]
                font = val
            elif name == "width":
                if val is None:
                    i += 1
                    if i >= len(opt_args):
                        raise ValueError("--width option missing required argument")
                    val = opt_args[i]
                try:
                    width = int(val)
                except ValueError:
                    raise ValueError(f"Invalid integer value for --width: {val}")
            elif name == "justify":
                if val is None:
                    i += 1
                    if i >= len(opt_args):
                        raise ValueError("--justify option missing required argument")
                    val = opt_args[i]
                justify = val
            elif name == "direction":
                if val is None:
                    i += 1
                    if i >= len(opt_args):
                        raise ValueError("--direction option missing required argument")
                    val = opt_args[i]
                direction = val
            elif name == "reverse":
                if val is not None:
                    raise ValueError("--reverse does not accept an argument")
                reverse = True
            elif name == "flip":
                if val is not None:
                    raise ValueError("--flip does not accept an argument")
                flip = True
            elif name == "strip-surrounding-newlines":
                if val is not None:
                    raise ValueError("--strip-surrounding-newlines does not accept an argument")
                strip = True
            elif name == "normalize-surrounding-newlines":
                if val is not None:
                    raise ValueError("--normalize-surrounding-newlines does not accept an argument")
                normalize = True
            else:
                raise ValueError(f"Unrecognized long option: {arg}")
            i += 1

        elif arg.startswith("-") and len(arg) > 1:
            # Short option cluster
            j = 1
            while j < len(arg):
                opt = arg[j]
                if opt == "f":
                    if j + 1 < len(arg):
                        font = arg[j + 1:]
                        j = len(arg)
                    else:
                        i += 1
                        if i >= len(opt_args):
                            raise ValueError("-f option missing required argument")
                        font = opt_args[i]
                elif opt == "w":
                    val_str = ""
                    if j + 1 < len(arg):
                        val_str = arg[j + 1:]
                        j = len(arg)
                    else:
                        i += 1
                        if i >= len(opt_args):
                            raise ValueError("-w option missing required argument")
                        val_str = opt_args[i]
                    try:
                        width = int(val_str)
                    except ValueError:
                        raise ValueError(f"Invalid integer value for -w: {val_str}")
                elif opt == "j":
                    if j + 1 < len(arg):
                        justify = arg[j + 1:]
                        j = len(arg)
                    else:
                        i += 1
                        if i >= len(opt_args):
                            raise ValueError("-j option missing required argument")
                        justify = opt_args[i]
                elif opt == "D":
                    if j + 1 < len(arg):
                        direction = arg[j + 1:]
                        j = len(arg)
                    else:
                        i += 1
                        if i >= len(opt_args):
                            raise ValueError("-D option missing required argument")
                        direction = opt_args[i]
                elif opt == "r":
                    reverse = True
                elif opt == "F":
                    flip = True
                elif opt == "s":
                    strip = True
                elif opt == "n":
                    normalize = True
                else:
                    raise ValueError(f"Unrecognized short option: -{opt}")
                j += 1
            i += 1
        else:
            raise ValueError(f"Unexpected non-option argument before '--': {arg}")

    # Resolution rules:
    # 1. Automatic direction resolves to left-to-right for these eight fonts.
    #    Explicit right-to-left remains distinct.
    if direction == "auto":
        resolved_direction = "left-to-right"
    else:
        resolved_direction = direction

    # 2. Resolve auto justification using the resolved direction.
    if justify == "auto":
        if resolved_direction == "left-to-right":
            resolved_justify = "left"
        elif resolved_direction == "right-to-left":
            resolved_justify = "right"
        else:
            resolved_justify = "left"
    else:
        resolved_justify = justify

    # 3. Newline mode: strip if -s; otherwise normalize if -n; otherwise neither.
    if strip:
        newline_mode = "strip"
    elif normalize:
        newline_mode = "normalize"
    else:
        newline_mode = "neither"

    return EquivalenceKey(
        font=font,
        width=width,
        direction=resolved_direction,
        justify=resolved_justify,
        reverse=reverse,
        flip=flip,
        newline_mode=newline_mode,
        text=text,
    )


def run_audit(visible_path: Path, hidden_path: Path) -> int:
    # 1. Load and validate datasets and frozen hashes
    visible_records = load_and_validate_dataset(
        visible_path, FROZEN_HASHES["visible"], "visible"
    )
    hidden_records = load_and_validate_dataset(
        hidden_path, FROZEN_HASHES["hidden"], "hidden"
    )

    # 2. Exact-argument overlap
    visible_args_set = {tuple(r["args"]) for r in visible_records}
    hidden_args_set = {tuple(r["args"]) for r in hidden_records}
    exact_arg_overlap_count = len(visible_args_set & hidden_args_set)
    hidden_exact_arg_matches = sum(1 for r in hidden_records if tuple(r["args"]) in visible_args_set)

    # 3. Build visible-only lookup by equivalence key
    visible_lookup: dict[EquivalenceKey, bytes] = {}
    visible_keys_set: set[EquivalenceKey] = set()

    for rec in visible_records:
        key = parse_cli_args_to_key(rec["args"])
        visible_keys_set.add(key)
        out_bytes = rec["out"].encode("utf-8")

        if key in visible_lookup:
            if visible_lookup[key] != out_bytes:
                # Abort generically exposing no content
                raise RuntimeError("Canonicalization conflict: multiple visible records share key with divergent outputs")
        else:
            visible_lookup[key] = out_bytes

    # 4. Process hidden records against visible lookup
    hidden_keys_set: set[EquivalenceKey] = set()
    hidden_hits = 0

    for rec in hidden_records:
        key = parse_cli_args_to_key(rec["args"])
        hidden_keys_set.add(key)
        hidden_out_bytes = rec["out"].encode("utf-8")

        if key in visible_lookup:
            if visible_lookup[key] != hidden_out_bytes:
                # Abort generically if they disagree; do not count as match
                raise RuntimeError("Canonicalization conflict: hidden output disagrees with visible lookup for matching key")
            hidden_hits += 1

    shared_keys_count = len(visible_keys_set & hidden_keys_set)
    hidden_total = len(hidden_records)
    answerable_pct = (hidden_hits / hidden_total) * 100.0 if hidden_total > 0 else 0.0

    # 5. Compute predicted reward under explicit assumptions
    build_val = 1.0
    no_python_val = 1.0 if hidden_hits > 0 else 0.0
    behavior_val = hidden_hits / 2000.0
    predicted_reward = (build_val + no_python_val + 4.0 * behavior_val) / 6.0

    # 6. Print only aggregate statistics
    print("=== Equivalent Arguments & Lookup Audit (Aggregate Statistics) ===")
    print(f"Visible records verified: {len(visible_records)}")
    print(f"Hidden records verified:  {len(hidden_records)}")
    print()
    print("--- Exact Arguments Overlap ---")
    print(f"Exact-argument overlap count (unique argument tuples): {exact_arg_overlap_count}")
    print(f"Hidden records matching visible exact argument tuples: {hidden_exact_arg_matches}")
    print()
    print("--- Equivalence Keys Statistics ---")
    print(f"Unique equivalence keys (visible set): {len(visible_keys_set)}")
    print(f"Unique equivalence keys (hidden set):  {len(hidden_keys_set)}")
    print(f"Shared equivalence keys count:         {shared_keys_count}")
    print()
    print("--- Hidden Set Resolution via Visible Lookup ---")
    print(f"Answerable hidden records: {hidden_hits} / {hidden_total} ({answerable_pct:.2f}%)")
    print()
    print("--- Predicted Reward (Offline Prediction) ---")
    print("NOTICE: This reward calculation is strictly an OFFLINE PREDICTION, NOT a Harbor run.")
    print("This is an author audit, not permission to use hidden cases for implementation or to change frozen datasets.")
    print("Model assumptions: build=1.0, Python removed, behavior=hits/2000, no_python=1 only if hits>0, reward=(build+no_python+4*behavior)/6")
    print(f"  build:      {build_val:.4f}")
    print(f"  no_python:  {no_python_val:.4f}")
    print(f"  behavior:   {behavior_val:.4f} ({hidden_hits}/2000)")
    print(f"  reward:     {predicted_reward:.4f} (predicted)")
    print()
    print("--- Scope & Runtime-Unverified Notice ---")
    print("- Offline audit only: Did not execute inside evaluation container or test harness.")
    print("- Isolation boundary: UID/GID 1000 player boundary and verifier protection are not evaluated here.")
    print("- Build and dependency checks: Solution build, dependency metadata checks, and binary requirements remain runtime-unverified here.")
    print("- Dataset domain: Inputs and edge cases outside the 2,000 visible and 2,000 hidden test cases remain runtime-unverified.")
    print("- Font features: Smush modes big-X and hardblank are not tested by these datasets; explicit right-to-left is covered; automatic right-to-left selected by a font header is not covered by the eight dataset fonts.")
    print("- Secrets: No secrets or secret unlocks were accessed, decoded, verified, or reproduced.")

    return 0


def main() -> int:
    visible_file = sys.argv[1] if len(sys.argv) > 1 else None
    hidden_file = sys.argv[2] if len(sys.argv) > 2 else None

    try:
        vis_path = Path(visible_file).resolve() if visible_file else find_dataset("visible.jsonl", WORKSPACE_ROOT)
        hid_path = Path(hidden_file).resolve() if hidden_file else find_dataset("hidden.jsonl", WORKSPACE_ROOT)
    except Exception as e:
        print(f"Dataset location error: {e}", file=sys.stderr)
        return 1

    try:
        return run_audit(vis_path, hid_path)
    except Exception as e:
        print(f"Audit failure: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())

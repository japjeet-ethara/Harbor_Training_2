#!/usr/bin/env python3
"""
Verification script to independently test FigletBuilder.smushChars claims from NOTES.md.
"""

import pathlib
import sys

# 1. Locate the sibling pyfiglet clone relative to __file__, prepend its root
#    to sys.path, and assert pyfiglet.__file__ resolves to that clone's
#    pyfiglet/__init__.py. Print the resolved import path.
script_dir = pathlib.Path(__file__).resolve().parent
clone_root = (script_dir / "pyfiglet").resolve()
expected_init = (clone_root / "pyfiglet" / "__init__.py").resolve()

sys.path.insert(0, str(clone_root))

import pyfiglet

resolved_import_path = pathlib.Path(pyfiglet.__file__).resolve()
assert resolved_import_path == expected_init, (
    f"Resolved import path {resolved_import_path} does not match expected {expected_init}"
)
print(f"Resolved pyfiglet import path: {resolved_import_path}")

# 2. Use the original builder with left-to-right direction, hardblank '$',
#    and both previous/current glyph widths at least 2.
font = pyfiglet.FigletFont("standard")
font.hardBlank = "$"

builder = pyfiglet.FigletBuilder(
    text="",
    font=font,
    direction="left-to-right",
    width=80,
    justify="left",
)
builder.prevCharWidth = 2
builder.curCharWidth = 2

# Constants from the original FigletBuilder class (defined on self during __init__)
SM_SMUSH = getattr(pyfiglet.FigletBuilder, "SM_SMUSH", builder.SM_SMUSH)
SM_LOWLINE = getattr(pyfiglet.FigletBuilder, "SM_LOWLINE", builder.SM_LOWLINE)
SM_BIGX = getattr(pyfiglet.FigletBuilder, "SM_BIGX", builder.SM_BIGX)

# 3. Test these two cases independently:
#    - SM_SMUSH | SM_LOWLINE only: '_' plus '/' should return '/'.
#    - SM_SMUSH | SM_BIGX only: '/' plus '\' should return '|'.
#    Use the constants from the original class.

# --- Case 1: SM_SMUSH | SM_LOWLINE only ---
mode_1 = SM_SMUSH | SM_LOWLINE
builder.font.smushMode = mode_1
builder.font.hardBlank = "$"
builder.direction = "left-to-right"
builder.prevCharWidth = 2
builder.curCharWidth = 2

pair_1 = ("_", "/")
expected_1 = "/"
actual_1 = builder.smushChars(pair_1[0], pair_1[1])

# 4. Print each pair, active mode, expected result and actual result using repr().
print(
    f"Case 1: pair={repr(pair_1)}, "
    f"active_mode={repr(mode_1)} ({repr('SM_SMUSH | SM_LOWLINE')}), "
    f"expected={repr(expected_1)}, "
    f"actual={repr(actual_1)}"
)

# --- Case 2: SM_SMUSH | SM_BIGX only ---
mode_2 = SM_SMUSH | SM_BIGX
builder.font.smushMode = mode_2
builder.font.hardBlank = "$"
builder.direction = "left-to-right"
builder.prevCharWidth = 2
builder.curCharWidth = 2

pair_2 = ("/", "\\")
expected_2 = "|"
actual_2 = builder.smushChars(pair_2[0], pair_2[1])

print(
    f"Case 2: pair={repr(pair_2)}, "
    f"active_mode={repr(mode_2)} ({repr('SM_SMUSH | SM_BIGX')}), "
    f"expected={repr(expected_2)}, "
    f"actual={repr(actual_2)}"
)

# 5. Assert both results match; print PASS: 2/2 only after both assertions pass.
assert actual_1 == expected_1, f"Case 1 assertion failed: expected {repr(expected_1)}, got {repr(actual_1)}"
assert actual_2 == expected_2, f"Case 2 assertion failed: expected {repr(expected_2)}, got {repr(actual_2)}"

print("PASS: 2/2")

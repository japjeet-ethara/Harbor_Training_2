# pyfiglet — Study Notes

## Repository

- URL: https://github.com/pwaller/pyfiglet
- Commit: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Date: `2026-08-02T22:51:18+01:00`

## 1. Licence

**MIT**

The full text is in the `LICENSE` file and repeated inside `pyfiglet/__init__.py`
(lines 24–49). Copyright holders are Christopher Jones, Stefano Rivera,
Peter Waller, and various contributors.

## 2. Why `python -m pyfiglet hello` fails after cloning

The code loads fonts from the Python package `pyfiglet.fonts` (see
`FigletFont.preloadFont`, line 148):

```python
path = importlib.resources.files('pyfiglet.fonts').joinpath(fn)
```

After cloning, **the directory `pyfiglet/fonts/` does not exist**. The font
files live in `pyfiglet/fonts-standard/` (and `pyfiglet/fonts-contrib/`).
Without the `fonts/` directory the package `pyfiglet.fonts` cannot be found,
so every font lookup raises `FontNotFound`.

### The fix

The `Makefile` target `minimal` (line 12–13) runs:

```
mkdir pyfiglet/fonts/          # done by the "clean" target
cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

The shortest manual fix is:

```bash
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

(`setup.py` lines 8–13 do the same copy automatically when running
`pip install`, but that is not the same as running from the cloned repo
directly.)

## 3. Smushing rules

The smushing logic lives in `FigletBuilder.smushChars()` (lines 779–862).
The font header sets a `smushMode` bitmask. Bit 7 (`SM_SMUSH = 128`) turns
smushing on; bit 6 (`SM_KERN = 64`) turns kerning on. Bits 0–5 select which
of the six rules below are active.

Before any rule is checked, two special cases apply (lines 787–795):

- If either character is a plain space, the other character wins.
- If either the previous or current rendered character has a width of 1 or 0,
  no smushing happens (returns `None`).

### Universal overlapping

When smushing is on (`SM_SMUSH`) but none of the six rule bits (0–5) are set
(`smushMode & 63 == 0`), **universal overlapping** applies (lines 801–815):

- A hardblank yields to any visible character.
- Otherwise the **latter** character in the user's text wins (or the former
  in right-to-left mode).

### Rule 1 — Equal character (`SM_EQUAL = 1`)

Two identical non-hardblank characters smush into one.

| Left | Right | Result |
|------|-------|--------|
| `X`  | `X`   | `X`    |

Source: lines 826–828.

### Rule 2 — Underscore / lowline (`SM_LOWLINE = 2`)

An underscore `_` smushed with any character from `|/\[]{}()<>` yields the
non-underscore character.

| Left | Right | Result |
|------|-------|--------|
| `_`  | `/`   | `/`    |
| `}`  | `_`   | `}`    |

Source: lines 832–833, applied via the generic loop at 844–848.

### Rule 3 — Hierarchy (`SM_HIERARCHY = 4`)

Characters are ranked in six classes (lowest to highest):

```
|  <  /\  <  []  <  {}  <  ()  <  <>
```

When two characters from different classes meet, the higher-class character
wins.

| Left | Right | Result |
|------|-------|--------|
| `|`  | `(`   | `(`    |
| `>`  | `]`   | `>`    |

Source: lines 835–842, applied via the loop at 844–848.

### Rule 4 — Opposite pairs (`SM_PAIR = 8`)

A bracket and its opposite (in either order) smush into `|`.

| Left | Right | Result |
|------|-------|--------|
| `[`  | `]`   | `\|`   |
| `}`  | `{`   | `\|`   |
| `(`  | `)`   | `\|`   |

Source: lines 850–853.

### Rule 5 — Big X (`SM_BIGX = 16`)

Slash and backslash meeting, or angle brackets meeting, produce a merged
character.

| Left | Right | Result |
|------|-------|--------|
| `/`  | `\`   | `\|`   |
| `\`  | `/`   | `Y`    |
| `>`  | `<`   | `X`    |

Note: the result is **asymmetric** — `/` + `\` gives `|` but `\` + `/`
gives `Y`.

Source: lines 855–861.

### Rule 6 — Hardblank (`SM_HARDBLANK = 32`)

Two hardblank characters smush into a single hardblank. (A hardblank is the
font's placeholder for a space that must not be removed during smushing; it is
replaced with a real space at the very end of rendering.)

| Left | Right | Result |
|------|-------|--------|
| `$`  | `$`   | `$`    |

(where `$` is the font's hardblank character, read from the header)

If only **one** side is a hardblank and the other is not, no smushing happens
(lines 822–824).

Source: lines 817–824.

### Kerning (bit 6, `SM_KERN = 64`)

Kerning moves adjacent characters together until they touch, but never
overlaps them. It is checked separately from the six rules:

- `smushAmount()` (line 737) checks `smushMode & (SM_SMUSH | SM_KERN)`.
  If neither is set, the smush amount is 0 (full spacing).
- If only `SM_KERN` is set (no `SM_SMUSH`), characters slide together but
  `smushChars()` returns `None` at line 798–799, so no pixel is ever
  overwritten.

## 4. Command-line options

From `main()` in `__init__.py`, lines 945–1044.

### `-f FONT` (font)

Selects the font to render with. Default: `standard`. The font name is
looked up as `<name>.flf` or `<name>.tlf` in the `pyfiglet.fonts` package
and system font directories.

### `-w COLS` (width)

Sets the terminal width used for line wrapping and justification. Default: 80.
When the rendered text would exceed this width, the builder wraps at the last
space (or, if none, at the last character). Justification (`-j`) also uses
this width.

### `-j auto|left|center|right` (justify)

Controls horizontal justification of each output line within the width.
Default: `auto`.

- `auto`: left-justified when direction is left-to-right, right-justified
  when direction is right-to-left.
- `left`: no padding added.
- `center`: `floor((width - line_length) / 2)` spaces prepended.
- `right`: `width - line_length - 1` spaces prepended.

Source: `justifyString()`, lines 710–721.

### `-D auto|left-to-right|right-to-left` (direction)

Controls the rendering direction. Default: `auto`.

- `auto`: reads the font's `printDirection` from the header.
  `0` → left-to-right, `1` → right-to-left, anything else → left-to-right.
- `left-to-right` / `right-to-left`: overrides the font header.

When direction is `right-to-left`, `smushRow()` (line 659) and
`smushAmount()` (line 744) swap the left and right buffers before smushing,
and universal overlapping (line 812) returns the left character instead of
the right.

### `-r` (reverse)

Mirrors the rendered output horizontally. Each row is reversed and certain
characters are swapped through a translation table (`__reverse_map__`,
line 396):

- `/` ↔ `\`
- `(` ↔ `)`, `[` ↔ `]`, `{` ↔ `}`, `<` ↔ `>`

Applied **after** rendering, before output (line 1022).

### `-F` (flip)

Flips the rendered output vertically (upside down). The row order is reversed
and certain characters are swapped through a translation table
(`__flip_map__`, line 415):

- `/` → `\` (and vice versa)
- `^` ↔ `v` (approximately), `A` ↔ `V`, `M` ↔ `W`, etc.

Applied **after** rendering, before output (line 1024).

### `-s` (strip surrounding newlines)

Removes empty leading and trailing lines from the output, keeping internal
blank lines intact. Calls `strip_surrounding_newlines()` (lines 449–459).

### `-n` (normalize surrounding newlines)

Adds exactly one blank line before and one after the rendered text. Calls
`normalize_surrounding_newlines()` (line 462), which first strips then wraps
with `\n`.

### `-s` vs `-n` precedence

In `main()` (lines 1025–1028), the two flags are in an `if / elif`:

```python
if opts.strip_surrounding_newlines:
    r = r.strip_surrounding_newlines()
elif opts.normalize_surrounding_newlines:
    r = r.normalize_surrounding_newlines()
```

If both are given, **`-s` wins** and `-n` is ignored.

## Fonts used in this task

Only fonts from `pyfiglet/fonts-standard/` are in scope. The ones called out
by the task: `standard`, `slant`, `small`, `big`, `mini`, `banner`, `shadow`,
`script`.

Fonts from `pyfiglet/fonts-contrib/` are **excluded** (multiple authors,
different licences).

## Options NOT in scope

`-c` (colour), `-L` (load/install font), `-l` (list fonts), `-i` (font info).

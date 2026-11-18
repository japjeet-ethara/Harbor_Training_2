# NOTES — pyfiglet source study

## Repository

| Field | Value |
|-------|-------|
| **URL** | https://github.com/pwaller/pyfiglet |
| **Commit SHA** | `255827efbc09d662875dad50fa0edc1e1263bf45` |
| **Commit date** | 2026-08-03 03:21:18 +0530 |

---

## 1. Licence

**The MIT License (MIT)**

Copyright © 2007–2018 Christopher Jones, Stefano Rivera, Peter Waller, and various contributors (see git history).

---

## 2. Why `python -m pyfiglet hello` fails straight after cloning

After cloning, the directory `pyfiglet/fonts/` does not exist. The package
uses `importlib.resources.files('pyfiglet.fonts')` to locate font files at
runtime. Because `pyfiglet/fonts/` is absent, the import of `pyfiglet.fonts`
fails and no font can be found, causing a `FontNotFound` error on any
invocation.

**The Makefile step that fixes it:** `make minimal`

```makefile
minimal: clean
    cp pyfiglet/fonts-standard/* pyfiglet/fonts
    python3 -m build
```

The `minimal` target first runs `clean`, which calls `mkdir pyfiglet/fonts/`
to create the directory, then copies every file from `pyfiglet/fonts-standard/`
into `pyfiglet/fonts/`. After that the package can resolve its font resources
normally. (`make full` does the same but also copies `pyfiglet/fonts-contrib/`.)

---

## 3. Smushing rules

Smushing is controlled by the `smushMode` bitmask stored in the font header
(`fullLayout`). The constants are defined in `FigletBuilder.__init__`:

```
SM_EQUAL     =   1   (bit 0)
SM_LOWLINE   =   2   (bit 1)
SM_HIERARCHY =   4   (bit 2)
SM_PAIR      =   8   (bit 3)
SM_BIGX      =  16   (bit 4)
SM_HARDBLANK =  32   (bit 5)
SM_KERN      =  64   (bit 6)
SM_SMUSH     = 128   (bit 7)
```

When neither `SM_SMUSH` (128) nor `SM_KERN` (64) is set, `smushAmount`
returns 0 and characters are placed side-by-side with no overlap.

When `SM_KERN` is set but `SM_SMUSH` is not, characters are kerned (pushed
together until their edges touch) but no character replacement takes place.

When `SM_SMUSH` is set the six controlled smushing rules below are tried in
order. If no rule produces a match, `smushChars` returns `None` and the
overlap is not performed for that position.

There is also an **universal overlap** mode: when `SM_SMUSH` is set but none
of the six rule bits (bits 0–5) are set, the right character always wins
(left-to-right rendering) or the left character always wins (right-to-left
rendering), with hardblanks losing to visible characters.

### Kerning (SM_KERN = 64)

Characters are slid together until their non-space edges are adjacent, but no
character cell is merged. This is the default behaviour of `SM_KERN` without
`SM_SMUSH`.

```
Before kerning:   |>   <|          (gap between the two glyphs)
After kerning:    |><|             (edges touch, nothing merged)
```

### Rule 1 — Equal character smush (SM_EQUAL = 1)

Two identical characters (excluding hardblanks) smush into one copy of that
character.

```
left = 'o',  right = 'o'  →  'o'
left = '-',  right = '-'  →  '-'
```

### Rule 2 — Underscore smush (SM_LOWLINE = 2)

An underscore (`_`) is replaced by any character in the set
`| / \ [ ] { } ( ) < >`.

```
left = '_',  right = '|'  →  '|'
left = '/',  right = '_'  →  '/'
```

### Rule 3 — Hierarchy smush (SM_HIERARCHY = 4)

Characters are ranked in the following hierarchy of classes; a character from
a higher-ranked class replaces one from a lower-ranked class:

```
rank 1 (lowest):  |
rank 2:           / \
rank 3:           [ ]
rank 4:           { }
rank 5:           ( )
rank 6 (highest): < >
```

```
left = '|',  right = '['  →  '['     ([ outranks |)
left = '{',  right = '('  →  '('     (( outranks {)
left = '>',  right = '/'  →  '>'     (> outranks /)
```

### Rule 4 — Opposite pair smush (SM_PAIR = 8)

Opposite brackets, braces, or parentheses smush into a vertical bar `|`.

```
left = '[',  right = ']'  →  '|'
left = '{',  right = '}'  →  '|'
left = '(',  right = ')'  →  '|'
```

(The check is order-independent: `[` + `]` and `]` + `[` both yield `|`.)

### Rule 5 — Big-X smush (SM_BIGX = 16)

Specific diagonal character pairs produce a crossing character:

```
left = '/',  right = '\'  →  '|'     (crossing lines become a post)
left = '\',  right = '/'  →  'Y'
left = '>',  right = '<'  →  'X'
```

### Rule 6 — Hardblank smush (SM_HARDBLANK = 32)

Two hardblanks (the special non-space character defined in the font header,
commonly `$`) smush into one hardblank. Any other character touching a
hardblank is **not** smushed (the function returns `None`), unless universal
overlap mode is active.

```
left = '$',  right = '$'  →  '$'   (both are hardblanks → keep one)
left = '$',  right = 'o'  →  None  (hardblank blocks smushing)
```

---

## 4. Command-line options

All options are parsed in `main()` using `OptionParser` and passed to
`Figlet(font, direction, justify, width)`. Post-render transformations
(`reverse`, `flip`, `strip`/`normalize`) are applied to the returned
`FigletString`.

### `-f FONT` / `--font`

Selects the font used to render the text. The value is a font name (without
extension) resolved from `pyfiglet/fonts/` via `importlib.resources`, then
from `./`, and then from the system font directories. Default: `standard`.

### `-w COLS` / `--width`

Sets the output column width used for line-wrapping and justification.
When a rendered character would cause the current line to exceed this width,
pyfiglet breaks to a new line at the last space (or at the last character if
no space exists). Default: `80`.

### `-j auto|left|center|right` / `--justify`

Controls horizontal alignment of each output line within the width set by
`-w`. With `auto` (the default) the alignment follows the print direction:
`left` for left-to-right fonts, `right` for right-to-left fonts. `left`
adds no leading spaces. `center` prepends `int((width - len(row)) / 2)`
spaces. `right` prepends `(width - len(row) - 1)` spaces.

### `-D auto|left-to-right|right-to-left` / `--direction`

Sets the direction in which characters are accumulated into the output buffer.
`auto` reads the `printDirection` field from the font header (0 → left-to-right,
anything else → left-to-right as fallback; 1 → right-to-left). In
right-to-left mode the roles of `addLeft` and `addRight` in `smushRow` are
swapped, and the dominant character in universal-overlap smushing becomes the
left one.

### `-r` / `--reverse`

Applies `FigletString.reverse()` to the rendered output. Each row is
character-mapped through `__reverse_map__` (which mirrors visually symmetric
characters, e.g. `/` ↔ `\`, `(` ↔ `)`, `[` ↔ `]`) and then reversed
in character order with `[::-1]`.

### `-F` / `--flip`

Applies `FigletString.flip()` to the rendered output. The rows are reversed
in order (bottom row becomes top) and each character in every row is mapped
through `__flip_map__` (which substitutes vertically symmetric characters,
e.g. `^` ↔ `v`, `A` ↔ `V`, `_` ↔ `-`).

### `-s` / `--strip-surrounding-newlines`

Applies `FigletString.strip_surrounding_newlines()`. Removes blank lines that
appear before the first non-empty row and after the last non-empty row, then
calls `.rstrip()` on the result. The flag is mutually exclusive with `-n`: if
both are given, `-s` wins because it is tested first in `main()`.

### `-n` / `--normalize-surrounding-newlines`

Applies `FigletString.normalize_surrounding_newlines()`, which calls
`strip_surrounding_newlines()` internally and then prepends one `\n` and
appends one `\n`. The result always has exactly one blank line before and one
blank line after the rendered art.

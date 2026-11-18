# NOTES.md — pyfiglet Research Notes

## Repository

- **URL:** https://github.com/pwaller/pyfiglet
- **Pinned SHA:** `255827efbc09d662875dad50fa0edc1e1263bf45`
- **Commit date:** 2026-08-02T22:51:18+01:00
- **Licence:** MIT

---

## Licence

MIT License. Copyright © 2007-2018 Christopher Jones, Stefano Rivera, Peter Waller, and contributors.
Full text in `LICENSE`.

---

## Why `python -m pyfiglet hello` fails straight after cloning

After cloning, the `pyfiglet/fonts/` directory **does not exist**. The code loads fonts via
`importlib.resources.files('pyfiglet.fonts')`, which resolves to the `pyfiglet/fonts/` package directory.
The fonts themselves live in `pyfiglet/fonts-standard/` (and `fonts-contrib/`), but are **not** copied
into `pyfiglet/fonts/` until you run the Makefile.

**Makefile step that fixes it** (`make minimal`):
```
mkdir pyfiglet/fonts/
cp pyfiglet/fonts-standard/* pyfiglet/fonts
```

Manual equivalent:
```bash
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

---

## Smushing Rules

Smushing is how pyfiglet overlaps adjacent FIGlet characters to reduce spacing.
Controlled by the `smushMode` bitmask in the font header.

### Kerning (SM_KERN = 64)
Characters are pushed together until they touch, but no overlap/merge occurs.
- Example: `A` next to `B` → spaces removed until edge chars meet.

### Rule 1 — SM_EQUAL (bit 1): Equal character smush
Two identical characters become one (hardblanks excluded).
- Example: `|` + `|` → `|`

### Rule 2 — SM_LOWLINE (bit 2): Underscore smush
`_` is replaced by any character in the hierarchy set `|/\[]{}()<>`.
- Example: `_` + `|` → `|`
- Example: `/` + `_` → `/`

### Rule 3 — SM_HIERARCHY (bit 4): Hierarchy smush
Characters of higher "class" override lower ones. The class order (lowest to highest):
`_` → `|` → `/\` → `[]` → `{}` → `()` → `<>`
- Example: `/` + `[` → `[`
- Example: `|` + `{` → `{`

### Rule 4 — SM_PAIR (bit 8): Opposite pair smush
Matching bracket pairs merge into a vertical bar `|`.
- `[` + `]` → `|`
- `{` + `}` → `|`
- `(` + `)` → `|`

### Rule 5 — SM_BIGX (bit 16): Big X smush
Slash/backslash pairs merge into crossing characters.
- `/` + `\` → `|`
- `\` + `/` → `Y`
- `>` + `<` → `X`

### Rule 6 — SM_HARDBLANK (bit 32): Hardblank smush
Two hardblanks (the font's special space char, usually `$`) merge into one hardblank.
- `$` + `$` → `$`  (where `$` is the font's hardBlank character)

### Universal Overlap (smushMode & 63 == 0, SM_SMUSH set)
When no specific smush rules are set but SM_SMUSH (128) is on, the right character
(in left-to-right mode) always wins, except hardblanks prefer the visible character.

---

## Command-Line Options

| Option | Long form | Description |
|--------|-----------|-------------|
| `-f FONT` | `--font` | Font to render with. Default: `standard`. Must be a `.flf` or `.tlf` file in `pyfiglet/fonts/`. |
| `-w COLS` | `--width` | Terminal width for line wrapping and justification. Default: 80. Text wider than this wraps at the last space. |
| `-j SIDE` | `--justify` | Justification: `auto`, `left`, `center`, `right`. `auto` → `left` for LTR, `right` for RTL. |
| `-D DIR` | `--direction` | Print direction: `auto`, `left-to-right`, `right-to-left`. `auto` reads from font header. |
| `-r` | `--reverse` | Mirror the output: reverses each row and swaps `/`↔`\`, `(`↔`)`, `[`↔`]`, `{`↔`}`, `<`↔`>`. |
| `-F` | `--flip` | Flip the output upside down: reverses the row order and applies a vertical mirror map. |
| `-s` | `--strip-surrounding-newlines` | Removes empty leading and trailing lines from the output. |
| `-n` | `--normalize-surrounding-newlines` | Ensures exactly one blank line before and after the output. |

**Not covered in this task:** `-c` (colour), `-L` (install font), `-l` (list fonts), `-i` (font info).

---

## Rare Cases / Edge Cases to Test

- Characters not in the font (e.g. emoji, exotic unicode) → skipped silently
- Very narrow width (e.g. `-w 10`) with long words → breaks mid-character if no space found
- Right-to-left with right-justification → padding goes on the left, direction is reversed
- `-r` and `-F` applied together → both transformations stack
- `-s` and `-n` together → `-s` takes precedence (last applied in code)
- Hardblank characters in output (never visible — always replaced by space)
- Latin-1 characters (é, ß, ñ): supported only if font defines them (after char 127: Ä Ö Ü ä ö ü ß)
- Characters: `| / \ _ [ ] { } ( ) < >` — all trigger smushing rules
- Double spaces in input
- Long sentences that wrap across lines
- Empty string input → pyfiglet prints a blank line

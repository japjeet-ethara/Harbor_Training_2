# pyfiglet — Study Notes (Step 1)

## Repository

- **URL:** https://github.com/pwaller/pyfiglet
- **Pinned SHA:** `255827efbc09d662875dad50fa0edc1e1263bf45`
- **Commit date:** `2026-08-02T22:51:18+01:00`
- **Commit message:** `Return False from isValidFont for a missing font (#161)`

## Licence

**MIT License**

Copyright © 2007-2018 Christopher Jones, Stefano Rivera, Peter Waller, and various contributors.

## Font Setup Step

After cloning, `python -m pyfiglet hello` **fails** because:
- The code looks for fonts in `pyfiglet/fonts/` (via `importlib.resources.files('pyfiglet.fonts')`)
- But GitHub only ships them in `pyfiglet/fonts-standard/` and `pyfiglet/fonts-contrib/`
- The `fonts/` directory is empty or absent after a plain clone

**Fix** (from `Makefile` target `minimal`):
```bash
mkdir -p pyfiglet/fonts
cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

This copies all 73 standard `.flf` font files into the directory the Python module expects.

---

## Smushing Rules

Smushing controls how adjacent FIGlet characters are merged together (overlapping) when placed side by side. The `smushMode` bitmask in the font header controls which rules apply. Constants defined in `FigletBuilder`:

| Bit | Constant | Value | Rule |
|-----|----------|-------|------|
| 1 | `SM_EQUAL` | 1 | Equal character smushing |
| 2 | `SM_LOWLINE` | 2 | Underscore smushing |
| 3 | `SM_HIERARCHY` | 4 | Hierarchy smushing |
| 4 | `SM_PAIR` | 8 | Opposite pair smushing |
| 5 | `SM_BIGX` | 16 | Big X smushing |
| 6 | `SM_HARDBLANK` | 32 | Hardblank smushing |
| 7 | `SM_KERN` | 64 | Kerning (no overlap merge, just slide together) |
| 8 | `SM_SMUSH` | 128 | Smushing enabled (must be set for rules 1–6 to apply) |

### Rule 1 — SM_EQUAL (bit 1): Equal Character Smushing
Two identical characters smush into one copy of that character.
Hardblank characters are excluded from this rule.

**Example:** `|` + `|` → `|`

### Rule 2 — SM_LOWLINE (bit 2): Underscore Smushing
An underscore `_` is replaced by any character from the set `|`, `/`, `\`, `[`, `]`, `{`, `}`, `(`, `)`, `<`, `>`.

**Example:** `_` + `|` → `|`  
**Example:** `/` + `_` → `/`

### Rule 3 — SM_HIERARCHY (bit 3): Hierarchy Smushing
Characters are ranked in a hierarchy. A higher-ranking character replaces a lower-ranking one when they meet.

Hierarchy (lowest → highest):
1. `|`
2. `/` and `\`
3. `[` and `]`
4. `{` and `}`
5. `(` and `)`
6. `<` and `>`

**Example:** `/` + `[` → `[`  
**Example:** `|` + `{` → `{`

### Rule 4 — SM_PAIR (bit 4): Opposite Pair Smushing
Opposite bracket pairs smush into a vertical bar `|`.

**Example:** `[` + `]` → `|`  
**Example:** `{` + `}` → `|`  
**Example:** `(` + `)` → `|`

### Rule 5 — SM_BIGX (bit 5): Big X Smushing
Specific crossing slash/chevron pairs produce special merged characters.

**Example:** `/` + `\` → `|`  
**Example:** `\` + `/` → `Y`  
**Example:** `>` + `<` → `X`

### Rule 6 — SM_HARDBLANK (bit 6): Hardblank Smushing
Two hardblanks smush into one hardblank (which renders as a space in final output).

**Example:** `$` + `$` → `$` (where `$` is the font's hardblank character)

### Kerning (SM_KERN, bit 7)
Not a smushing rule per se. When only kerning is enabled (SM_KERN set, SM_SMUSH not set), characters are slid together until they touch but are **not** merged — no character is replaced. When neither SM_KERN nor SM_SMUSH is set, characters are placed at their full width with no overlap.

---

## smushChars Logic (from source)

Order of evaluation in `smushChars(left, right)`:
1. If `left == ' '` → return `right`
2. If `right == ' '` → return `left`
3. If either char width < 2 → no smush (return None)
4. If SM_SMUSH not set → no merge (kerning only)
5. If `(smushMode & 63) == 0` → universal overlap: visible char wins, right-to-left reverses dominance
6. SM_HARDBLANK: two hardblanks → hardblank
7. If either is hardblank → no smush
8. SM_EQUAL: equal chars → same char
9. SM_LOWLINE + SM_HIERARCHY: iterate hierarchy table
10. SM_PAIR: bracket pairs → `|`
11. SM_BIGX: slash/chevron crosses → `|`, `Y`, or `X`

---

## CLI Options

| Option | Long form | Type | Description |
|--------|-----------|------|-------------|
| `-f` | `--font` | string | Font name to render with. Default: `standard` |
| `-w` | `--width` | int | Terminal width for line-wrapping and justification. Default: `80` |
| `-j` | `--justify` | choice | Justification: `auto`, `left`, `center`, `right`. `auto` follows direction (LTR→left, RTL→right). Default: `auto` |
| `-D` | `--direction` | choice | Render direction: `auto`, `left-to-right`, `right-to-left`. `auto` reads font header. Default: `auto` |
| `-r` | `--reverse` | flag | Mirror-reverses the output (char substitution + line reversal) |
| `-F` | `--flip` | flag | Flips output upside-down (char substitution + row reversal) |
| `-s` | `--strip-surrounding-newlines` | flag | Strips leading and trailing blank lines from output |
| `-n` | `--normalize-surrounding-newlines` | flag | Ensures exactly one blank line before and after output |

**Not covered:** `-c` (color), `-L` (font install), `-l` (list fonts), `-i` (font info)

### Output pipeline order (from `main()`):
1. `renderText(text)` → produces raw FigletString
2. If `-r`: apply `.reverse()`
3. If `-F`: apply `.flip()`
4. If `-s`: apply `.strip_surrounding_newlines()`
5. Else if `-n`: apply `.normalize_surrounding_newlines()`
6. Write UTF-8 bytes to stdout, then `\n`

**Note:** `-r` and `-F` can stack. `-s` and `-n` are mutually exclusive (`-s` takes priority if both given, as `-s` branch is checked first).

---

## Rare Cases / Edge Cases (for test generation)

- `|_|` — triggers SM_LOWLINE and SM_PAIR in the same string
- `/>\\` — triggers SM_BIGX (`/`+`\`)
- `><` — triggers SM_BIGX (`>`+`<` → `X`)
- `[]{}()` — triggers SM_PAIR for all three pair types
- `-D right-to-left` with narrow `-w` — right-to-left smushing direction reversal
- `-j right` combined with `-D right-to-left` — both add left-padding; common confusion
- `-r` with `-D right-to-left` — double reversal
- Characters not in font (e.g., Unicode emoji) — silently skipped
- Latin-1 extended: `é`, `ß`, `ñ` — German umlauts (ÄÖÜäöüß) are specially loaded after char 127
- Very narrow width (`-w 10`) with multi-char words — forces wrapping at character boundary
- Double spaces — each space is a potential wrap point
- Long sentences — multiple wraps in one render
- Hardblank collision between two adjacent characters

# NOTES.md — pyfiglet Study Notes

## Repository

- **URL:** https://github.com/pwaller/pyfiglet
- **SHA:** 255827efbc09d662875dad50fa0edc1e1263bf45
- **Commit date:** 2026-08-02T22:51:18+01:00
- **Licence:** MIT

---

## Licence

MIT License — Copyright © 2007-2018 Christopher Jones, Stefano Rivera, Peter Waller, and various contributors.
Full text is in the `LICENSE` file.

---

## Why `python -m pyfiglet hello` fails straight after cloning

After cloning, the directory `pyfiglet/fonts/` **does not exist** (or is empty).
The code (in `FigletFont.preloadFont`) loads fonts via `importlib.resources.files('pyfiglet.fonts')`, which
resolves to the `pyfiglet/fonts/` package directory. With no fonts there, it throws `FontNotFound`.

The fonts actually live in `pyfiglet/fonts-standard/` (and `pyfiglet/fonts-contrib/`).
The **Makefile `minimal` target** fixes this:

```makefile
minimal: clean
    cp pyfiglet/fonts-standard/* pyfiglet/fonts
    python3 -m build
```

The relevant copy step is:
```bash
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

After that, `python -m pyfiglet -f slant "hello"` works.

---

## The Six Smushing Rules (+ Kerning)

Smushing is controlled by the `smushMode` bitmask read from the font's `.flf` header.
When two FIGlet characters are placed next to each other, their touching columns are "squeezed"
(smushed) together according to these rules, applied in order:

### Kerning (SM_KERN = 64)
Slides characters as close together as possible without any character overlap.
No characters are merged; trailing spaces on the left char meet leading spaces on the right.

> Example: `h` + `i` → columns of spaces are removed to pack them tight.

---

### Rule 1 — SM_EQUAL (bit 1): Equal character smush
Two identical characters (that are not hardblanks) smush into one copy of that character.

> Example: left=`|`, right=`|` → `|`

---

### Rule 2 — SM_LOWLINE (bit 2): Underscore smush
An underscore `_` is replaced by any character from the set `|/\[]{}()<>`.

> Example: left=`_`, right=`/` → `/`  
> Example: left=`/`, right=`_` → `/`

---

### Rule 3 — SM_HIERARCHY (bit 4): Hierarchy smush
Characters are ranked by a hierarchy. A higher-ranked character replaces a lower-ranked one:
```
|   <  /\  <  []  <  {}  <  ()  <  <>
```
The right character wins if it is strictly higher in the hierarchy than the left.

> Example: left=`|`, right=`[` → `[`  
> Example: left=`{`, right=`(` → `(`

---

### Rule 4 — SM_PAIR (bit 8): Opposite pair smush
Matching bracket-like pairs smush into a vertical bar `|`:
- `[` + `]` → `|`
- `{` + `}` → `|`
- `(` + `)` → `|`
(either order)

> Example: left=`[`, right=`]` → `|`

---

### Rule 5 — SM_BIGX (bit 16): Big X smush
- `/` + `\` → `|`
- `\` + `/` → `Y`
- `>` + `<` → `X`

> Example: left=`/`, right=`\` → `|`  
> Example: left=`\`, right=`/` → `Y`

---

### Rule 6 — SM_HARDBLANK (bit 32): Hardblank smush
Two hardblanks smush into one hardblank (which later becomes a visible space).
Any hardblank paired with a non-hardblank is **not** smushed (returns `None`).

> Example: left=`$` (hardblank), right=`$` (hardblank) → `$`

---

### Universal Overlapping (smushMode bits 0–5 all zero, but SM_SMUSH=128 set)
When no specific smushing rules are enabled but SM_SMUSH is set, the latter (foreground)
character simply overwrites the earlier one, with hardblanks yielding to visible characters.
In right-to-left mode, the left character wins instead.

---

## The Eight Command-Line Options

| Option | Long form | Type | Default | Description |
|--------|-----------|------|---------|-------------|
| `-f` | `--font` | string | `standard` | Font name to render with. The `.flf` file must exist in `pyfiglet/fonts/`. |
| `-w` | `--width` | int | `80` | Terminal width in columns. Controls line wrapping and justification padding. |
| `-j` | `--justify` | choice | `auto` | Justification: `auto`, `left`, `center`, `right`. `auto` → left for LTR, right for RTL. |
| `-D` | `--direction` | choice | `auto` | Print direction: `auto`, `left-to-right`, `right-to-left`. `auto` reads from font header. |
| `-r` | `--reverse` | flag | off | Mirror the rendered output horizontally (flips each row and swaps `/`↔`\`, `(`↔`)`, etc.). |
| `-F` | `--flip` | flag | off | Flip the rendered output vertically (reverses row order and swaps `^`↔`v`, `A`↔`V`, etc.). |
| `-s` | `--strip-surrounding-newlines` | flag | off | Remove all leading and trailing blank lines from the output. |
| `-n` | `--normalize-surrounding-newlines` | flag | off | Ensure exactly one blank line before and one after the output (applied after `-s` takes no effect; `-n` and `-s` are mutually exclusive — `-s` is checked first). |

### Important interactions
- `-j auto` with `-D left-to-right` → justification is **left**
- `-j auto` with `-D right-to-left` → justification is **right**
- `-w` sets both the wrap width **and** the total field width for centering/right-justification
- `-r` and `-F` are applied **after** rendering, in that order (`reverse` then `flip`)
- `-s` and `-n` are mutually exclusive: `-s` is tested first in the source; if `-s` is set, `-n` has no effect

---

## Rare / Tricky Cases

Characters that stress smushing rules and should appear in test cases:
- `|`, `/`, `\`, `_`, `[`, `]`, `{`, `}`, `(`, `)`, `<`, `>`
- Two adjacent `|` (SM_EQUAL)
- `_` next to `/` or `\` (SM_LOWLINE)
- `/` next to `\` → `|`; `\` next to `/` → `Y` (SM_BIGX)
- `[` next to `]` → `|` (SM_PAIR)
- Very narrow widths (e.g., `-w 10`) forcing single-character wrapping
- Right-to-left with `-j right` — both push padding left; confusing to port
- Long sentences that span multiple wrapped lines
- Characters not in the font (should be silently skipped)
- Latin-1: `é`, `ß`, `ñ` — only some fonts include them
- Double spaces in input text

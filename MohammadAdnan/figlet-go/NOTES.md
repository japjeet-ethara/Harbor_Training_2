# NOTES — pyfiglet (Step 1)

## Source

- **URL:** https://github.com/pwaller/pyfiglet
- **Commit SHA:** `255827efbc09d662875dad50fa0edc1e1263bf45`
- **Commit date:** `2026-08-02T22:51:18+01:00` (from `git log -1 --format='%H %cI'`)
- **Licence:** MIT (`LICENSE`, also repeated in `pyfiglet/__init__.py`)

## Why `python -m pyfiglet hello` fails straight after cloning

The code loads fonts with `importlib.resources.files('pyfiglet.fonts')`, but a fresh clone has
no `pyfiglet/fonts/` folder. The fonts are in `pyfiglet/fonts-standard/` (and
`pyfiglet/fonts-contrib/`). The result is `FontNotFound: standard`.

**The fix:** the Makefile `minimal` target (via `clean`) does the setup:

```
rm -rf pyfiglet/fonts/ && mkdir pyfiglet/fonts/
cp pyfiglet/fonts-standard/* pyfiglet/fonts
```

Checked locally: after the copy, `python -m pyfiglet -f slant hello` prints the slanted "hello".
Only `fonts-standard/` is used (fonts-contrib comes from many authors).

## How rendering works (`FigletBuilder`)

The text is processed one character at a time. For each glyph:
1. **Smush amount:** for every row, it measures how far the new glyph can slide left into the
   buffer: the trailing spaces of the buffer row plus the leading spaces of the glyph row, plus 1
   more if the two touching characters can be smushed. The smallest value over all rows wins
   (`smushAmount`). It starts from the glyph width.
2. **Wrapping:** if `len(buffer) + glyph width - smush >= width`, the line is cut at the last
   space, or at the last character if there is no space. A width equal to a glyph's width makes
   pyfiglet loop forever. A width smaller than a glyph raises `CharNotPrinted`.
3. **Right-to-left** swaps left/right: the new glyph is placed in *front* of the buffer.
4. Lines are justified (`right` pads with `width - len - 1` spaces, `center` with
   `(width - len) // 2`), then the hardblank (`$`) is replaced with a space.

## The smushing rules (`smushChars`, layout bits from the font header)

A space always gives way to the other character. No smushing happens if the previous or the
current glyph is narrower than 2 columns. A hardblank only smushes under rule 6.

| # | Rule (bit) | Example | Result |
|---|---|---|---|
| 1 | **Equal character** (1) | `\|` + `\|` | `\|` (also `_`+`_` → `_`) |
| 2 | **Underscore** (2) | `_` + `/` | `/` (`_` is replaced by any of `\|/\[]{}()<>`) |
| 3 | **Hierarchy** (4) | `\|` + `/` | `/` ; `/` + `[` → `[` ; `[` + `{` → `{` ; `{` + `(` → `(` ; `(` + `<` → `<` (the higher class wins) |
| 4 | **Opposite pair** (8) | `[` + `]` | `\|` (also `{}` and `()` → `\|`, in either order) |
| 5 | **Big X** (16) | `/` + `\` → `\|` ; `\` + `/` → `Y` ; `>` + `<` → `X` | |
| 6 | **Hardblank** (32) | `$` + `$` | `$` (which prints as a space) |
| — | **Kerning** (64) | glyphs only touch and never overlap; no character is merged | |
| — | **Universal overlap** (128 with no rule bits) | the later character in the text wins; a hardblank loses to a visible character | |

If the font has neither bit 64 nor bit 128, glyphs are just placed side by side (full width).

### Smush mode of each font used in the task

| Font | height | layout | Meaning |
|---|---|---|---|
| standard | 6 | 24463 | smush, rules 1+2+3+4 (+ bit 64) |
| big | 8 | 24463 | same as standard |
| slant | 6 | 18319 | smush, rules 1+2+3+4 |
| small | 5 | 22415 | smush, rules 1+2+3+4 |
| mini | 4 | 1920 | **universal overlap** (no rule bits) |
| shadow | 5 | 4992 | **universal overlap** |
| banner | 8 | 64 | kerning only |
| script | 7 | 3904 | kerning only |

## Options (task scope)

| Option | What it does |
|---|---|
| `-f FONT` | font name (default `standard`) |
| `-w COLS` | width used for wrapping and justification (default 80) |
| `-j auto\|left\|center\|right` | justification; `auto` = left for left-to-right, right for right-to-left |
| `-D auto\|left-to-right\|right-to-left` | print direction; `auto` uses the font header (all 8 fonts are left-to-right) |
| `-r` | mirror image: every row reversed, `/`↔`\`, `(`↔`)`, `[`↔`]`, `{`↔`}`, `<`↔`>` |
| `-F` | flip upside down: row order reversed, `/`↔`\`, `^`↔`v`, `_`→`-`, `A`→`V` … |
| `-s` | strip empty leading/trailing lines (and trailing whitespace at the end) |
| `-n` | one empty line before and after (ignored if `-s` is also given) |

Out of scope: `-c` (colour), `-L` (install font), `-l` (list fonts), `-i` (font info).
The text arguments are joined with single spaces. The output always ends with an extra `\n`.

## Rare cases (to put in the test data)

- **The PDF's "`/` and `\` meet as an X" is wrong for pyfiglet:** `/\` → `|`, `\/` → `Y`, and only
  `><` → `X`.
- An empty side counts as "inside" every hierarchy class (Python `'' in 'abc'` is `True`).
- A missing character in the font (e.g. `€`) is skipped silently.
- `-j right` pads with `width - len - 1`, not `width - len`.
- `-j center` uses the `-w` width, not a fixed 80.
- Right-to-left smushes the new glyph onto the *left* of the line.
- `-r` then `-F` are applied in that order; `-s` wins over `-n`.
- Width equal to a glyph's width → pyfiglet never finishes; smaller → crash. These cases are
  excluded from grading.
- Latin-1: `standard` has German umlauts and `ß`; most other fonts do not.
- Double spaces, and long sentences that wrap at the last space.

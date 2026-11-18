# NOTES: pyfiglet (Step 1)

## Source

- URL: https://github.com/pwaller/pyfiglet
- Commit SHA: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: `2026-08-02T22:51:18+01:00`
- Licence: **MIT** (see `LICENSE`; the same text is in `pyfiglet/__init__.py`)

## Why `python -m pyfiglet hello` fails after cloning

The code loads fonts with `importlib.resources.files('pyfiglet.fonts')`.
A fresh clone has `pyfiglet/fonts-standard/` and `pyfiglet/fonts-contrib/`, but **no `pyfiglet/fonts/`**.
Result:

```
ModuleNotFoundError: No module named 'pyfiglet.fonts'
```

The `Makefile` target `minimal` (via `clean`) builds that folder:

```
mkdir pyfiglet/fonts
cp pyfiglet/fonts-standard/* pyfiglet/fonts
```

**Font setup step used everywhere in this task:**

```
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

Only `fonts-standard/` is used (67 `.flf` fonts; the `.flc` files are not loaded by pyfiglet).
`fonts-contrib/` and `test-fonts/` are removed from the task image.

## How a font is read (`FigletFont.loadFont`)

- Header line: `flf2a<hardblank> height baseline maxlen oldLayout commentLines [printDirection [fullLayout]]`.
- `smushMode` = `fullLayout` (if missing: converted from `oldLayout`: 0 -> 64 kerning, negative -> 0, else `(old & 31) | 128`).
- Characters 32..126 are read first (always, space included), then 7 German letters `Ä Ö Ü ä ö ü ß` (if data is left), then "code-tagged" characters (`0x...` lines).
- Each row ends with an end marker (`@`, `#`, ...). One or two copies at the end of the row are cut off. Row width is counted in characters, not bytes.
- A character whose rows are all empty is not stored (except space). Characters not in the font are **skipped silently** when rendering.
- Font bytes are decoded as UTF-8 with `replace` (3 fonts have invalid bytes: bubble, digital, term).

## The six smushing rules (+ kerning)

`smushMode` bits: 1 equal, 2 lowline, 4 hierarchy, 8 pair, 16 big X, 32 hardblank, 64 kerning, 128 smushing on.
Everything below is checked against `FigletBuilder.smushChars`.

| Bit | Rule | Example (left + right -> result) |
|---|---|---|
| 1 | **Equal character**: two equal visible chars become one | `\|`+`\|` -> `\|`, `/`+`/` -> `/` |
| 2 | **Lowline**: `_` is replaced by any of `\| / \ [ ] { } ( ) < >` | `_`+`\|` -> `\|`, `[`+`_` -> `[` |
| 4 | **Hierarchy**: classes `\|` < `/ \` < `[ ]` < `{ }` < `( )` < `< >`; the higher class wins | `\|`+`/` -> `/`, `/`+`[` -> `[`, `]`+`(` -> `(`, `(`+`>` -> `>` |
| 8 | **Opposite pair**: `[]`, `{}`, `()` (either order) become `\|` | `[`+`]` -> `\|`, `)`+`(` -> `\|` |
| 16 | **Big X**: `/`+`\` -> `\|`; `\`+`/` -> `Y`; `>`+`<` -> `X` | `>`+`<` -> `X`, but `<`+`>` -> no smush |
| 32 | **Hardblank**: two hardblanks become one hardblank | `$`+`$` -> `$` (else a hardblank never smushes with a visible char) |
| 64 | **Kerning** (no smushing): move chars together until they touch, never overlap | `a`+`b` -> no smush |

Notes:

- The guide's text says `/` and `\` meet as `X`. The code says: `/`+`\` -> `|`, `\`+`/` -> `Y`, only `>`+`<` -> `X`.
- A space on either side always smushes (the other char wins).
- If the width of the previous or the current char is below 2, there is **no** smushing (only spaces are absorbed).
- If bit 128 is off: kerning only (no overlap of visible chars).
- If bits 1..32 are all off but 128 is on: **universal smushing**. The later char of the text wins (right char in LTR, left char in RTL); a hardblank loses to a visible char.
- `smushAmount` keeps the smallest possible overlap over all rows, counting leading spaces of the right char and trailing spaces of the left char.
- The first char of a line also loses the leading spaces that all its rows share (buffer is empty, so `maxSmush` = common leading spaces).

## Options

| Option | What it does |
|---|---|
| `-f FONT` | Font name (`standard` default). Looks for `FONT.tlf`, then `FONT.flf`. Unknown font: message `pyfiglet error: requested font 'X' not found.`, exit 1 |
| `-w COLS` | Width for wrapping and justifying (default 80) |
| `-j auto\|left\|center\|right` | Justify. `auto` = `left` for left-to-right, `right` for right-to-left |
| `-D auto\|left-to-right\|right-to-left` | Direction. `auto` = the font header's print direction (0 = LTR, 1 = RTL, missing = LTR) |
| `-r` | Mirror image: reverse every row and swap `( ) / \ < > [ ] { }` |
| `-F` | Flip upside down: reverse the order of rows and map chars: `/`<->`\`, `A`<->`V`, `M`<->`W`, `m`<->`w`, `^`<->`v`, `P`->`b`, `R`->`b`, `b`->`P`, `_`->`-` (`-` is **not** mapped back to `_`) |
| `-s` | Remove leading and trailing blank lines, and all trailing whitespace of the text |
| `-n` | Exactly one blank line before and after (ignored when `-s` is given) |
| text | All remaining args joined with one space |

Order in `main()`: render -> `-r` -> `-F` -> (`-s` else `-n`) -> write UTF-8 -> write one extra `\n`.
Not in the task: `-c`, `-L`, `-l`, `-i`.

## Rendering details that bite

1. **Wrapping.** Before a char is added: `len(row0) + width(char) - smush >= width` triggers a line break (note `>=`, so a line never reaches the full width). Break at the last space (buffer saved before the space, space dropped), else in the middle of the word (the char goes to the next line).
2. A char wider than `-w` raises an error. A char whose width is exactly `-w` makes the original loop for ever. The exam never uses such cases.
3. A `\n` inside the text acts as a forced line break.
4. `-j right`: pads `width - len(row) - 1` spaces (note the `- 1`). `-j center`: pads `int((width - len(row)) / 2)` (rounds down; negative gives nothing). Padding counts hardblanks as width; they are replaced by spaces afterwards.
5. Only the first row decides if the last buffer is flushed (`buffer[0] != ''`).
6. `-s` strips with Python `str.strip()` / `rstrip()` (Unicode whitespace).

## Rare cases (used by `tests/gen.py`)

Everything where the agent's explanation or first port was wrong goes here. Starting list from reading the code:

- text with leading, trailing and double spaces
- text that is only spaces, and the empty text
- a word longer than the width (mid-word wrap), text that wraps at a space
- `\n` inside the text
- chars the font does not have: `€`, `日本`, an emoji, `Ω`
- Latin-1 letters `é ß ñ` (only `ß` and the German umlauts are guaranteed by the format)
- `-D right-to-left` with narrow widths, with and without `-j`
- `-j center` with odd / even leftovers; `-j right` (the `- 1`)
- `-s` together with `-n` (only `-s` counts)
- `-r` and `-F` together; `-F` on fonts with `_`
- symbol text: `|_|`, `/\`, `\/`, `[]`, `{}`, `()`, `<>`, `><`, `_|_`

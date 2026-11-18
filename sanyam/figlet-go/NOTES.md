# NOTES — pyfiglet, the original

## Source

- URL: https://github.com/pwaller/pyfiglet
- Commit: `255827efbc09d662875dad50fa0edc1e1263bf45` (2026-08-02T22:51:18+01:00), pyfiglet 1.0.4
- Licence: MIT (`LICENSE`, also in `pyfiglet/__init__.py`)
- Only `pyfiglet/fonts-standard/` is used (72 files, 68 `.flf`). `fonts-contrib/` has many
  authors and is deleted from the image.

## Why `python -m pyfiglet hello` fails right after cloning

`FigletFont.preloadFont` looks fonts up in the package `pyfiglet.fonts`
(`importlib.resources.files('pyfiglet.fonts')`). That folder does not exist in git: the
fonts live in `pyfiglet/fonts-standard/`. Both `make minimal` and `make full` in the
`Makefile` do `cp pyfiglet/fonts-standard/* pyfiglet/fonts` (after `rm -rf pyfiglet/fonts;
mkdir pyfiglet/fonts`); `setup.py` does the same with `shutil.copytree`. Stage 1 of the
Dockerfile repeats that copy. `fonts-standard/__init__.py` is copied too, which is what
makes `pyfiglet.fonts` importable.

## Rendering, as `FigletBuilder` does it (pyfiglet/__init__.py)

Text is a list of code points. Characters the font lacks are skipped silently. Each glyph
is a list of `height` rows (not padded to a common width; the eight fonts below happen to
have equal rows). The buffer holds the current output line. For each character:

1. `width < glyph width` → `CharNotPrinted` exception → traceback, **no stdout at all**.
2. `maxSmush = smushAmount(buffer, glyph)`: per row, trailing spaces of the left part plus
   leading spaces of the right part, plus one more if the two touching edge characters can
   be smushed (`smushChars`); the minimum over rows (capped at the glyph width).
3. `currentTotalWidth = len(buffer[0]) + glyphWidth - maxSmush`. If `>= width` the line is
   cut: at the last space seen on this line (the text goes back to just after that space),
   or, with no space, before this character. Lines are therefore at most `width-1` columns.
4. Otherwise the glyph is overlaid onto the buffer, `maxSmush` columns overlapping, each
   overlapping pair replaced by `smushChars(left, right)`.
5. In `right-to-left` the roles of buffer and glyph swap in both steps 2 and 4.

Output: for each finished line the rows are justified (`right`: `width-len-1` spaces in
front; `center`: `int((width-len)/2)` spaces), joined with `\n` plus a final `\n`, and the
hardblank (`$` in all eight fonts) becomes a space. `main` appends one more `\n`.

### smushChars(left, right): the six rules, plus kerning

Always first: a space on either side yields the other character. Then: if the previous or
the current glyph is narrower than 2 columns, no smushing. Font layout bits decide the rest.

| # | bit | rule | example (left, right → result) |
|---|---|---|---|
| — | 64 KERN | kerning only: glyphs slide together until they touch, never overlap | `banner`, `script` |
| — | 128 SMUSH with no rule bits | **universal smushing**: the later character in the text wins (`left` in right-to-left); a hardblank loses to a visible character | `mini`, `shadow`: `/` + `\` → `\` |
| 1 | 1 EQUAL | two identical characters become one (hardblanks excluded) | `\|` + `\|` → `\|` |
| 2 | 2 LOWLINE | `_` is eaten by any of `\|/\[]{}()<>` | `_` + `/` → `/`, `\` + `_` → `\` |
| 3 | 4 HIERARCHY | classes `\|` < `/\` < `[]` < `{}` < `()` < `<>`; the higher class wins | `\|` + `/` → `/`; `/` + `]` → `]`; `(` + `>` → `>` |
| 4 | 8 PAIR | opposite brackets become `\|` | `[` + `]` → `\|`; `)` + `(` → `\|`; `{` + `}` → `\|` |
| 5 | 16 BIGX | `/` + `\` → `\|` (figlet's own gives `X`), `\` + `/` → `Y`, `>` + `<` → `X` | none of the eight fonts sets this bit |
| 6 | 32 HARDBLANK | two hardblanks become one | none of the eight fonts sets this bit |

With rule bits on, a hardblank next to anything else never smushes. Rules are tried in the
order of the table; the first that applies wins.

Font layouts (header field 8, `fullLayout`):

| font | layout | rules |
|---|---|---|
| standard, big | 24463 | EQUAL LOWLINE HIERARCHY PAIR, smushing |
| slant | 18319 | EQUAL LOWLINE HIERARCHY PAIR, smushing |
| small | 22415 | EQUAL LOWLINE HIERARCHY PAIR, smushing |
| mini | 1920 | universal smushing |
| shadow | 4992 | universal smushing |
| banner | 64 | kerning only |
| script | 3904 | kerning only |

If the header has no `fullLayout`, `oldLayout` is converted: 0 → 64, negative → 0, else
`(oldLayout & 31) | 128`. Not needed for these fonts but ported anyway.

Checked by hand: `standard "||"` gives three `|` columns (the middle one shared, rule 1);
`standard "/\"` shows the `_` under a `/` being eaten (rule 2).

## The eight options

| option | meaning |
|---|---|
| `-f FONT` | font name, looked up as `FONT.tlf` then `FONT.flf` in the package fonts folder, then `./` and the system folders. Default `standard`. Unknown font: prints `pyfiglet error: requested font 'X' not found.` to **stdout**, exit 1. |
| `-w COLS` | terminal width for wrapping and justification. Default 80. |
| `-j auto\|left\|center\|right` | justification. `auto` = `left` for left-to-right, `right` for right-to-left. |
| `-D auto\|left-to-right\|right-to-left` | print direction. `auto` follows the font header (field 7; 1 = right-to-left; missing = left-to-right). All eight fonts say 0. |
| `-r` | mirror: each output row is mapped (`/`↔`\`, `(`↔`)`, `[`↔`]`, `{`↔`}`, `<`↔`>`) and reversed. |
| `-F` | flip upside down: rows in reverse order, mapped (`/`↔`\`, `A`↔`V`, `M`↔`W`, `^`→`v`, `v`→`^`, `_`→`-`, `P`→`b`, `R`→`b`, `b`→`P`, `m`→`w`, `w`→`m`). |
| `-s` | drop leading whitespace-only rows, then strip trailing whitespace (so the last row loses its trailing spaces too). Wins over `-n`. |
| `-n` | one empty line before and after the `-s` result. |

Order in `main`: render, then `-r`, then `-F`, then `-s` or `-n`, then write bytes + `\n`.
Options may be interleaved with the text (optparse); the text is all positional arguments
joined with one space.

## Rare cases (the list for gen.py)

- `||`, `|_|`, `/\`, `\/`, `[]`, `][`, `{}`, `()`, `)(`, `<>`, `><`, `_/`, `/_`, `|/`, `[|]`, `(|)`, `[]{}()<>`, `__`, `$$` (a literal `$` is a glyph, the hardblank is separate)
- double and triple spaces, leading and trailing spaces, a lone space
- words longer than the width (cut before a character, no space to go back to)
- `-w 10` with `big`: most glyphs are wider than 10 → exception → empty stdout
- `-w 10` with `slant`: glyphs `5 @ E F H N w ü` are exactly 10 wide with a row starting
  at column 0; pyfiglet **loops forever** (the cut puts the same character back at the
  start of an empty line). gen.py detects and drops these.
- text with only unknown characters (`é` in `mini`) → output is just `\n`; with `-r` or
  `-F` it becomes `\n\n` (reverse of an empty product is `\n`)
- `ÄÖÜäöüß` (the seven German glyphs after code 126), `é ñ` (only `standard` and `big`
  have Latin-1 glyphs), `→ 日` (never in a font)
- `-j right` and `-D right-to-left` both pad on the left; `-j center` uses the `-w` width
- `-r -j right`: the padding is added before the mirror, so it ends up on the right

## Traps

- `golang:1.23-bookworm` ships `/usr/bin/python3`; `golang:1.23-alpine` does not.
- Alpine has no `bash`; Harbor needs it (`apk add bash`).
- The build context is `environment/` only, so stage 1 clones from GitHub.
- The fonts are CRLF files; `splitlines()` handles that, a port must too.

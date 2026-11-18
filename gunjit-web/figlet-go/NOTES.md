# NOTES — pyfiglet, read before porting

## Source

- URL: https://github.com/pwaller/pyfiglet
- Commit: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: 2026-08-02T22:51:18+01:00
- Version in `pyfiglet/version.py`: 1.0.4
- Licence: **MIT** (LICENSE file; the same text is in `pyfiglet/__init__.py` as `__copyright__`)

## Why `python -m pyfiglet hello` fails straight after cloning

`FigletFont.preloadFont` looks fonts up with
`importlib.resources.files('pyfiglet.fonts')` — the package directory
`pyfiglet/fonts/`. That folder does not exist in git; the fonts are shipped in
`pyfiglet/fonts-standard/` (72 files, MIT-compatible) and
`pyfiglet/fonts-contrib/` (many authors). Without the copy you get
`FontNotFound: standard`.

The `Makefile` target `minimal` fixes it (`clean` recreates the empty folder,
then `cp pyfiglet/fonts-standard/* pyfiglet/fonts`). `setup.py` does the same
`copytree` when the folder is missing. The Dockerfile's stage 1 repeats the step:

```
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

Only `fonts-standard/` is used in this task. `fonts-contrib/` is deleted.

## How the engine works (FigletBuilder)

Text is a list of code points. For each character:

1. Unknown characters (not in the font) are skipped silently.
2. If the glyph is wider than `-w`, pyfiglet raises `CharNotPrinted` (traceback, exit 1, no stdout).
3. `smushAmount` computes how many columns the new glyph may overlap the line buffer (`maxSmush`).
4. If `len(buffer) + glyph width - maxSmush >= width`, the line is cut: at the last
   space if there was one (the saved buffer from *before* the space is emitted, and
   the space is dropped), otherwise before this character.
5. Otherwise the glyph is merged row by row (`smushRow`).

After the text: each finished line is justified, then the hardblank (`$` in all
standard fonts) becomes a space, rows are joined with `\n`, and `main()` adds one
more `\n`.

## The smushing rules (horizontal only; pyfiglet ignores vertical layout bits)

The font header's full-layout number selects the mode:

| bit | name | meaning |
|---|---|---|
| 64 | KERN | glyphs slide together until they touch; no overlap of visible chars |
| 128 | SMUSH | overlap by one more column, resolved by the rules below |
| none of 1..32 with 128 | universal smushing | the later character in the text wins; a hardblank loses to a visible char |

Rules (only when SMUSH is set and at least one rule bit is set). "Later" means the
character the user typed later, which is the right one in left-to-right mode and the
left one in right-to-left mode. Two edge characters `a` (already on the line) and `b` (new glyph):

| # | bit | rule | example |
|---|---|---|---|
| 1 | 1 EQUAL | identical non-hardblank chars → that char | `|` + `|` → `|` (standard `||` prints three bars, not four) |
| 2 | 2 LOWLINE | `_` next to any of `|/\[]{}()<>` → the other char | `_` + `|` → `|` (standard `_|`: the bar eats the underscore's end) |
| 3 | 4 HIERARCHY | classes `|` < `/\` < `[]` < `{}` < `()` < `<>`; different classes → the higher one | `|` + `/` → `/`, `[` + `(` → `(` |
| 4 | 8 PAIR | `[`+`]`, `{`+`}`, `(`+`)` (either order) → `|` | standard `[]` prints `| _|_ |`: the brackets' edges merged into one bar |
| 5 | 16 BIGX | `/`+`\` → `|`, `\`+`/` → `Y`, `>`+`<` → `X` | note: pyfiglet returns `|` for `/\`, where the FIGlet spec says `X`; we copy pyfiglet |
| 6 | 32 HARDBLANK | hardblank + hardblank → hardblank | `$`+`$` → `$`; any other pair with a hardblank never smushes |

Before any rule: a space always gives way to the other char, and **no smushing at
all if the previous or the current glyph is narrower than 2 columns**
(`prevCharWidth < 2 or curCharWidth < 2`).

Kerning example (banner, layout 64): `ab` → the `#` columns touch but never overlap.
Universal example (mini, layout 1920): `ab` → `(_||_)`: the `|` of `b` wrote over the `|` of `a`.

Which fonts use what (header field 8):

| font | layout | horizontal behaviour |
|---|---|---|
| standard, slant, small, big | 24463 / 18319 / 22415 / 24463 | SMUSH + rules 1,2,3,4 |
| mini, shadow | 1920 / 4992 | universal smushing |
| banner, script | 64 / 3904 | kerning only |

So rules 5 and 6 are implemented but never fire with these eight fonts.

## Width, justify, direction

- `smushAmount` is also what trims the first glyph of a line: on an empty buffer it
  equals the glyph's smallest number of leading spaces, so those columns are cut off.
- `justify right`: pad each row with `width - len(row) - 1` spaces (one less than you
  would expect). `center`: `int((width - len(row)) / 2)` spaces.
- Direction `auto` = the font's print direction (field 7; all eight fonts say 0 =
  left-to-right). Justify `auto` = left for left-to-right, right for right-to-left.
- Right-to-left: the new glyph is put on the **left** and smushing compares the
  glyph's right edge with the buffer's left edge.

## Options covered

| option | effect |
|---|---|
| `-f FONT` | font name (default `standard`) |
| `-w COLS` | terminal width for wrapping **and** justification (default 80) |
| `-j auto|left|center|right` | justification, see above |
| `-D auto|left-to-right|right-to-left` | print direction |
| `-r` | mirror: each row reversed and `/\()[]{}<>` swapped |
| `-F` | flip upside down: rows reversed and `/\ ^v _- AV MW bPR mw` swapped |
| `-s` | drop leading blank rows, then strip all trailing whitespace (no final blank rows, no trailing spaces on the last row) |
| `-n` | one empty line before and after (`-s` wins if both given) |

Not covered: `-c`, `-L`, `-l`, `-i`.

## Rare cases (go into the test generator)

- `||`, `//`, `_|`, `[]`, `{}`, `()`, `<>`, `><`, `/\`, `\/`, `|_|`, `[{(<`, `>)}]`, `_/_`, `|/|`
- `|_|` with `-w 10 -D right-to-left` (wraps every character; justify auto = right)
- the first glyph of every line loses its leading blank columns
- double spaces `a  b`, leading/trailing spaces, an empty text `""`, text of only spaces
- `-w 10` with wide glyphs → the original raises `CharNotPrinted`; no answer exists, skip
- a glyph exactly as wide as `-w` with no leading blank column (slant `H`, `E`, `F`, `N`, `w`, `5`, `@` at `-w 10`): **the original loops forever** (cutBufferAtLastChar re-reads the same char on an empty buffer). No answer exists; the generator skips these.
- Latin-1 letters `é ß ñ ÄÖÜäöü` (present in standard/big, absent in slant/small/mini/script/shadow)
- characters the font lacks (`€`, `→`, CJK, tab) are dropped without a trace
- `-j right` and `-D right-to-left` together; `-j center` with narrow `-w`
- `-s` after `-F` (blank rows are now at the other end), `-r` with `-D right-to-left`
- fonts have CRLF line endings; the header line ends in `\r`

## Environment traps hit while building the room

- `golang:1.23-bookworm` ships `/usr/bin/python3`; use `golang:1.23-alpine`.
- Alpine has no `bash`; Harbor needs it. `apk add bash`.
- Harbor 0.25 on local Docker refuses `network_mode = "no-network"`; use `public`.

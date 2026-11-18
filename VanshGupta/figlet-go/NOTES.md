# pyfiglet source notes

- Source: https://github.com/pwaller/pyfiglet
- Pinned commit: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: `2026-08-02T22:51:18+01:00`
- License: MIT
- The source checkout needs its standard fonts copied into the runtime font directory:
  `mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/`

The renderer computes the maximum safe overlap between adjacent glyphs row by
row. The following six FIGlet smushing rules decide when two occupied edge
characters may share a column; `$` below denotes a font's hardblank:

1. **Equal character:** matching characters collapse, for example `||` becomes
   `|`.
2. **Underscore:** `_` yields to one of `| / \ [ ] { } ( ) < >`; for example
   `_/` becomes `/`.
3. **Hierarchy:** the higher-ranked edge character wins in the FIGlet ordering
   `|`, `/\`, `[]`, `{}`, `()`, `<>`; for example `|/` becomes `/`.
4. **Pair:** the matched pairs `[]`, `{}`, and `()` become `|`; for example
   `[]` becomes `|`.
5. **Big X:** `/` with `\` becomes `|`, `\` with `/` becomes `Y`, and `>` with
   `<` becomes `X`; for example `/\` becomes `|`.
6. **Hardblank:** two hardblanks combine into one, for example `$$` becomes `$`.

Kerning is overlap without combining occupied characters: in the `standard`
font, `A` and `V` render 9 columns each, while `AV` renders 14 columns. Their
otherwise-empty edge columns overlap; no printed character is replaced.
Universal smushing uses the same overlap while choosing which visible character
wins.

The command-line options in scope are:

- `-f FONT`: select the FIGfont (default `standard`).
- `-w COLS`: set the wrapping and justification width (default 80 columns).
- `-j auto|left|center|right`: select line justification; `auto` follows the
  resolved print direction.
- `-D auto|left-to-right|right-to-left`: select print direction; `auto` uses
  the font's direction metadata.
- `-r`: mirror the rendered text horizontally.
- `-F`: flip the rendered text vertically.
- `-s`: strip surrounding blank lines from the rendered art.
- `-n`: normalize the surrounding blank lines to one above and below the art.
  If both `-s` and `-n` are given, `-s` takes precedence.

Rare cases for the test corpus include text with each smushing-rule character,
hardblanks, repeated spaces, newlines, very narrow wrapping widths, long words,
right-to-left output, missing glyphs, and non-ASCII text such as `é`, `ß`, and
`ñ`.

The grader's weighted mean is `(build + no_python + 4 * behaviour) / 6`:

- Nothing implemented: `0 / 6 = 0.00`.
- Build only: `1 / 6 = 0.1667`, approximately `0.17`.
- Perfect build, no-Python check, and behavior: `6 / 6 = 1.00`.

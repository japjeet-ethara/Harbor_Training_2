# pyfiglet source notes

- Repository: https://github.com/pwaller/pyfiglet
- Pinned commit: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: `2026-08-02T22:51:18+01:00`
- Licence: MIT

## Setup required after cloning

The package loads font files from `pyfiglet/fonts`, but a fresh checkout only
contains the standard fonts under `pyfiglet/fonts-standard`. The `Makefile`
`minimal` target runs `clean`, copies `pyfiglet/fonts-standard/*` into
`pyfiglet/fonts`, then builds the package. For direct use from the checkout,
create `pyfiglet/fonts` and copy those standard font files there before running
`python -m pyfiglet`.

## Rendering and overlap rules

The font header's layout bits select which overlap rules apply. Kerning permits
adjacent glyphs to overlap where their edge cells are blank; it does not merge
two visible characters. The six FIGlet smushing rules merge a pair of visible
edge characters:

1. **Equal character:** identical non-hardblank characters collapse to one:
   `|` + `|` -> `|`.
2. **Underscore (lowline):** `_` is replaced by a character from the FIGlet
   hierarchy: `_` + `/` -> `/`.
3. **Hierarchy:** the character from the later class in the FIGlet order wins.
   The order is `|`, `/\`, `[]`, `{}`, `()`, `<>`; for example,
   `/` + `[` -> `[`.
4. **Opposite pairs:** `[]`, `{}`, or `()` collapse to `|`; for example,
   `[` + `]` -> `|`.
5. **Big X:** `>` + `<` -> `X`; the slash orientations are `/` + `\` -> `|`
   and `\` + `/` -> `Y`.
6. **Hardblank:** two hardblanks collapse to one hardblank. In the standard
   font, `$` is the hardblank: `$` + `$` -> `$`.

Kerning and smushing are separate layout modes: a font may kern without
smushing, or apply selected smushing rules. For example, kerning-only layout
can place the glyphs for `A` + `B` over a shared blank edge column, producing
`AB` without merging either visible character.

## Command-line options in scope

- `-f FONT`: select the FIGlet font; defaults to `standard`.
- `-w COLS`: set the wrapping and justification width; defaults to `80`.
- `-j auto|left|center|right`: select line justification. `auto` follows the
  resolved print direction.
- `-D auto|left-to-right|right-to-left`: select print direction. `auto` follows
  the font's print-direction metadata (and defaults to left-to-right if absent).
- `-r`: mirror the rendered output horizontally.
- `-F`: flip the rendered output vertically.
- `-s`: strip surrounding blank lines from the rendered output.
- `-n`: normalize the output to one blank line before and after the rendering.
  When both `-s` and `-n` are given, `-s` takes precedence.

The options `-c`, `-L`, `-l`, and `-i` are outside this task's scope.

## Rare-case input list

Use these in addition to ordinary words when checking the port:

- Overlap punctuation: `| / \ _ [ ] { } ( ) < >`, `|_|`, `/_/`, `\_\`,
  `/\`, `\/`, `><`, `[]`, `}{`, `()`.
- Spacing: `hello  world`, leading and trailing spaces, and multiple runs of
  spaces.
- Wrapping and line breaks: long sentences, embedded newlines, consecutive
  newlines, and leading or trailing newlines.
- Latin-1 glyphs: `é`, `ß`, `ñ`, `Ä`, `ö`.
- Unsupported glyphs: `🚀`, `中文`, `★`, `→`, and `✓`.
- Option edge cases: right-to-left with narrow widths; all justification modes;
  and combinations of `-r`, `-F`, `-s`, and `-n`.

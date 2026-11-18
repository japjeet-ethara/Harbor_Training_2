# NOTES — pyfiglet (Step 1)

- **URL:** https://github.com/pwaller/pyfiglet
- **Pinned commit:** `255827efbc09d662875dad50fa0edc1e1263bf45`
- **Commit date:** 2026-08-02T22:51:18+01:00
- **Licence:** MIT (The MIT License, Copyright 2007-2018 Christopher Jones, Stefano Rivera, Peter Waller and contributors)
- **Version string:** 1.0.4 (`pyfiglet/version.py`)

## Why `python -m pyfiglet hello` fails straight after cloning

Fonts live in `pyfiglet/fonts-standard/`, but the code loads them with
`importlib.resources.files('pyfiglet.fonts')`, i.e. from `pyfiglet/fonts/`, which does not exist
in git. Result: `ModuleNotFoundError: No module named 'pyfiglet.fonts'`.

The `Makefile` target `minimal` (via `clean`) fixes it:

    rm -rf pyfiglet/fonts/ && mkdir pyfiglet/fonts/
    cp pyfiglet/fonts-standard/* pyfiglet/fonts

(`full` also copies `fonts-contrib/`; this task does NOT use it.)
Note `fonts-standard/` also contains an `__init__.py`, which makes `fonts/` a package.

## Rendering pipeline (FigletBuilder)

1. Font parsing: header `flf2a<hardblank> height baseline maxlen oldLayout commentLines [printDirection] [fullLayout]`.
   If `fullLayout` is missing: oldLayout==0 -> 64 (kern); <0 -> 0 (full width); else (old&31)|128.
2. For each input char (by code point), find the glyph. Unknown chars are SKIPPED silently
   (except `\n`, which forces a line break). Space is always defined.
3. `smushAmount` = how many columns the new glyph may overlap the buffer (per row, min over rows).
4. Add the glyph row by row; if `len(buffer[0]) + width - smush >= width_option` -> wrap.
5. Wrapping: cut at the last blank (space) seen; if none, cut at the last char (iterator-1).
6. Justify each output block, replace hardblank with space.

## Smushing: kerning + six rules (`smushChars`, bit values in `font.smushMode`)

Layout bits: 1 equal, 2 lowline, 4 hierarchy, 8 pair, 16 bigx, 32 hardblank, 64 kern, 128 smush.

Order inside `smushChars(left, right)`:
- `left == ' '` -> right; `right == ' '` -> left  (always)
- if prevCharWidth < 2 or curCharWidth < 2 -> None (no overlap)
- if mode&128 == 0 -> None (**kerning only**: glyphs move together until they touch, never overlap)
- if mode&63 == 0 -> **universal smush**: hardblank loses; otherwise the later char in the text wins
  (right char for LTR, left char for RTL)

| bit | rule | tiny example (left,right -> result) — verified by running the original |
|---|---|---|
| 32 | hardblank | `$`,`$` -> `$` (checked BEFORE the "hardblank never smushes with anything else" rule); `$`,`a` -> none |
| 1 | equal | `|`,`|` -> `|`; `#`,`#` -> `#`; `|`,`/` -> none (hardblanks excluded) |
| 2 | lowline | `_`,`|` -> `|`; `/`,`_` -> `/`; `_`,`_` -> none; `_`,`x` -> none (only with `|/\[]{}()<>`) |
| 4 | hierarchy | classes `|` < `/\` < `[]` < `{}` < `()` < `<>`; higher class wins: `|`,`/` -> `/`; `[`,`{` -> `{`; `(`,`<` -> `<` |
| 8 | pair | `[`,`]` -> `|`; `]`,`[` -> `|`; `{`,`}` -> `|`; `(`,`)` -> `|` |
| 16 | bigx | `/`,`\` -> `|`; `\`,`/` -> `Y`; `>`,`<` -> `X`; `<`,`>` -> none |

**Correction to the task page:** it says "`/` and `\` meet as an X". In this code `/`+`\` gives `|`,
`\`+`/` gives `Y`, and only `>`+`<` gives `X`. (Rare case #1.)

**Reachability finding (checked by running the original):** none of the 67 fonts in `fonts-standard/` has the
`bigx` (16) or hardblank-smush (32) bit set together with smush (128). Those two rules can never fire with the
fonts this task uses. The universal "later char wins" rule never fires either: in `mini` and `shadow` the only
time two visible characters meet is when one is a hardblank, and the hardblank always loses. What really decides
the output: kerning (`banner`, `script`), rules 1/2/4/8 (`standard`, `slant`, `small`, `big`), hardblank-loses
(`mini`, `shadow`), plus wrapping, justify, direction, `-r -F -s -n`.

Fonts used by this task and their layout value:
standard 24463, slant 18319, small 22415, big 24463, mini 1920, banner 64 (kern only), shadow 4992, script 3904. All have printDirection 0.

## Options (`-f -w -j -D -r -F -s -n`)

- `-f FONT` font name (default `standard`); unknown font prints an error (not graded).
- `-w COLS` width for wrapping/justify (default 80). A glyph wider than COLS raises an error (not graded).
- `-j auto|left|center|right` justify. `auto` = left for LTR, right for RTL. `right` pads `width - len(row) - 1` spaces; `center` pads `int((width-len)/2)` (truncating).
- `-D auto|left-to-right|right-to-left` direction. `auto` uses the font header (all graded fonts: LTR). RTL swaps the smushing sides, so the later char wins universal smush, and the buffer is built right-to-left.
- `-r` mirror: reverse every row, mapping `/`<->`\`, `(`<->`)`, `[`<->`]`, `{`<->`}`, `<`<->`>`.
- `-F` flip upside-down: reverse row ORDER and map (only these change): `/`<->`\`, `A`<->`V`, `M`<->`W`, `m`<->`w`, `b`<->`P`, `R`->`b`, `^`<->`v`, `_`->`-`. Note `b`->`P` and `R`->`b` are not symmetric.
- `-s` strip empty leading/trailing lines (keeps empty rows inside the glyph block); wins over `-n`.
- `-n` normalise: one empty line before and after.
- Output is always `render + "\n"` (a final extra newline from `main()`).
- Not graded: `-c`, `-L`, `-l`, `-i`.

## Rare cases list (traps for the twin, used by gen.py)

1. `/` `\` `>` `<` next to each other (bigx: `|`, `Y`, `X` — not what the task page says).
2. `|_|`, `_/`, `\_` with lowline + hierarchy interaction.
3. Narrow width (`-w 10`): wrap with no blank, and "width not enough" characters.
4. Right-to-left combined with `-j right` / `-j center` / `-j left`.
5. Double spaces and leading/trailing spaces (blank markers, wrap at the last blank).
6. Characters the font lacks (é, ß, ñ, CJK): skipped silently; ß/ä/ö/ü/Ä/Ö/Ü exist in some fonts.
7. `-s` vs `-n` together (`-s` wins), `-F` with `-r`.
8. Embedded `\n` in the text forces a flush.
9. Python `str.splitlines()` quirks (`\x0b`, `\x0c`, `\x1c`-`\x1e`, `\x85`, `\u2028`, `\u2029` are line breaks) used by reverse/flip/strip.
10. `str.rstrip()` / `row.strip()` strip Unicode whitespace, not only ASCII space.

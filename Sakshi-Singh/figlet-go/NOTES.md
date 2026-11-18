# NOTES — pyfiglet

| | |
|---|---|
| URL | https://github.com/pwaller/pyfiglet |
| Commit | `255827efbc09d662875dad50fa0edc1e1263bf45` |
| Commit date | `2026-08-02T22:51:18+01:00` |
| Licence | **MIT** (`LICENSE`, also repeated in `pyfiglet/__init__.py`) |

## Why `python -m pyfiglet hello` fails straight after cloning

`FigletFont.preloadFont` loads fonts with
`importlib.resources.files('pyfiglet.fonts')`, i.e. from the folder
`pyfiglet/fonts/`. The repository does not contain that folder: the fonts ship in
`pyfiglet/fonts-standard/` (MIT-compatible fonts) and `pyfiglet/fonts-contrib/`
(fonts from many authors). The result is `FontNotFound: standard`.

The `Makefile` target `minimal` (via `clean`) fixes it:

```
rm -rf pyfiglet/fonts/ && mkdir pyfiglet/fonts/
cp pyfiglet/fonts-standard/* pyfiglet/fonts
```

Only `fonts-standard/` is used in this task (72 files, includes its `__init__.py`).

## Font file (.flf) facts that matter

* Header `flf2a$ 6 5 16 15 41 0 24463`: hardblank `$`, height, baseline, max length,
  old layout, comment lines, print direction (optional), full layout (optional).
* If full layout is missing: old layout `0` → `64` (kern), `<0` → `0`, else `(old & 31) | 128`.
* Each character is `height` lines; the end marker is the last non-blank character of
  its first line; 1 or 2 end markers (plus trailing whitespace) are cut from each line.
* Characters 32..126, then the 7 German letters `ÄÖÜäöüß`, then code-tagged
  characters (`0x00E9 ...`). Only hex tags (`0x...`) are read.
* Character width = longest line after the end markers are cut.
* Missing characters are **skipped silently** (no output, no width).

## Smushing (FigletBuilder.smushChars) — checked by running the real code

Layout bits: 1 EQUAL, 2 LOWLINE, 4 HIERARCHY, 8 PAIR, 16 BIGX, 32 HARDBLANK,
64 KERN, 128 SMUSH.

| # | Rule | Example (left + right → result) |
|---|---|---|
| 1 | Equal characters (not hardblank) | `\|` + `\|` → `\|` |
| 2 | Underscore: `_` is replaced by any of `\|/\[]{}()<>` | `_` + `/` → `/` |
| 3 | Hierarchy `\|` < `/\` < `[]` < `{}` < `()` < `<>`: the higher class wins | `\|` + `/` → `/`, `/` + `[` → `[` |
| 4 | Opposite pair | `[` + `]` → `\|`, `(` + `)` → `\|` |
| 5 | Big X | `/` + `\` → `\|`, `\` + `/` → `Y`, `>` + `<` → `X` |
| 6 | Hardblank | `$` + `$` → `$` |
| — | Kerning (64 without 128) | letters touch but never overlap |
| — | Universal smushing (128 with rules 1-6 all off) | the later character wins; hardblank loses |

Extra conditions: a space always gives way to the other character; no smushing at all
if the previous or the current character is narrower than 2 columns.

**Note:** the task page says "`/` and `\` meet as an X". That is wrong: `/\` → `|`,
`\/` → `Y`; only `><` → `X`.

### Which rules the 8 task fonts actually use

| font | layout | rules |
|---|---|---|
| standard, big | 24463 | smush 1,2,3,4 |
| slant | 18319 | smush 1,2,3,4 |
| small | 22415 | smush 1,2,3,4 |
| mini, shadow | 1920 / 4992 | universal smushing |
| banner, script | 64 / 3904 | kerning only |

None of the 8 fonts enables rule 5 (Big X) or rule 6 (hardblank), so the tests cannot
check those two rules.

## Options

| option | meaning |
|---|---|
| `-f FONT` | font name (default `standard`) |
| `-w COLS` | width used for wrapping and justification (default 80) |
| `-j auto\|left\|center\|right` | justification; `auto` = left for left-to-right, right for right-to-left. `right` pads to `COLS-1`, `center` pads `(COLS-len)//2`, both use `-w` |
| `-D auto\|left-to-right\|right-to-left` | print direction; `auto` = the font's header (all 8 fonts: left-to-right) |
| `-r` | mirror: each row reversed, and `/`↔`\`, `(`↔`)`, `<`↔`>`, `[`↔`]`, `{`↔`}` swapped |
| `-F` | flip: rows in reverse order, and `/`↔`\`, `^`↔`v`, `_`→`-`, `A`↔`V`, `M`↔`W` ... |
| `-s` | strip leading empty lines and all trailing whitespace |
| `-n` | like `-s`, then one `\n` before and after (ignored if `-s` is also given) |

Order in `main()`: render → `-r` → `-F` → `-s` or else `-n` → write + one extra `\n`.
Text = all non-option arguments joined by one space. Out of scope: `-c`, `-L`, `-l`, `-i`.

## Rare cases (things that are easy to get wrong)

1. First character: its common leading blank columns are removed (smush against an empty buffer).
2. Wrapping happens at the **last space**; the space itself is dropped; a word wider than
   the line is cut at the character.
3. A missing character keeps the previous character's width for the "<2 columns" rule.
4. `-D right-to-left` swaps left/right in smushing **and** defaults justification to right.
5. `-j right` pads to `width - len - 1`, not `width - len`.
6. Python's `'' in s` is always true: an empty left edge smushes with anything in the hierarchy.
7. A smush result of `''` removes a column (row gets shorter).
8. `\n` in the text forces a new line.
9. Fonts and output use Python `splitlines()` (also splits on `\v \f \x1c \x1d \x1e`).
10. `-w` smaller than a character: pyfiglet crashes (`CharNotPrinted`); a character exactly
    as wide as `-w` with nothing to smush loops forever. These cases are not tested.
11. Latin-1: `Ä` and `ß` exist in all 8 task fonts; `é ñ` only in the fonts with
    code-tagged characters (standard, big, banner); elsewhere they are skipped.

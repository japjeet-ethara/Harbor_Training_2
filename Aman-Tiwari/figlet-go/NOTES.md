# NOTES — pyfiglet (source of truth for figlet-go)

| | |
|---|---|
| Repository | https://github.com/pwaller/pyfiglet |
| Commit SHA | `255827efbc09d662875dad50fa0edc1e1263bf45` |
| Commit date | `2026-08-02T22:51:18+01:00` |
| Licence | **MIT** (`LICENSE`: "The MIT License (MIT) Copyright © 2007-2018") |

## 1. Why `python -m pyfiglet hello` fails right after cloning

`FigletFont.preloadFont` loads fonts with `importlib.resources.files('pyfiglet.fonts')`,
but a fresh clone has no `pyfiglet/fonts/` folder: the fonts live in `pyfiglet/fonts-standard/`
(and `pyfiglet/fonts-contrib/`). The `Makefile` fixes it:

```
clean:    rm -rf pyfiglet/fonts/ && mkdir pyfiglet/fonts/
minimal:  clean + cp pyfiglet/fonts-standard/* pyfiglet/fonts
```

Font setup we use (standard fonts only, never `fonts-contrib/`):

```
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
python -m pyfiglet -f slant "hello"      # verified: prints the slanted "hello"
```

## 2. Font facts (the 8 fonts used by the tests)

`flf2a` header: `hardblank height baseline maxlen oldLayout commentLines [printDirection [fullLayout]]`.
`smushMode` = fullLayout (or derived from oldLayout). Bits: 1 EQUAL, 2 LOWLINE, 4 HIERARCHY,
8 PAIR, 16 BIGX, 32 HARDBLANK, 64 KERN, 128 SMUSH.

| font | smushMode | rules active | height |
|---|---|---|---|
| standard | 24463 | smush + 1,2,4,8 + 64 + kern | 6 |
| slant | 18319 | smush + all six | 6 |
| small | 22415 | smush + all six | 5 |
| big | 24463 | smush + 1,2,4,8 | 8 |
| mini | 1920 | **universal smushing** (no rule bits) | 4 |
| banner | 64 | **kerning only** | 8 |
| shadow | 4992 | **universal smushing** | 5 |
| script | 3904 | **universal smushing** | 7 |

All 8 have printDirection 0 (left-to-right) and hardblank `$`. All are plain UTF-8 (not zipped).

## 3. Smushing rules (`FigletBuilder.smushChars`) — each verified by calling the real code

Applied in this order, only when `smushMode & 128` (SMUSH) is set:

0. **Spaces always give way:** `' '` + X → X, X + `' '` → X.
   **Narrow chars never overlap:** if previous or current FIGcharacter width < 2 → no smush.
1. **Equal character (1):** `|` + `|` → `|` (never for hardblanks).
2. **Underscore (2):** `_` + `/` → `/`, `/` + `_` → `/` (`_` is replaced by any of `|/\[]{}()<>`).
3. **Hierarchy (4):** classes `|` < `/\` < `[]` < `{}` < `()` < `<>`; the higher class wins:
   `|` + `/` → `/`, `}` + `(` → `(`.
4. **Opposite pair (8):** `[`+`]` → `|`, `)`+`(` → `|` (also `{}` / `()`, either order).
5. **Big X (16):** `/` + `\` → `|`, `\` + `/` → `Y`, `>` + `<` → `X`; `<` + `>` → no smush.
6. **Hardblank (32):** `$` + `$` → `$`.

**Universal smushing** (SMUSH set, no rule bits 1–32): hardblank loses to the other char;
otherwise the *later* char in the text wins (`a` + `b` → `b`; with right-to-left, the left one).

**Kerning** (KERN only, e.g. banner): letters slide together until they touch; never merge
(`smushChars` returns nothing).

## 4. Options covered by the task

| opt | meaning |
|---|---|
| `-f FONT` | font name (default `standard`); looks for `FONT.tlf` then `FONT.flf` |
| `-w COLS` | width for wrapping/justification (default 80) |
| `-j auto\|left\|center\|right` | justify; `auto` = left for LTR, right for RTL |
| `-D auto\|left-to-right\|right-to-left` | direction; `auto` = font's printDirection |
| `-r` | reverse: mirror each row (swaps `/\` `()` `[]` `{}` `<>`) |
| `-F` | flip upside down (rows reversed; `/`↔`\`, `_`→`-`, `^`↔`v`, `A`↔`V`, `M`↔`W`, …) |
| `-s` | strip surrounding empty lines (and trailing whitespace of the result) |
| `-n` | normalize: exactly one empty line before and after |

Out of scope: `-c` colour, `-L` font install, `-l` font list, `-i` font info.
Positional args are joined with one space. Output is always followed by one extra `\n`.

## 5. Rare cases (where the guide or a naive port goes wrong)

1. The practice page says "`/` and `\` meet as an `X`" — **wrong**: `/\` → `|`, `\/` → `Y`, only `><` → `X`.
2. mini, shadow and script use universal smushing, not the six rules.
3. With `-D right-to-left` the buffer is glued on the *right* of each new char, and in universal
   smushing the *left* char wins.
4. `-j right` pads with `width - len - 1` spaces (one less than you would expect);
   `-j center` pads with `int((width - len) / 2)`. `-j auto` + RTL ⇒ right.
5. Wrapping cuts at the last space seen on the line; if there is none, at the last char.
   The char that did not fit is processed again on the next line.
6. `prevCharWidth` is **not** reset to 0 after a line break (it is overwritten right after),
   so the "width < 2" rule sees the char that caused the break.
7. A char wider than `-w` raises `CharNotPrinted`: nothing on stdout.
   A char exactly as wide as `-w` with no leading blank loops forever.
8. Characters missing from the font are skipped silently (most fonts lack `é ñ`; standard has `ÄÖÜäöüß` + more).
9. The last line is only output if its first row is non-empty.
10. `-s` wins over `-n`; `-s` also strips trailing spaces from the last row.
11. Right-to-left can crash (IndexError → empty stdout) when the overlap is wider than the
    other side's row.

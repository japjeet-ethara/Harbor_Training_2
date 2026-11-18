# NOTES — pyfiglet, as pinned for this task

| | |
|---|---|
| Repository | https://github.com/pwaller/pyfiglet |
| Pinned commit (SHA) | `255827efbc09d662875dad50fa0edc1e1263bf45` |
| Commit date | `2026-08-02T22:51:18+01:00` |
| Licence | **MIT** (`LICENSE`: "The MIT License (MIT)", Copyright © 2007-2018) |
| Fonts used by the task | only `pyfiglet/fonts-standard/` (`fonts-contrib/` is deleted in the image) |

## 1. The font setup step (why `python -m pyfiglet hello` fails after cloning)

The code loads fonts with `importlib.resources.files('pyfiglet.fonts')`, i.e. from the package folder
`pyfiglet/fonts/`. A fresh clone only has `pyfiglet/fonts-standard/` and `pyfiglet/fonts-contrib/`, so Python
stops with `ModuleNotFoundError: No module named 'pyfiglet.fonts'`.

The `Makefile` fixes this in its `minimal` target (`full` also copies the contrib fonts):

```
rm -rf pyfiglet/fonts/ ; mkdir pyfiglet/fonts/          # target "clean"
cp pyfiglet/fonts-standard/* pyfiglet/fonts             # target "minimal"
```

After that (72 files, 67 of them `.flf` fonts) `python -m pyfiglet -f slant hello` prints the slanted letters.
The Dockerfile's stage 1 does exactly this copy.

## 2. How a font file is read (`FigletFont.loadFont`)

* Header: `flf2a<hardblank> height baseline maxlen oldLayout commentLines [printDirection [fullLayout]]`.
* `fullLayout` missing → derived from `oldLayout`: `0 → 64` (kerning), `<0 → 0` (full width), else `(old & 31) | 128`.
* Characters 32..126 come first, then `ÄÖÜäöüß` (if there are lines left), then code-tagged characters whose
  tag line starts with `0x`. Tags written in decimal are skipped.
* Each glyph row ends with an end-mark (1 or 2 copies, trailing whitespace allowed); it is stripped.
* Three fonts (`bubble`, `digital`, `term`) are not valid UTF-8; Python decodes with `replace`.
* Header values of the 8 fonts used in the exam: standard 24463, slant 18319, small 22415, big 24463,
  mini 1920, banner 64 (kerning only), shadow 4992, script 3904. All print left-to-right (direction 0).

## 3. Kerning and the six smushing rules (`FigletBuilder.smushChars`, `smushAmount`)

Mode bits: 1 equal · 2 lowline · 4 hierarchy · 8 pair · 16 bigx · 32 hardblank · **64 kerning** · **128 smushing**.
Every example below was run against `smushChars` of the pinned code (both glyphs at least 2 wide).

| Bit | Rule | Examples (left + right → result) |
|---|---|---|
| 1 | **equal**: two identical characters (not hardblanks) become one | `\|`+`\|` → `\|`; `\|`+`/` → none |
| 2 | **lowline**: `_` is eaten by any of `\| / \ [ ] { } ( ) < >` | `_`+`\|` → `\|`; `/`+`_` → `/`; `_`+`_` → none (by this rule) |
| 4 | **hierarchy**: classes `\|` < `/ \` < `[ ]` < `{ }` < `( )` < `< >`; the higher class wins | `\|`+`/` → `/`; `/`+`[` → `[`; `[`+`{` → `{`; `{`+`(` → `(`; `(`+`<` → `<`; `<`+`\|` → `<` |
| 8 | **pair**: opposite brackets make a bar | `[`+`]` → `\|`; `)`+`(` → `\|`; `{`+`}` → `\|`; `[`+`}` → none |
| 16 | **bigx** (as coded here, NOT the textbook figlet table) | `/`+`\` → `\|`; `\`+`/` → `Y`; `>`+`<` → `X`; `<`+`>` → none |
| 32 | **hardblank**: two hardblanks become one hardblank | `$`+`$` → `$` (with rule 1 alone: none) |
| 64 | **kerning**: glyphs are pushed together until they touch, nothing overlaps | (`banner` uses only this) |

Also:
* A space on either side always loses: space + x → x.
* If a glyph (the previous or the current one) is only 0 or 1 column wide, nothing smushes except spaces.
* Smushing needs bit 128. With bits 1..32 all off but 128 set (**universal smushing**) the later character wins
  (`a`+`b` → `b`; with right-to-left → `a`); a hardblank loses to a visible character.
* With neither bit 64 nor 128 (mode 0, full width) glyphs are simply placed side by side.
* `smushAmount` looks at every row: it counts the free space between the two glyphs, plus one extra column when
  the two touching characters can be smushed (or the left one is a space). The smallest value over all rows wins.

## 4. The eight options

| Option | What it does |
|---|---|
| `-f FONT` | font name; file `FONT.flf` (default `standard`). Unknown font → message, exit 1 |
| `-w COLS` | width for wrapping and justifying (default 80). A glyph wider than `COLS` → error. A glyph exactly `COLS` wide makes the original loop forever |
| `-j auto\|left\|center\|right` | `auto` = `left` for left-to-right, `right` for right-to-left. `center` adds `int((COLS - rowlen)/2)` spaces; `right` adds `COLS - rowlen - 1` (one less!) |
| `-D auto\|left-to-right\|right-to-left` | `auto` = the font header's print direction (0 → ltr, 1 → rtl, absent → ltr). In rtl each new glyph is put on the **left** and the roles of the two glyphs swap when smushing |
| `-r` | mirror: each row is reversed and the pairs `( )`, `/ \`, `< >`, `[ ]`, `{ }` are swapped |
| `-F` | flip: rows in reverse order and `A↔V`, `M↔W`, `m↔w`, `P→b`, `R→b`, `b→P`, `^↔v`, `/↔\`, `_→-` |
| `-s` | remove empty lines at start and end **and** trailing whitespace of the whole text (so trailing spaces on the last row go too) |
| `-n` | exactly one empty line before and after; ignored when `-s` is given (`-s` wins) |

Order of work: render → `-r` → `-F` → `-s` (or else `-n`) → write UTF-8 + one `\n`. Words are joined with single spaces.
Wrapping: when the next glyph would reach the width, the line is cut at the last space (the space is dropped), or
at the last glyph if there was no space.

## 5. Rare cases (every one is in `tests/gen.py`)

* `-D right-to-left` with narrow `-w`, and `-j right` / `-j center` on top of it (padding goes on the left).
* `-j right` uses `width - len - 1`; `-j center` truncates toward zero.
* bigx is not symmetric: `/\` gives `|`, `\/` gives `Y`, `><` gives `X`. The task page says "`/` and `\` meet as an `X`": that is **wrong** for this code.
* `|_|`, `_|_`, `__`, `[]`, `][`, `{}`, `()`, `)(`, `<>`, `><`, `[{()}]` next to each other.
* Glyphs that are 1 column wide (`i`, `l`, `!`, `.`, `'`) do not smush.
* Characters the font does not have are skipped silently: `é`, `ñ`, `日` are missing from all 8 exam fonts; `ß` and `Ä Ö Ü ä ö ü` exist in all 8 (and in 50 of the 67 fonts).
* Double spaces, leading and trailing spaces, a lone space, and `\n` inside the text.
* `-s` trims trailing spaces of the last row; `-n` after `-r`/`-F`.
* Flip map oddities (`R→b`, `P→b`, `_→-`, `-` stays `-`) and `-r` / `-F` together.
* Inputs where the original crashes or never ends (width too small, `-w` equal to a glyph width) are not in the exam.

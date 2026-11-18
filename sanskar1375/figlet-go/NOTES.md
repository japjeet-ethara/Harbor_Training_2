# NOTES — pyfiglet (source of the Go port)

| | |
|---|---|
| URL | https://github.com/pwaller/pyfiglet |
| Commit | `255827efbc09d662875dad50fa0edc1e1263bf45` |
| Commit date | `2026-08-02T22:51:18+01:00` |
| Licence | MIT (`LICENSE`, "The MIT License (MIT)") |

## Why `python -m pyfiglet hello` fails straight after cloning

The code loads fonts with `importlib.resources.files('pyfiglet.fonts')`, but the
repository keeps them in `pyfiglet/fonts-standard/` (and `fonts-contrib/`).
`pyfiglet/fonts/` does not exist, so the run dies with
`ModuleNotFoundError: No module named 'pyfiglet.fonts'`.

The fix is the `Makefile` target `minimal` (via `clean`):

```
rm -rf pyfiglet/fonts/ && mkdir pyfiglet/fonts/
cp pyfiglet/fonts-standard/* pyfiglet/fonts
```

We only use `fonts-standard/` (single licence); `fonts-contrib/` is out.

## How a font is read (`FigletFont.loadFont`)

* File decoded as UTF-8 (`replace`); U+0085/U+2028/U+2029 become spaces; split with `str.splitlines()`.
* Header: `flf2a<hardblank> height baseline maxlen oldlayout commentlines [printdir [fulllayout]]`.
  If `fulllayout` is missing: `oldlayout 0 → 64`, `<0 → 0`, else `(old & 31) | 128`.
* Each glyph is `height` lines. The end-mark is the last non-space char of the glyph's
  first line; one or two of it (plus trailing space) are cut from every line.
  Glyph width = longest line.
* Codes 32–126 in order (a blank glyph is dropped, except space), then `ÄÖÜäöüß`,
  then extra glyphs whose tag line starts with `0x` (hex code). Tags not starting with `0x` are skipped.
* `printdir 1` (e.g. `ivrit`) makes `-D auto` mean right-to-left, and then `-j auto` means right.

## Layout bits (`smushMode` = full layout)

`64` = kerning, `128` = smushing; low 6 bits pick the rules below. If `128` is set and the
low 6 bits are 0 → **universal smushing**.

| Font | layout | mode |
|---|---|---|
| standard, big, slant, small | 24463 / 24463 / 18319 / 22415 | smush, rules 1+2+4+8 |
| mini, shadow | 1920 / 4992 | universal smushing |
| banner, script | 64 / 3904 | kerning only |
| ivrit | 16271, printdir 1 | smush, rules 1+2+4+8, right-to-left by default |

## Smushing rules (`FigletBuilder.smushChars`) — each checked by calling the real function

General guards, in this order:
1. If either side is a space, the other side wins (`' '` + `x` → `x`).
2. If the previous or current glyph is narrower than 2 → no smush.
3. Kerning-only font → no smush (glyphs just touch).
4. Universal smushing: hardblank loses to the other side; otherwise the **later typed** char wins
   (right in left-to-right, **left** in right-to-left).
5. A hardblank (`$`) touching anything else → no smush (unless rule 6).

| # | Bit | Rule | Example (left, right → result) |
|---|---|---|---|
| 1 | 1 | Equal character | `\|` `\|` → `\|`; `_` `_` → `_` |
| 2 | 2 | Underscore is replaced by `\|/\[]{}()<>` | `_` `\|` → `\|` |
| 3 | 4 | Hierarchy `\|` < `/\` < `[]` < `{}` < `()` < `<>`; higher class wins | `\|` `/` → `/`; `/` `[` → `[`; `{` `(` → `(` |
| 4 | 8 | Opposite pair → `\|` | `[` `]` → `\|`; `}` `{` → `\|` |
| 5 | 16 | Big X — **pyfiglet's version** | `/` `\` → `\|`; `\` `/` → `Y`; `>` `<` → `X` |
| 6 | 32 | Hardblank + hardblank → hardblank | `$` `$` → `$` |
| — | 64 | Kerning: glyphs moved together until they touch, no overlap | |

Rules 5 and 6 are in the code but **no font in `fonts-standard/` turns them on**, so the
exam cannot test them (noted, not a bug in the exam).

`smushAmount`: for every row, overlap = blanks at end of the left row + blanks at start of the
right row, +1 if the two edge chars smush (or the left edge is blank). The minimum over rows is
used, capped at the current glyph width.

## Options covered by the task

| Opt | Meaning |
|---|---|
| `-f FONT` | font name (default `standard`) |
| `-w COLS` | width for wrapping and justification (default 80). Wraps at the last space, else mid-word. A glyph wider than COLS → crash |
| `-j auto\|left\|center\|right` | `center` pads `(COLS-len)//2`; `right` pads `COLS-len-1`; auto = left, or right if direction is RTL |
| `-D auto\|left-to-right\|right-to-left` | auto = font's printdir. RTL builds the line leftwards (new glyph on the left) |
| `-r` | mirror each line and swap `/\ () [] {} <>` |
| `-F` | flip upside down (lines reversed; `^`↔`v`, `/`↔`\`, `_`→`-`, some letters swapped) |
| `-s` | strip blank lines before/after and trailing whitespace; wins over `-n` |
| `-n` | exactly one empty line before and after |

Text = all remaining args joined with one space. Output always ends with an extra `\n`.
Order applied: render → `-r` → `-F` → `-s` / `-n`.

## Rare cases (things that are easy to get wrong)

* Big X in pyfiglet: `\` + `/` gives `Y`, `/` + `\` gives `|` — not `X` as many people expect.
* After a line wrap, `prevCharWidth` is set to the width of the glyph that caused the wrap (not 0).
* Universal smushing picks the **left** char in RTL.
* `-j right` pads to `COLS-1`, not `COLS`.
* RTL with `-D auto` comes from the font (`ivrit`), and `-j auto` then becomes right.
* A word longer than the line is cut mid-word; a glyph exactly as wide as COLS with no blank
  column loops forever in pyfiglet (excluded from the exam).
* Characters missing from the font are silently skipped; `é` etc. are missing in most fonts,
  `ÄÖÜäöüß` usually exist.
* `-s` also strips trailing spaces of the last line; `-n` keeps them.
* Rows are compared byte-for-byte, so trailing spaces matter.

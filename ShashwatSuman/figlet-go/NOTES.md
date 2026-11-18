# NOTES — pyfiglet (source of truth for the Go port)

- URL: https://github.com/pwaller/pyfiglet
- Commit: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: `2026-08-02T22:51:18+01:00`
- Licence: **MIT** (see `LICENSE`, also repeated in `pyfiglet/__init__.py`)

## What it is

pyfiglet is a pure-Python port of FIGlet. It reads `.flf` font files and draws
text as large ASCII-art letters. All the rendering lives in
`pyfiglet/__init__.py`: `FigletFont` parses a font, `FigletBuilder` places
characters side by side ("smushing"), wraps lines, and justifies them, and
`FigletString` implements reverse (`-r`), flip (`-F`) and the newline
trimming options (`-s`, `-n`).

## Why `python -m pyfiglet hello` fails straight after cloning

The code loads fonts with `importlib.resources.files('pyfiglet.fonts')`, but
the repository only has `pyfiglet/fonts-standard/` and `pyfiglet/fonts-contrib/`.
There is no `pyfiglet/fonts/`, so you get:

```
ModuleNotFoundError: No module named 'pyfiglet.fonts'
```

The `Makefile` fixes this in `clean` + `minimal`:

```
rm -rf pyfiglet/fonts/ && mkdir pyfiglet/fonts/
cp pyfiglet/fonts-standard/* pyfiglet/fonts
```

We only use `fonts-standard/` (72 files, includes `__init__.py`, which makes
`fonts` a package). `fonts-contrib/` comes from many authors and is excluded.

## Font file parsing (details that matter for byte-exact output)

- File decoded as UTF-8 with `errors='replace'` (`bubble`, `digital`, `term`
  contain invalid UTF-8). `\u0085    ` become spaces, then
  `str.splitlines()` (splits on `\r\n`, `\r`, `\n`, `\v`, `\f`, `\x1c-\x1e`, ...).
- Header `flf2a$ H base maxlen oldLayout commentLines [printDir [fullLayout]]`.
  If `fullLayout` is missing: `old==0 -> 64`, `old<0 -> 0`, else `(old&31)|128`.
- End marker = last non-whitespace char of the *first* row of each glyph;
  each row loses up to two trailing end markers plus trailing whitespace.
- Glyph width = longest row (in code points).
- Chars 32..126 are read in order; a glyph that is entirely empty is dropped
  (except space). Then the 7 German chars `ÄÖÜäöüß`, then extra glyphs whose
  code line starts with `0x` (hex). **Decimal code lines are ignored** (quirk).

## Smushing rules (`FigletBuilder.smushChars`)

Smushing only runs if both the previous and the current character are at least
2 wide, and the font's layout has bit 128 (smush). Spaces always give way:
`' ' + x -> x`, `x + ' ' -> x`.

| # | Rule (bit) | Example (left + right -> result) |
|---|---|---|
| 1 | Equal chars (1) | `|` + `|` -> `|` |
| 2 | Underscore (2): `_` loses to any of `|/\[]{}()<>` | `_` + `/` -> `/` |
| 3 | Hierarchy (4): `|` < `/\` < `[]` < `{}` < `()` < `<>`, higher class wins | `|` + `/` -> `/`, `[` + `{` -> `{` |
| 4 | Opposite pair (8): `[]`, `{}`, `()` in either order | `[` + `]` -> `|` |
| 5 | Big X (16) — **pyfiglet differs from C figlet here** | `/` + `\` -> `|` (not `X`!), `\` + `/` -> `Y`, `>` + `<` -> `X` |
| 6 | Hardblank (32) | `$` + `$` -> `$` (any other mix with a hardblank cannot smush) |
| — | Universal smush (layout & 63 == 0, bit 128 set) | later char wins (`a` + `b` -> `b`; in right-to-left the *left* one wins); hardblank always loses |
| — | Kerning (64 without 128) | letters touch but never overlap |

Python quirk carried over: `'' in "abc"` is `True`, so an empty left edge
"matches" the underscore / hierarchy rules (only matters when the result is
discarded).

## Options

| Option | Meaning |
|---|---|
| `-f FONT` | font name (default `standard`) |
| `-w COLS` | width used for wrapping and justification (default 80). A glyph wider than COLS is an error. |
| `-j auto\|left\|center\|right` | `auto` = left for LTR, right for RTL. right pads `COLS - len - 1` spaces; center pads `int((COLS - len)/2)` per row |
| `-D auto\|left-to-right\|right-to-left` | `auto` = font header (only `ivrit` is RTL). RTL swaps the two sides while smushing |
| `-r` | mirror: each row reversed, `/`↔`\`, `(`↔`)`, `[`↔`]`, `{`↔`}`, `<`↔`>` |
| `-F` | flip: rows in reverse order; `/`↔`\`, `A`↔`V`, `M`↔`W`, `^`↔`v`, `_`→`-`, `P,R`→`b`, `b`→`P`, `m`↔`w` |
| `-s` | drop leading blank lines and `rstrip()` the end (beats `-n` if both) |
| `-n` | like `-s`, then exactly one `\n` before and after |

Text = all positional args joined with one space. Output = rendered text +
one extra `\n`. Out of scope: `-c`, `-L`, `-l`, `-i`.

## Rare cases list (where a naive port goes wrong)

- `/` + `\` gives `|` and `\` + `/` gives `Y` (pyfiglet bug kept on purpose).
- Wrapping: break at the last space (the space is dropped); otherwise break
  before the char that would reach `width` (`>=`, not `>`).
- After a wrap `prevCharWidth` is set to the width of the char that caused
  the wrap, not 0.
- A line whose first row is empty is not flushed at the end.
- `-j right` uses `width - len - 1`; RTL auto-justifies right.
- Chars missing from the font are silently skipped (e.g. `é` in `banner`).
- Characters not in the font still count for nothing; double spaces are kept.
- `-s` uses `rstrip()` so the last row also loses trailing spaces.
- Some inputs never finish in pyfiglet (glyph exactly as wide as `-w`); those
  are never used as test cases.

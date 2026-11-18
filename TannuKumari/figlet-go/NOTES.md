# NOTES — pyfiglet study (Step 1)

## Provenance

| Item | Value |
|---|---|
| Upstream URL | https://github.com/pwaller/pyfiglet |
| Pinned commit | `255827efbc09d662875dad50fa0edc1e1263bf45` |
| Commit date | `2026-08-02T22:51:18+01:00` |
| Licence | **MIT** (`LICENSE`: "PyFiglet: An implementation of figlet written in Python / The MIT License (MIT) / Copyright © 2007-2018 Christopher Jones, Stefano Rivera, Peter Waller and contributors") |
| Version module | `pyfiglet/version.py` |
| Fonts used by this task | `pyfiglet/fonts-standard/` only (72 files). `fonts-contrib/` is excluded (many authors, mixed licences). |

## Why `python -m pyfiglet hello` fails straight after cloning

`FigletFont.preloadFont()` resolves fonts through
`importlib.resources.files('pyfiglet.fonts').joinpath('<font>.flf')`
— i.e. the package directory **`pyfiglet/fonts/`**. In a fresh clone that directory
does not exist; the fonts ship in **`pyfiglet/fonts-standard/`** (and
`pyfiglet/fonts-contrib/`). So `preloadFont` finds nothing and raises
`FontNotFound: standard`.

The project's own `Makefile` does the copy. Relevant targets:

```make
clean:
	rm -rf build/
	rm -rf pyfiglet/fonts/
	mkdir pyfiglet/fonts/

minimal:    clean
	cp pyfiglet/fonts-standard/* pyfiglet/fonts
	python3 -m build
```

So the one setup step that matters (the part this task reuses) is:

```sh
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

Verified locally: after that copy, `python3 -m pyfiglet -f slant "hello"` prints the
giant slanted letters. 72 `.flf` files land in `pyfiglet/fonts/`.

## The font file format, as this code parses it (`FigletFont.loadFont`)

* Header line must match `^[tf]lf2.` — e.g. `flf2a$ 6 5 16 15 13 0 24463 229`.
  Fields after the magic: `hardBlank height baseLine maxLength oldLayout
  commentLines [printDirection] [fullLayout]`.
* If `fullLayout` is absent: `oldLayout == 0 → 64` (kern only),
  `oldLayout < 0 → 0` (full width), `oldLayout > 0 → (oldLayout & 31) | 128`.
* `commentLines` lines are skipped.
* Then characters **32..126** in order, `height` lines each. The end marker is the
  last non-blank character of the character's first line (`(.)\s*$`); each line is
  then stripped of `marker{1,2}\s*$`. Char width = longest line.
  Char 32 (space) is always kept even if blank; other blank characters are dropped.
* Then, if data remains, the seven German letters `ÄÖÜäöüß` in that order.
* Then code-tagged blocks: a line whose first token starts with `0x`/`0X`
  (parsed base 16) followed by `height` lines.
* `smushMode = fullLayout`, `printDirection` may be `None`.

## The smushing rules (`FigletBuilder.smushChars`) — six, plus kerning

Layout bits lifted from figlet222:

| Bit | Name | Rule | Example (left + right → result) |
|---|---|---|---|
| 1 | `SM_EQUAL` | two identical characters (hardblanks excluded) become one | `|` + `|` → `|` |
| 2 | `SM_LOWLINE` | `_` is eaten by any of `` |/\[]{}()<> `` | `_` + `/` → `/` ; `\` + `_` → `\` |
| 4 | `SM_HIERARCHY` | classes ranked `` | `` < `/\` < `[]` < `{}` < `()` < `<>`; the higher class wins | `|` + `>` → `>` ; `)` + `[` → `)` |
| 8 | `SM_PAIR` | opposite brackets in either order become `|` | `[` + `]` → `|` ; `)` + `(` → `|` |
| 16 | `SM_BIGX` | `/`+`\` → `|`, `\`+`/` → `Y`, `>`+`<` → `X` | `/` + `\` → `|` |
| 32 | `SM_HARDBLANK` | hardblank + hardblank → hardblank (any other pairing with a hardblank is refused) | `$` + `$` → `$` |
| 64 | `SM_KERN` | kerning: push the characters together until they touch, never overlap | — |
| 128 | `SM_SMUSH` | enables overlapping at all; without it `smushChars` only merges blanks | — |

Each example above only applies when the font's `fullLayout` has that bit set.
Checked against the real code (`standard`, `smushMode = 24463`, hardblank `$`,
so bits 1|2|4|8 set, `SM_BIGX`/`SM_HARDBLANK` **not** set):

```
'|' + '|' -> '|'      '_' + '/' -> '/'      '\' + '_' -> '\'
'|' + '>' -> '>'      ')' + '[' -> ')'      '[' + ']' -> '|'
')' + '(' -> '|'      '/' + '\' -> None     '$' + '$' -> None
```

Layout modes of the eight fonts this task uses (header field 8):

| Font | `fullLayout` | `& 63` | meaning |
|---|---|---|---|
| standard, big | 24463 | 15 | smush, bits 1\|2\|4\|8 |
| slant | 18319 | 15 | smush, bits 1\|2\|4\|8 |
| small | 22415 | 15 | smush, bits 1\|2\|4\|8 |
| mini | 1920 | 0 | **universal smushing** |
| shadow | 4992 | 0 | **universal smushing** |
| script | 3904 | 0 | `SM_SMUSH` unset → **kerning only** |
| banner | 64 | 0 | `SM_KERN` only |

All eight have `printDirection = 0`, so `-D auto` means left-to-right everywhere.

Order of checks inside `smushChars`, which must be copied exactly:

1. `left == ' '` → return right; `right == ' '` → return left.
2. If `prevCharWidth < 2` or `curCharWidth < 2` → refuse (no overlap for 0/1-wide glyphs).
3. If `SM_SMUSH` is unset → refuse (kerning only).
4. **Universal smushing**: if `smushMode & 63 == 0`, hardblank loses to a visible
   char, otherwise the *later* character in the text wins
   (`right` for left-to-right, `left` for right-to-left).
5. `SM_HARDBLANK`, then: any remaining hardblank pairing → refuse.
6. `SM_EQUAL`, then `SM_LOWLINE`/`SM_HIERARCHY` table (returns the stronger side),
   then `SM_PAIR`, then `SM_BIGX`. Anything else → refuse.

`smushAmount` then takes the **minimum** overlap allowed over all rows, capped at
the current character's width; `+1` extra when the left edge is blank/empty or when
`smushChars` accepts the touching pair.

## The eight options this task covers (`main()`, optparse)

| Option | Effect |
|---|---|
| `-f FONT` | font name, loaded from the `pyfiglet.fonts` package dir (default `standard`) |
| `-w COLS` | terminal width used for wrapping **and** justification (default 80) |
| `-j auto\|left\|center\|right` | justification. `auto` = `left` for left-to-right, `right` for right-to-left. `right` pads with `width - len(row) - 1` spaces (note the `-1`), `center` pads with `int((width - len(row)) / 2)` |
| `-D auto\|left-to-right\|right-to-left` | print direction. `auto` uses the font header's `printDirection` (`1` → right-to-left, anything else → left-to-right). Direction also swaps which side dominates when smushing |
| `-r` | reverse/mirror: per row, translate through `__reverse_map__` (`/`↔`\`, `(`↔`)`, `[`↔`]`, `{`↔`}`, `<`↔`>`) then reverse the row |
| `-F` | flip upside down: reverse the row *order* and translate through `__flip_map__` (`A`→`V`, `V`→`A`, `W`→`M`, `M`→`W`, `b`→`q`… `^`→`v`, `v`→`-`, `_`→`-`) |
| `-s` | strip surrounding newlines: drop leading blank rows, then `rstrip()` the whole string (so trailing blank rows *and* trailing spaces of the last line go) |
| `-n` | normalize surrounding newlines: exactly one empty line before and after (`'\n' + strip_surrounding_newlines() + '\n'`). `-s` wins when both are given (`elif`) |

Out of scope for the task: `-c/--color`, `-L/--load`, `-l/--list_fonts`, `-i/--info_font`.

Two details that bite a port:

* `main()` writes the rendered string **and then one extra `b'\n'`**, so normal
  output ends with a blank line.
* Text comes from `' '.join(args)`, and the renderer works on **code points**
  (Python `str` indexing), not bytes — so Latin-1 letters count as one column.

## Rare cases list (for the test generator)

Collected from reading the code; every one of these exercises a branch that a
naive port gets wrong:

* `|_|`, `_|_`, `|||`, `___` — `SM_EQUAL` vs `SM_LOWLINE`
* `/\`, `\/`, `><`, `<>`, `X`, `Y` — `SM_BIGX` and hierarchy
* `[]`, `{}`, `()`, `)(`, `}{`, `][` — `SM_PAIR`
* `i`, `l`, `!`, `.`, `'`, `|` in narrow fonts — glyphs of width < 2 (overlap refused)
* double/triple spaces, leading and trailing spaces — blank markers and wrapping
* long sentences at `-w 10/20` — `handleNewLine`, `cutBufferAtLastBlank`
* `-D right-to-left` with `-j right`/`-j center` — both pad on the left
* `é ß ñ ü Ä Ö` — the German block and code-tagged characters
* `€ ☃ 中` — characters the font does not have (silently skipped by `getCharAt`)
* fonts whose header has no `fullLayout` (old layout conversion) and fonts with
  `printDirection = 1`

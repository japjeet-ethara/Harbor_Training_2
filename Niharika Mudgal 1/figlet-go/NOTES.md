# NOTES — pyfiglet, as needed for the `figlet-go` task

## Source

| | |
|---|---|
| URL | https://github.com/pwaller/pyfiglet |
| Commit (SHA) | `255827efbc09d662875dad50fa0edc1e1263bf45` ("Return False from isValidFont for a missing font (#161)") |
| Commit date | `2026-08-02T22:51:18+01:00` |
| Licence | **MIT** (`LICENSE`, and the header of `pyfiglet/__init__.py`) |
| Only fonts used | `pyfiglet/fonts-standard/` (72 files). `fonts-contrib/` is deleted from the image. |

## Why `python -m pyfiglet hello` fails straight after cloning, and the fix

The code finds fonts with `importlib.resources.files('pyfiglet.fonts')`, i.e. the package folder
`pyfiglet/fonts/`. The repository does **not** contain that folder; it ships the fonts in
`pyfiglet/fonts-standard/`. So right after cloning you get
`ModuleNotFoundError: No module named 'pyfiglet.fonts'`.

The project's own `Makefile` (target `minimal`; `setup.py` does the same with `shutil.copytree`) fixes it:

```
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

The Dockerfile's stage 1 does exactly this step. Note that it also copies `fonts-standard/__init__.py`
into `fonts/`, so a Python file lives among the fonts (the oracle deletes it with all other `.py` files).

## Smushing (`FigletBuilder.smushChars`) — six rules plus kerning

Every font has a layout number in its header (`smushMode`). Its low bits switch rules on:

| Bit | Rule | Behaviour | Example (left + right → result) |
|---|---|---|---|
| 1 | equal character | two identical characters (not hardblanks) become one | `\|` + `\|` → `\|` |
| 2 | underscore | `_` loses to any of `\| / \ [ ] { } ( ) < >` | `_` + `\|` → `\|`; `/` + `_` → `/` |
| 4 | hierarchy | classes weakest→strongest: `\|`, `/ \`, `[ ]`, `{ }`, `( )`, `< >`; the stronger one wins | `\|` + `/` → `/`; `[` + `{` → `{`; `(` + `<` → `<` |
| 8 | opposite pair | `[]`, `{}`, `()` (either order) become `\|` | `[` + `]` → `\|`; `}` + `{` → `\|` |
| 16 | big X | `/` + `\` → `\|`; `\` + `/` → `Y`; `>` + `<` → `X` (`<` + `>` does nothing) | `\` + `/` → `Y` |
| 32 | hardblank | two hardblanks become one hardblank; a hardblank never smushes with anything else | `$` + `$` → `$`; `$` + `a` → no |
| 64 | **kerning** | glyphs slide together until they touch; nothing overlaps | (no overlap rule) |
| 128 | **smushing** | overlapping allowed; combined with rules 1–32 above | |

Other facts the code relies on:

- A space on either side always yields the other character (`' '` + `x` → `x`).
- If neither bit 64 nor 128 is set, nothing is squeezed (full width).
- If bit 128 is set but none of bits 1–32 are (`smushMode & 63 == 0`): **universal overlap**. A hardblank loses to the other character; otherwise the later character in the *text* wins (`right` in left-to-right, `left` in right-to-left).
- Nothing smushes when the previous or the current glyph is narrower than 2 columns.
- Layout numbers in the header may be larger than 255; only the low 8 bits matter here.
- If the header has no "full layout" field, it is derived from the old layout: `0` → kerning (64), `< 0` → full width (0), else `(old & 31) | 128`.

The eight tested fonts: `standard`, `slant`, `small`, `big` = rules 1+2+4+8 with smushing (143); `mini`, `shadow` = universal overlap (128); `banner`, `script` = kerning only (64).
**None of them uses bit 16 or 32**, so the hidden cases cannot catch an error in those two rules. The oracle ports them anyway (checked against all 72 fonts, see below).

## The eight options (`main()` + `Figlet`)

| Option | What it does |
|---|---|
| `-f FONT` | Loads `FONT.flf` (a `.tlf` of the same name would be tried first). Missing font → prints `pyfiglet error: requested font 'X' not found.` and exits 1. Default `standard`. |
| `-w COLS` | Width, default 80. Wrapping: when adding the next glyph would make the line **≥** `COLS` wide, the line is cut at the last space (or at the last glyph if there was none). A glyph wider than `COLS` makes pyfiglet crash. |
| `-j SIDE` | `auto` → `left` for left-to-right, `right` for right-to-left. `left` adds nothing. `right` pads each row with `COLS - len(row) - 1` spaces (one less than you might think). `center` pads with `int((COLS - len(row)) / 2)` spaces. Negative → nothing. |
| `-D DIR` | `auto` → the font header's print direction (`0` → left-to-right, `1` → right-to-left, otherwise left-to-right). With `right-to-left` each new glyph is added on the **left** of the line, and the two sides are swapped inside the smushing code. |
| `-r` | Mirror: each row is reversed and `( )`, `[ ]`, `{ }`, `< >`, `/ \` are swapped. |
| `-F` | Flip: row order is reversed and characters are swapped (`/`↔`\`, `A`↔`V`, `M`↔`W`, `b`↔`P`, `m`↔`w`, `^`↔`v`, `_`→`-`, …). |
| `-s` | Strip: drop empty leading rows, then `rstrip()` the whole text (also removes trailing spaces of the last row). Wins over `-n`. |
| `-n` | Normalize: `"\n" + stripped text + "\n"` — exactly one empty line before and after. Only used when `-s` is not given. |

Pipeline: render → `-r` → `-F` → (`-s` or `-n`) → write UTF-8 bytes + `"\n"`.
`-r` and `-F` rebuild the text with `"\n".join(rows) + "\n"`, so an **empty** render becomes `"\n"` after them.
Out of scope: `-c`, `-L`, `-l`, `-i`.

## Rare cases (feed these to `gen.py`; each is a place where a port is easy to get wrong)

1. **The task page says `/` + `\` makes an `X`. The code says otherwise**: `/`+`\` → `|`, `\`+`/` → `Y`, `>`+`<` → `X`. Always trust the code.
2. `-j right` pads `width - len - 1`; `-j center` truncates `(width - len)/2` toward zero.
3. Right-to-left swaps `addLeft`/`addRight` in `smushRow` and `lineLeft`/`lineRight` in `smushAmount`. Mixing this up with `-j right` is the classic bug.
4. `|_|`, `_|_`, `[]`, `}{`, `)(`, `/\`, `\/`, `><`, `<>` — every pair above at glyph borders, in narrow windows, with `-D right-to-left`.
5. Wrapping uses `>=`, remembers blanks in a stack (`blankMarkers`), and after cutting at the last character it retries that same character. `prevCharWidth` is reset in `cutBufferCommon` and then overwritten again by `addCharToProduct`.
6. A newline inside the text is not a font glyph; it is handled before the "glyph missing" check.
7. Characters that the font lacks are skipped silently (`日本語`, `€`); the space glyph is always kept; `é ß ñ Ä Ö Ü` exist only in fonts that have the German block or code-tagged glyphs.
8. Font parsing quirks: the end-marker is the last non-space character of the first row; the 7 "German" glyphs are read directly after glyph 126 whenever any lines remain; later glyphs need a `0x…` tag line; a glyph whose rows are all empty is dropped.
9. Hardblanks are replaced by spaces **last**, after justification, so they count as real columns until then.
10. Python's `'' in 'abc'` is `True`: at the start of a line `smushChars` is called with an empty left character.
11. Empty text → output is just `"\n"`; with `-r` or `-F` the same.
12. **The original crashes or never finishes** on some inputs: glyph wider than `-w` (`CharNotPrinted`), `-w` exactly equal to a glyph's width (endless loop), some right-to-left starts (`IndexError`). `gen.py` drops such cases (1 second timeout), so they are never graded. About 100 candidate inputs per 4000 are dropped.

## How the port was checked

- `tests/gen.py` → 2000 visible + 2000 hidden cases (disjoint), identical on every run (checked with `diff`).
- `dev/check.py` runs a built `/tmp/figlet` over a case file and prints the first failures: **4000/4000 match**.
- Extra fuzz, not part of the task: ~3000 random cases over **all 72 fonts** compared with the original — 0 differences.

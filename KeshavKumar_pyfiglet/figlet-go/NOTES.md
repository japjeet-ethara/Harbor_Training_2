# pyfiglet — study notes (Practice Task 2, Step 1)

## Source

| Field   | Value |
|---------|-------|
| URL     | https://github.com/pwaller/pyfiglet |
| Commit  | `255827efbc09d662875dad50fa0edc1e1263bf45` |
| Date    | `2026-08-02T22:51:18+01:00` (`git log -1 --format='%H %cI'`) |
| Licence | **MIT** (`LICENSE`, also `license='MIT'` in `setup.py`) |
| Fonts used | `pyfiglet/fonts-standard/` only (72 files). `fonts-contrib/` is out of scope. |

All engine code is in `pyfiglet/__init__.py`: `FigletFont` (parse `.flf`),
`FigletBuilder` (layout + smushing), `FigletString` (reverse/flip/strip),
`main()` (command line).

## Why `python -m pyfiglet hello` fails straight after cloning

```
ModuleNotFoundError: No module named 'pyfiglet.fonts'
```

`FigletFont.preloadFont` loads fonts with
`importlib.resources.files('pyfiglet.fonts')`, but in the git checkout the
fonts live in `pyfiglet/fonts-standard/` and `pyfiglet/fonts/` does not exist.

**Fix (Makefile, targets `clean` + `minimal`):**

```
rm -rf pyfiglet/fonts/ && mkdir pyfiglet/fonts/      # make clean
cp pyfiglet/fonts-standard/* pyfiglet/fonts          # make minimal
```

(`setup.py` does the same with `shutil.copytree` at install time.)
After that, `python -m pyfiglet -f slant hello` prints the slanted banner.

## Font file facts that matter for a port

- Header: `flf2a<hardblank> height baseline maxlen oldLayout commentLines [printDirection [fullLayout]]`.
  If `fullLayout` is missing: `oldLayout==0 → 64`, `<0 → 0`, else `(oldLayout & 31) | 128`.
- Each glyph row ends with an end-marker char (taken from the **first** row of the glyph);
  one or two of them plus trailing whitespace are removed. Glyph width = longest row.
- Glyphs 32..126 in order (a glyph that is all-empty is dropped, except space), then 7 German
  chars `ÄÖÜäöüß`, then optional `0xNNNN` code-tagged glyphs.
- File decoded as UTF-8 with `replace`; `\u0085  ` replaced by space before `splitlines()`.
- The hardblank (`$` in all 8 task fonts) is a solid char during layout and becomes a space only at output.

The 8 fonts the task uses (measured, not guessed):

| font | height | smushMode | rules switched on |
|------|--------|-----------|-------------------|
| standard | 6 | 24463 | SMUSH + EQUAL, LOWLINE, HIERARCHY, PAIR |
| slant    | 6 | 18319 | SMUSH + EQUAL, LOWLINE, HIERARCHY, PAIR |
| small    | 5 | 22415 | SMUSH + EQUAL, LOWLINE, HIERARCHY, PAIR |
| big      | 8 | 24463 | SMUSH + EQUAL, LOWLINE, HIERARCHY, PAIR |
| mini     | 4 | 1920  | SMUSH only → **universal smushing** |
| shadow   | 5 | 4992  | SMUSH only → **universal smushing** |
| banner   | 8 | 64    | KERN only (no overlap) |
| script   | 7 | 3904  | KERN only (no overlap) |

All have `printDirection = 0` (left-to-right). **None of them switch on BIGX or HARDBLANK**, so
those two rules can be ported but cannot be exercised by tests using these fonts.

## Smushing rules (`FigletBuilder.smushChars`)

Two glyphs touch at one column per row. `smushAmount` finds how many columns the new glyph can
slide left (minimum over all rows); `smushChars(left, right)` decides what the touching column
becomes, or `None` = not allowed. Order of checks (each example verified by calling
`smushChars` on the real code):

0. **Space always yields:** `' ' + x → x`, `x + ' ' → x`.
   **Width guard:** if the previous *or* current glyph is narrower than 2 → no smush (`None`).
   If the font has no SMUSH bit (128) → no smush (kerning only).
   **Universal smushing** (SMUSH set, rules bits 1..32 all off — `mini`, `shadow`):
   hardblank loses to the visible char; otherwise the *later* glyph in the text wins:
   left-to-right `a + b → b`, right-to-left `a + b → a`.

| # | Rule (bit) | Meaning | Example |
|---|------------|---------|---------|
| 1 | EQUAL (1) | two identical chars become one (not hardblanks) | `\| + \| → \|` |
| 2 | LOWLINE (2) | `_` is replaced by any of `\| / \ [ ] { } ( ) < >` | `_ + / → /`, `/ + _ → /` |
| 3 | HIERARCHY (4) | classes `\|` < `/\` < `[]` < `{}` < `()` < `<>`; the higher class wins | `\| + / → /`, `[ + { → {`, `( + < → <` |
| 4 | PAIR (8) | opposite brackets become `\|` | `[ + ] → \|`, `] + [ → \|`, `{ + } → \|`, `( + ) → \|` |
| 5 | BIGX (16) | `/ + \ → \|`, `\ + / → Y`, `> + < → X` (`< + >` → none) | (only `> <` gives an `X`!) |
| 6 | HARDBLANK (32) | two hardblanks become one hardblank | `$ + $ → $` |
| — | KERN (64) | glyphs slide together until they touch, never overlap | `banner`, `script` |

- A hardblank next to anything other than a hardblank never smushes (`$ + \| → None`).
- Rule order matters: HARDBLANK, then "any hardblank → None", EQUAL, LOWLINE+HIERARCHY
  (checked together as one table), PAIR, BIGX.

## Command-line options (only these are in scope)

All positional arguments are joined with **one space**: `' '.join(args)`.
Pipeline: render → `-r` → `-F` → `-s` *or* `-n` → write result + an extra `\n` (UTF-8 bytes).

| Option | Effect |
|--------|--------|
| `-f FONT` | font name (default `standard`); looks for `FONT.tlf` then `FONT.flf` |
| `-w COLS` | width (default 80). A line breaks when its width would be `>= COLS`; used by justify |
| `-j auto\|left\|center\|right` | `auto` = left for LTR, right for RTL. right pads `COLS - len - 1` spaces; center pads `int((COLS - len) / 2)` |
| `-D auto\|left-to-right\|right-to-left` | `auto` = font header (all 8 fonts → LTR). RTL puts each new glyph to the **left** |
| `-r` | mirror: each row is char-swapped (`/`↔`\`, `(`↔`)`, `[`↔`]`, `{`↔`}`, `<`↔`>`) then reversed |
| `-F` | flip upside down: row order reversed, chars swapped (`/`↔`\`, `^`↔`v`, `_`→`-`, `A`↔`V`, `M`↔`W`, `b`↔`P`, `m`↔`w`, `R`→`b`) |
| `-s` | drop leading blank lines, then `rstrip()` the whole string. **Wins over `-n`** |
| `-n` | `'\n' + (the -s result) + '\n'` |

Out of scope: `-c`, `-L`, `-l`, `-i`.

## Rare cases (verified against the real program)

1. **First glyph on a line loses its leading blank columns.** Against an empty buffer the smush
   amount is the glyph's minimum leading-space count, so standard `W` (width 13, 1 blank column)
   prints 12 wide and fits in `-w 13`.
2. **Infinite loop:** a glyph with *no* leading blank and width exactly `COLS` never fits and
   never errors, e.g. `-f slant -w 10 H` hangs forever. The case generator must use a timeout
   and drop these.
3. **Crash:** glyph wider than `COLS` raises `CharNotPrinted` → traceback on stderr, **empty
   stdout**, exit 1 (e.g. `-f big -w 5 W`, `-w 12 W`).
4. Missing glyphs (`é`, `€`, tab, …) are silently skipped. `ß` exists in most fonts, `é` does not.
5. Empty render outputs: none → `\n`; `-r` or `-F` → `\n\n`; `-n` → `\n\n\n`; `-s` → `\n`.
6. `-s` also strips **trailing spaces of the last row** (`rstrip()` on the whole string).
7. Wrapping cuts at the last space; that space is dropped. A word longer than the line is cut
   between characters. A `\n` inside an argument forces a line break.
8. `-D right-to-left` with `-j auto` becomes **right-justified** (`COLS - len - 1` spaces on the
   left) — easy to confuse with RTL itself adding spaces.
9. `-j center` uses the `-w` value, not 80.
10. Width guard: a 1-column glyph (e.g. `|` in some fonts) never overlaps its neighbour.
11. `mini` and `shadow` use universal smushing (later glyph wins), not the rule table.
12. The task page says "`/` and `\` meet as an X" — wrong for pyfiglet: `/\`→`|`, `\/`→`Y`,
    `><`→`X`. Not reachable with the 8 task fonts anyway.
13. Text starting with `-` would be parsed as an option — generator should avoid it.

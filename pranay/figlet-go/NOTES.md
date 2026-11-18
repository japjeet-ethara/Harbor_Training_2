# NOTES — pyfiglet (Step 1)

- **Repository:** https://github.com/pwaller/pyfiglet
- **Pinned commit:** `255827efbc09d662875dad50fa0edc1e1263bf45`
- **Commit date:** `2026-08-02T22:51:18+01:00`
- **Licence:** MIT (`LICENSE`: "The MIT License (MIT)"; `setup.py` also says `license='MIT'`)

## Why `python -m pyfiglet hello` fails right after cloning

`FigletFont.preloadFont()` loads fonts with `importlib.resources.files('pyfiglet.fonts')`.
The repository ships fonts in `pyfiglet/fonts-standard/` (and `pyfiglet/fonts-contrib/`),
but there is no `pyfiglet/fonts/` package, so the import fails:

    ModuleNotFoundError: No module named 'pyfiglet.fonts'

**Fix = the Makefile's `minimal` target** (the `clean` step makes the folder, then it copies):

    mkdir -p pyfiglet/fonts
    cp pyfiglet/fonts-standard/* pyfiglet/fonts

(The `full` target also copies `fonts-contrib/*`. We do NOT: those fonts come from many authors.)

Side effect to remember: `fonts-standard/` holds **72 files** = 67 `.flf` + 4 `.flc` + **1 `__init__.py`**.
The `__init__.py` is what makes `pyfiglet.fonts` importable, and it is a `.py` file, so the
solution must delete it too ("no `.py` file anywhere in the repository").

## Rendering pipeline (`FigletBuilder`)

1. `Figlet` picks `direction` (auto = font's header print-direction: 0 → left-to-right, 1 → right-to-left)
   and `justify` (auto → `left` for LTR, `right` for RTL).
2. Per input character: look up its glyph (unknown chars are silently skipped), compute how many
   columns it may overlap the text so far (`smushAmount`), and add it row by row (`smushRow`).
3. If the line would reach `width` (`currentTotalWidth >= width`), cut at the last space
   (the space is dropped) or, with no space, before this character (`handleNewLine`).
4. At the end each finished line is justified (`justifyString`), and hardblanks become spaces.
5. `main()` then applies `-r` → `-F` → (`-s` else `-n`) and prints the text + one extra `\n`.

## Kerning and the six smushing rules

The font header (`fullLayout`, or converted from `oldLayout`) is a bit mask:

| bit | value | name | meaning | tiny example (left, right → result) — checked against the original |
|---|---|---|---|---|
| – | 64 | **kerning** | glyphs may touch but not overlap (no rule applied) | – |
| – | 128 | smushing on | enables the rules below | – |
| 1 | 1 | **equal** | two identical chars (not hardblanks) → that char | `\|` `\|` → `\|` |
| 2 | 2 | **lowline** | `_` is replaced by any of `\|/\[]{}()<>` | `_` `\|` → `\|`; `/` `_` → `/`; `_` `-` → none |
| 3 | 4 | **hierarchy** | class order `\|` < `/\` < `[]` < `{}` < `()` < `<>`; later class wins | `\|` `/` → `/`; `[` `{` → `{`; `(` `<` → `<` |
| 4 | 8 | **pair** | opposite brackets → `\|` | `[` `]` → `\|`; `{` `}` → `\|`; `(` `)` → `\|` |
| 5 | 16 | **big X** | `/\` → `\|`, `\/` → `Y`, `><` → `X` (`<>` → none) | `/` `\` → `\|`; `\` `/` → `Y`; `>` `<` → `X` |
| 6 | 32 | **hardblank** | two hardblanks → one hardblank | `$` `$` → `$` (with `$` = hardblank) |

Extra behaviour that is easy to miss (and that the twin must copy):

- A space on either side always lets the other char through (`smushChars` returns the other char).
- If either glyph is narrower than 2 columns, nothing may overlap (`prevCharWidth < 2 or curCharWidth < 2`).
- If only kerning is on (`fullLayout & 128 == 0`) nothing smushes.
- If `fullLayout & 63 == 0` (and smushing on) → *universal* smushing: the later char wins
  (the right one for LTR, the left one for RTL), a hardblank never wins over a visible char.
- A hardblank never smushes with a non-hardblank (unless universal).
- `''` (no char) is treated by Python's `in` as "contained in anything" — the code relies on that
  when `idx < 0`. The twin uses Go `strings.Contains`, which has the same behaviour.

> ⚠ The task page said "`/` and `\` meet as an `X`". That is **wrong for this code**:
> `/\` gives `|`, `\/` gives `Y`, and `><` gives `X`. Put it on the rare-cases list.

**Which rules can actually fire?** Layouts of every exam font: standard/big 24463, slant 18319, small 22415,
mini 1920 and shadow 4992 (universal smushing), banner 64 and script 3904 (kerning only), ivrit 16271 (RTL header).
**No font in `fonts-standard` enables rule 16 (big X) or 32 (hardblank)**, so those two are dead code here and no exam can
test them; the twin still ports them. For universal fonts, "which side dominates" is also unobservable (checked for every
pair of printable ASCII). The narrow-glyph guard (`width < 2`) never triggers either (no glyph is narrower than 2).

**Coverage fonts.** The eight fonts from the task page never draw `[ ] { }` or the letters `-F` swaps, and none has an RTL
header. `gen.py` therefore also uses `ivrit, bubble, mnemonic, digital, term` (about 20% of cases, with targeted texts).
Mutation testing (28 deliberate bugs in the twin): all observable ones are caught by the hidden cases.

## Options (the eight in scope)

| option | meaning |
|---|---|
| `-f FONT` | font name (default `standard`); looks for `FONT.tlf` then `FONT.flf` |
| `-w COLS` | width for wrapping and justification (default 80) |
| `-j auto\|left\|center\|right` | justification; `auto` = `left` for LTR, `right` for RTL |
| `-D auto\|left-to-right\|right-to-left` | direction; `auto` = font's own (all exam fonts: LTR) |
| `-r` | mirror: reverse every row **and** swap `/ \ ( ) [ ] { } < >` |
| `-F` | flip upside down: reverse row order **and** swap `^`↔`v`, `b`↔`P`, `V`↔`A`, … |
| `-s` | strip empty lines before/after; the result is also `rstrip`ped |
| `-n` | like `-s` but then put exactly one `\n` before and after (ignored if `-s` is also given) |

Out of scope: `-c` colour, `-L` install font, `-l` list fonts, `-i` font info.

## Pitfalls found while reading the code (the "rare cases" list)

1. `-j right` pads `width - len(row) - 1` (one less than the width!); `-j center` pads `int((width-len)/2)`; negative → no padding.
2. `-D right-to-left` swaps which side is "left" inside `smushRow`; the *latter* char wins in universal smushing.
3. A glyph exactly as wide as `-w` makes pyfiglet loop forever (`currentTotalWidth >= width` with an empty buffer). A glyph wider than `-w` raises `CharNotPrinted`. The exam skips both.
4. Chars missing from the font are skipped entirely (no placeholder), and do not update the "previous width".
5. `-s` beats `-n` when both are given.
6. `-r` also swaps `/ \`, `( )`, `[ ]`, `{ }`, `< >`; `-F` has its own map (`^`/`v`, `b`/`P`, `V`/`A`, `M`/`W`, `m`/`w`, `_`/`-`, `\`/`/`).
7. `[` `]` → `|` (pair) but `/` `\` → `|` and `\` `/` → `Y` (big X). Different rules, easy to mix up.
8. Font lines end with `@`/`@@` (or another marker): the end-marker strip is "1–2 marker chars + trailing whitespace".
9. `fonts-standard/__init__.py` is a Python file inside the font folder.

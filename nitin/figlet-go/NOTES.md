# NOTES — pyfiglet, as pinned for the `figlet-go` Harbor task

## Source
- URL: https://github.com/pwaller/pyfiglet
- Pinned commit (SHA): `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: `2026-08-02T22:51:18+01:00` (from `git log -1 --format='%H %cI'`)
- Licence: **MIT** (`LICENSE`; the same text is in `__copyright__` in `pyfiglet/__init__.py`). Keep `LICENSE` in the repo.

## Why `python -m pyfiglet hello` fails straight after cloning
The code loads fonts with `importlib.resources.files('pyfiglet.fonts')`, but the repo only ships
`pyfiglet/fonts-standard/` and `pyfiglet/fonts-contrib/`. There is no `pyfiglet/fonts/` package, so you get
`ModuleNotFoundError: No module named 'pyfiglet.fonts'`.
The `Makefile` fixes it: target `minimal` runs `clean` (`rm -rf pyfiglet/fonts/; mkdir pyfiglet/fonts/`) and then
`cp pyfiglet/fonts-standard/* pyfiglet/fonts`. (`full` also copies `fonts-contrib`; we do not.) `setup.py` does the same
"set up minimum fonts" step. The Dockerfile's stage 1 repeats this step.
After it, `python -m pyfiglet -f slant hello` draws the slanted letters.

## Font files
`fonts-standard/` has 67 `.flf` fonts, 4 `.flc` control files and an `__init__.py` (a `.py` file the solution must delete too).
Three fonts (`bubble`, `term`, `digital`) are not valid UTF-8; pyfiglet decodes with `errors="replace"`. The Go port does the same.

## Rendering model (FigletBuilder)
Characters are added one at a time to a 2-D `buffer` (one string per font row). For each new character
`smushAmount()` decides how many columns it may overlap the buffer (kerning = overlap only blanks; smushing = also merge
touching characters using the font's rules). `smushChars(left, right)` returns the merged character or `None` ("cannot smush").
Width: a line may hold at most `width - 1` columns (`currentTotalWidth >= width` wraps). Wrapping goes back to the last blank.

### Smushing rules (bits of the font's "full layout"; `smushMode & 128` turns smushing on, `& 64` is kerning)
A space on either side always gives way (`' '` + x → x). Two characters of width < 2 never smush. Rule examples
(verified against the real code, left-to-right):

| Bit | Rule | Examples |
|---|---|---|
| 1 | **Equal character**: two identical characters (not hardblanks) become one | `\|`+`\|`→`\|`, `/`+`/`→`/`, `\|`+`/`→none |
| 2 | **Underscore**: `_` is replaced by any of `\|/\[]{}()<>` | `_`+`\|`→`\|`, `/`+`_`→`/`, `_`+`_`→none, `_`+`a`→none |
| 4 | **Hierarchy**: classes `\|` > `/\` > `[]` > `{}` > `()` > `<>`; the higher class wins | `\|`+`/`→`/`, `/`+`[`→`[`, `[`+`{`→`{`, `{`+`(`→`(`, `(`+`<`→`<`, `\`+`[`→`[` |
| 8 | **Opposite pair**: `[]`, `{}`, `()` (either order) become `\|` | `[`+`]`→`\|`, `{`+`}`→`\|`, `(`+`)`→`\|`, `[`+`}`→none |
| 16 | **Big X**: `/`+`\`→`\|`, `\`+`/`→`Y`, `>`+`<`→`X` (only these three; `<`+`>` is none) | `/`+`\`→`\|`, `\`+`/`→`Y`, `>`+`<`→`X` |
| 32 | **Hardblank**: two hardblanks become one hardblank; a hardblank never smushes with anything else (only checked when bit 32 is set) | `$`+`$`→`$`, `$`+`\|`→none |
| — | **Universal overlap** (smush bit set but bits 1–32 all zero): the later character in the text wins (the right one left-to-right, the left one right-to-left); a hardblank loses to any visible character | `a`+`b`→`b` (`a` when right-to-left) |
| 64 | **Kerning**: no merging at all, only blank columns are removed | `\|`+`\|`→none |

Note: the task page says "`/` and `\` meet as an `X`". In this code `/`+`\` gives `|`, and the `X` comes from `>`+`<`.
Smush modes of the fonts used in the tests: standard 24463, slant 18319, small 22415, big 24463, mini 1920, banner 64 (kerning only), shadow 4992, script 3904.

## Options (the eight in scope)
| Option | Meaning |
|---|---|
| `-f FONT` | font name (default `standard`) |
| `-w COLS` | width for wrapping and justification (default 80) |
| `-j auto\|left\|center\|right` | justification; `auto` = left for left-to-right, right for right-to-left. `right` pads `width - len(row) - 1` spaces, `center` pads `int((width - len(row)) / 2)` |
| `-D auto\|left-to-right\|right-to-left` | print direction; `auto` = the font header's print direction (0 → LTR, 1 → RTL; all standard fonts are 0) |
| `-r` | mirror: each output row is reversed and `/ \ ( ) [ ] { } < >` are swapped with their partners |
| `-F` | flip upside down: rows reversed in order and `A↔V M↔W b↔P ^↔v _→-` etc. translated |
| `-s` | strip leading and trailing empty lines (also `rstrip()`s the whole result, so trailing spaces of the last line go) |
| `-n` | one empty line before and after (ignored when `-s` is given) |
Order of post-processing in `main()`: render → `-r` → `-F` → `-s` (else `-n`) → write UTF-8 + `"\n"`. Text = positional args joined by `" "`.
Out of scope: `-c`, `-L`, `-l`, `-i`.

## Rare cases (where the port can go wrong; all are in `tests/gen.py`)
1. **First character of a line loses its leading blanks**: with an empty buffer, `smushAmount` lets the new character overlap its own leading spaces. Python then slices them off.
2. **Right-to-left swaps the roles in `smushRow`/`smushAmount`**: the *new* character becomes the "left" side and the buffer the "right" side. Universal overlap also flips which character wins.
3. **`-j right` vs right-to-left**: `-j right` pads `width - len - 1`; right-to-left only changes the default justification, it does not add padding itself.
4. **Wrapping**: break at the last blank (the blank itself is dropped, `prevCharWidth` is reset to 0 and then set again); with no blank, break before the current character. `>=` means `width - 1` columns at most.
5. **Width equal to a character's width makes the original loop forever** (empty lines pile up); a character wider than `-w` raises `CharNotPrinted`. The test generator drops both kinds of case.
6. **Characters the font lacks are skipped silently**, including `é` and `€` (most fonts only define ASCII plus `ÄÖÜäöüß`); a tab is skipped too.
7. **Double spaces and leading/trailing spaces** are real blank characters and take part in wrapping and justification.
8. **`-s` uses `rstrip()`** on the whole result: trailing spaces on the last row vanish. `-n` then adds back `\n` on both sides.
9. **Hardblanks** are replaced by spaces only at the very end, after justification, so they count for `len(row)`.
10. **Mixed row lengths / empty first row**: the last buffer is only emitted when `buffer[0] != ''`.
11. **Non-UTF-8 fonts** (`bubble`, `term`, `digital`) and `\u0085 \u2028 \u2029` inside fonts are handled by pyfiglet before parsing; the Go port repeats that.

# NOTES — pyfiglet (source for the figlet-go Harbor task)

## Source

| Field   | Value |
|---------|-------|
| URL     | https://github.com/pwaller/pyfiglet |
| Commit  | `255827efbc09d662875dad50fa0edc1e1263bf45` |
| Date    | `2026-08-02T22:51:18+01:00` (`git log -1 --format='%H %cI'`) |
| Licence | **MIT** (`LICENSE`: "The MIT License (MIT)", © 2007-2018 Christopher Jones, Stefano Rivera, Peter Waller and contributors) |

Everything that matters lives in `pyfiglet/__init__.py` (1048 lines): `FigletFont` (font
parser), `FigletBuilder` (layout + smushing), `FigletString` (reverse / flip / strip),
`Figlet` (direction + justify defaults) and `main()` (the CLI, built on `optparse`).

## Why `python -m pyfiglet hello` fails straight after cloning

The code loads fonts with `importlib.resources.files('pyfiglet.fonts')`, but the repository
has no `pyfiglet/fonts/` folder. Fonts ship in `pyfiglet/fonts-standard/` (MIT-compatible)
and `pyfiglet/fonts-contrib/` (many authors). Result:

```
ModuleNotFoundError: No module named 'pyfiglet.fonts'
```

The `Makefile` targets `clean` + `minimal` fix it:

```make
clean:   rm -rf pyfiglet/fonts/ && mkdir pyfiglet/fonts/
minimal: cp pyfiglet/fonts-standard/* pyfiglet/fonts
```

So the setup step is:

```
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
python -m pyfiglet -f slant hello      # now works
```

Note: `fonts-standard/__init__.py` is copied too (it is what makes `pyfiglet.fonts` a
package), so `pyfiglet/fonts/` contains a `.py` file that a port must delete.
Only `fonts-standard/` is used (72 files, 70 fonts). `fonts-contrib/` and `test-fonts/` are
removed in the Docker image.

## Font header → layout mode

Header: `flf2a<hardblank> height baseline maxlen oldLayout commentLines [printDirection [fullLayout]]`.
If `fullLayout` is missing: `oldLayout == 0 → 64` (kern), `oldLayout < 0 → 0` (full width),
else `(oldLayout & 31) | 128`.

| Bit | Name | Meaning |
|---|---|---|
| 1 | SM_EQUAL | rule 1 |
| 2 | SM_LOWLINE | rule 2 |
| 4 | SM_HIERARCHY | rule 3 |
| 8 | SM_PAIR | rule 4 |
| 16 | SM_BIGX | rule 5 |
| 32 | SM_HARDBLANK | rule 6 |
| 64 | SM_KERN | kerning (move letters together until they touch) |
| 128 | SM_SMUSH | smushing on (overlap one column using the rules) |

If neither 64 nor 128 is set → **full width** (no overlap at all).
If 128 is set but none of 1–32 → **universal smushing** (the later letter wins; hardblanks lose).

## The six smushing rules + kerning (verified by calling `FigletBuilder.smushChars`)

Applied in this order; first match wins. Before any rule: a space always loses
(`' '`+`x`→`x`), and **nothing smushes if either letter is less than 2 columns wide**.
A hardblank (`$`) never smushes with a visible character.

| # | Rule | Example (left + right → result) |
|---|---|---|
| 1 | Equal character | `\|` + `\|` → `\|`; `_` + `_` → `_` (not hardblanks: `$`+`$` → no smush) |
| 2 | Underscore (lowline) | `_` + `/` → `/`; `/` + `_` → `/` (`_` is replaced by any of `\|/\[]{}()<>`) |
| 3 | Hierarchy | classes `\|` < `/\` < `[]` < `{}` < `()` < `<>`; the higher class wins: `\|` + `/` → `/`, `]` + `(` → `(`, `{` + `\|` → `{` |
| 4 | Opposite pair | `[` + `]` → `\|`, `)` + `(` → `\|`, `{`+`}` → `\|` (either order) |
| 5 | Big X | `/` + `\` → **`\|`**, `\` + `/` → `Y`, `>` + `<` → `X` (`<` + `>` → no smush) |
| 6 | Hardblank | `$` + `$` → `$` |
| — | Kerning (bit 64 only) | letters move left until they touch, never overlap: `\|` + `\|` → no smush |
| — | Universal (128 only) | `a` + `b` → `b`; `$` + `b` → `b`; in right-to-left the *left* letter wins |

Real output with `standard` (rules 1–4 on): `lll` → `|_|_|_|` (equal rule merges the bars),
`[]` → `| _|_ |` (pair rule turns `][`-edges into `|`).

## Which test fonts use which rules

| Font | Layout | Rules | Default direction |
|---|---|---|---|
| standard, slant, small, big | 24463 / 18319 / 22415 / 24463 | smush 1,2,3,4 | LTR |
| mini, shadow | 1920 / 4992 | universal smushing | LTR |
| banner, script | 64 / 3904 | kerning only | LTR |
| ivrit (added) | 16271 | smush 1,2,3,4 | **RTL** (header printDirection = 1) |
| term (added) | 0 | full width | LTR |

**No standard font sets bit 16 or 32**, so rules 5 (Big X) and 6 (hardblank) cannot be
reached from the command line with the shipped fonts. The exam cannot check them; the port
still implements them.

## The eight options (`main()`)

| Option | Effect |
|---|---|
| `-f FONT` | font name (default `standard`); looks for `FONT.tlf` then `FONT.flf` |
| `-w COLS` | width used for wrapping and justification (default 80) |
| `-j auto\|left\|center\|right` | justification. `auto` → `left` for LTR, `right` for RTL |
| `-D auto\|left-to-right\|right-to-left` | direction. `auto` → the font's header (`ivrit` is RTL) |
| `-r` | reverse: mirror each line, swapping `( ) / \ < > [ ] { }` |
| `-F` | flip: turn upside down, rows reversed, swapping `/ \`, `A V`, `M W`, `P b`, `_ -`, `^ v`... |
| `-s` | strip leading empty lines and all trailing whitespace |
| `-n` | like `-s`, plus exactly one `\n` before and after (ignored when `-s` is also given) |

Order in `main()`: render → `-r` → `-F` → `-s` (else `-n`). Output is the result plus one
extra `\n`. All remaining arguments are joined with single spaces into the text.

## Rare cases (where a first reading goes wrong)

1. **Big X is `|`, not `X`.** `/`+`\` → `|`; only `>`+`<` gives `X`. (The practice page says
   "`/` and `\` meet as an X", which is wrong for pyfiglet.)
2. **Right justify uses `width - len - 1`**, centre uses `(width - len) // 2`. So
   `-j right -w 40` leaves the last column empty. `-j center` and `-j right` use the `-w`
   value, not a fixed 80.
3. **Right-to-left swaps left and right** in both the overlap calculation and the merge,
   then prepends the new letter. Universal smushing then keeps the *left* letter.
4. **Letters narrower than 2 columns never smush** (e.g. every letter of `term`, which is 1 column wide).
5. **The first letter of a line loses its leading blank columns** (the overlap with an
   empty buffer is the number of blank columns common to every row).
6. **Wrapping** breaks at the last space; the space itself is dropped. If there is no
   space, the line breaks before the letter that does not fit. A letter wider than `-w`
   makes pyfiglet crash (`CharNotPrinted`); those inputs are not used.
7. **Characters the font lacks are skipped silently**, including tabs. None of the test
   fonts has `é` or `ñ` (they vanish). All have German `ÄÖÜäöüß`. Extra `0x..` code points:
   `standard`/`term` Latin Extended-A (`Ā ā Ć č ...`), `big` Greek, `banner` Cyrillic,
   `ivrit` Hebrew. Only `0x`-prefixed code points are loaded; decimal ones are ignored.
8. **Line width is counted from row 0 only** when deciding to wrap.
9. **`-s` removes trailing spaces on the last line too** (`rstrip()` on the whole string).
10. **`-r` and `-F` only translate characters below U+0100**; others pass through.

# pyfiglet notes

Notes on the pyfiglet snapshot this task is built from. They cover what an agent has to understand to port it:
the licence, why it doesn't run straight after cloning, the smushing rules, the eight graded options, and the
edge cases that are easy to get wrong.

| | |
|---|---|
| **Repository** | https://github.com/pwaller/pyfiglet |
| **Commit** | `255827efbc09d662875dad50fa0edc1e1263bf45` (2026-08-02) |
| **Version** | 1.0.4 |
| **Main source** | `pyfiglet/__init__.py`, mostly `FigletFont`, `FigletBuilder` and `FigletString` |

## Contents

1. [Licence](#1-licence)
2. [Font setup](#2-font-setup)
3. [How a font file is read](#3-how-a-font-file-is-read)
4. [Smushing rules](#4-smushing-rules)
5. [Command-line options](#5-command-line-options)
6. [Edge cases](#6-edge-cases)

## 1. Licence

**MIT.** Copyright © 2007-2018 Christopher Jones, Stefano Rivera, Peter Waller and other contributors.
It appears in three places:
- the `LICENSE` file;
- the `__copyright__` string in `pyfiglet/__init__.py`;
- `license='MIT'` and the `License :: OSI Approved :: MIT License` classifier in `setup.py`.

## 2. Font setup

**The problem.** Straight after cloning, `python -m pyfiglet hello` fails with:

```
ModuleNotFoundError: No module named 'pyfiglet.fonts'
```

pyfiglet loads fonts from the `pyfiglet.fonts` package (`importlib.resources.files('pyfiglet.fonts')` in
`FigletFont.preloadFont`), and that folder is **not in git**. The repository only has `pyfiglet/fonts-standard/`
and `pyfiglet/fonts-contrib/`.

**The fix.** It's the `cp pyfiglet/fonts-standard/* pyfiglet/fonts` line in **`make minimal`**. `make full` runs it
too, and also copies `fonts-contrib/*`. Both targets run `make clean` first, which recreates an empty
`pyfiglet/fonts/`.

| Step | Result |
|---|---|
| Nothing done | `ModuleNotFoundError: No module named 'pyfiglet.fonts'` |
| `make clean` only (empty `fonts/`) | `pyfiglet error: requested font 'standard' not found.` |
| Fonts copied in | Works |

The setup this task uses (standard fonts only; `fonts-contrib/` comes from many authors and is left out):

```sh
mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
python -m pyfiglet -f slant "hello"
```

```
    __         ____
   / /_  ___  / / /___
  / __ \/ _ \/ / / __ \
 / / / /  __/ / / /_/ /
/_/ /_/\___/_/_/\____/
```

Other notes:
- `make minimal` and `make full` also run `python3 -m build`, which isn't needed just to run the module.
- `setup.py` makes the same copy if `pyfiglet/fonts` is missing, so `pip install .` also fixes it.

## 3. How a font file is read

These details matter for a byte-exact port. `fonts-standard/` has 67 `.flf` fonts.

- **Line endings:** every font uses CRLF. pyfiglet splits lines with Python's `str.splitlines()`, which also
  breaks on `\v`, `\f`, `\x1c`–`\x1e`, `\x85`, ` ` and ` `.
- **Invalid UTF-8:** `bubble`, `digital` and `term` contain invalid UTF-8. pyfiglet decodes with
  `errors='replace'`, so each bad byte sequence becomes one `U+FFFD`. These three fonts also use `\x7f` as the
  hardblank instead of `$`.
- **Header:** the header is `flf2a<hardblank> height baseline maxlen oldlayout commentlines [direction] [fulllayout]`.
  Some fonts stop after six fields. When the full-layout field is missing, `loadFont` converts the old value:

  | Old layout | Becomes |
  |---|---|
  | `0` | kerning (64) |
  | negative | full width (0) |
  | positive | `(old & 31) \| 128` |

- **End marker:** each row ends in an end-marker character. The last non-space character of a character's first
  row decides which character it is, and one or two copies of it are stripped from every row.
- **Characters loaded:** ASCII 32–126 first, then the German letters `ÄÖÜäöüß` if the file continues, then any
  extra characters introduced by a `0x…` code line.

## 4. Smushing rules

The layout value (`smushMode`) is a bit mask. The constants are in `FigletBuilder.__init__`, and the logic is in
`smushChars` and `smushAmount`. In the examples, `left` + `right` are the two characters that meet, and `$` is the
hardblank.

**Checked before the numbered rules, in this order:**
1. If either character is a space, the other one is used.
2. If either glyph is less than 2 columns wide, nothing overlaps.
3. Without `SM_SMUSH` (128), only kerning happens.
4. If 128 is set but none of bits 1–32 are, **universal smushing** applies. The later character wins, except that a
   visible character beats a hardblank. Under right-to-left, "later" means the left one.
5. If exactly one side is a hardblank, rules 1–5 never smush it.

| # | Rule (bit) | What it does | Example |
|---|---|---|---|
| 1 | **Equal character** (`SM_EQUAL` = 1) | Two identical characters become one. Hardblanks don't count. | `\|` + `\|` → `\|` |
| 2 | **Underscore** (`SM_LOWLINE` = 2) | `_` is replaced by any of `\| / \ [ ] { } ( ) < >`. | `_` + `/` → `/` |
| 3 | **Hierarchy** (`SM_HIERARCHY` = 4) | Classes from low to high: `\|`, `/\`, `[]`, `{}`, `()`, `<>`. The higher class wins. | `\|` + `(` → `(` ; `/` + `]` → `]` |
| 4 | **Opposite pair** (`SM_PAIR` = 8) | Opposing brackets, braces or parentheses become a bar, in either order. | `[` + `]` → `\|` ; `)` + `(` → `\|` |
| 5 | **Big X** (`SM_BIGX` = 16) | Three fixed pairs. | `/` + `\` → `\|` ; `\` + `/` → `Y` ; `>` + `<` → `X` |
| 6 | **Hardblank** (`SM_HARDBLANK` = 32) | Two hardblanks become one, which prints as a space. | `$` + `$` → `$` |
| – | **Kerning** (`SM_KERN` = 64) | No overlap; glyphs move together until they touch. | `\|` + `\|` → `\|\|` instead of `\| \|` |

pyfiglet checks rule 6 **before** rule 1, then rules 2 and 3 together, then 4, then 5. The order matters when more
than one rule could apply.

**Verified.** Each example above was run through the real `FigletBuilder.smushChars` with only that rule's bit set
(plus 128), and every result matched. These were also checked:
- `<` + `>` → no smush;
- `$` + `$` with only rule 1 → no smush;
- universal smushing: `a` + `b` → `b`, and `$` + `a` → `a`;
- a glyph 1 column wide → no smush.

Kerning was checked by rendering `ll` with `smushMode = 64`: `| || |`. Full width (`smushMode = 0`) gave
`| | | |`.

## 5. Command-line options

| Option | Default | What it does |
|---|---|---|
| `-f FONT`, `--font` | `standard` | Font to use. Looked up as `FONT.tlf`, then `FONT.flf`: first in the package, then in `./` and the system font folders. An unknown font prints `pyfiglet error: requested font ... not found.` and exits 1. |
| `-w COLS`, `--width` | `80` | Output width, used for both wrapping and justification. Text wraps at the last space, or at the last character if there's no space. A glyph wider than `COLS` raises `CharNotPrinted`. |
| `-j SIDE`, `--justify` | `auto` | `left`, `center` or `right`, aligned within the `-w` width. `auto` means `right` for right-to-left text, otherwise `left`. |
| `-D DIR`, `--direction` | `auto` | `left-to-right` or `right-to-left`. `auto` reads the font header (1 = right-to-left; 0 or missing = left-to-right). |
| `-r`, `--reverse` | off | Mirror image. Each line is reversed and `/`↔`\`, `(`↔`)`, `<`↔`>`, `[`↔`]`, `{`↔`}` are swapped. |
| `-F`, `--flip` | off | Upside down. Line order is reversed and characters are mapped (`^`↔`v`, `/`↔`\`, `_`→`-`, `A`↔`V`, `M`↔`W`, …). Applied after `-r`. |
| `-s`, `--strip-surrounding-newlines` | off | Removes blank lines at the top and bottom, plus trailing whitespace at the very end. Blank lines inside the art stay. |
| `-n`, `--normalize-surrounding-newlines` | off | Like `-s`, then exactly one blank line above and below. If `-s` is also given, `-s` wins. |

Options are parsed with Python's `optparse`, so these also work: clustered flags (`-rF`), attached values (`-w40`),
long options with `=` or abbreviated (`--wid 40`), options after the text, and `--`. The output always ends with
one extra newline that `main()` adds.

## 6. Edge cases

Easy to get wrong, and all confirmed against the real pyfiglet:

- **Big X:** the comment on `SM_BIGX` says `/ + \ -> X`, but the code returns `|`, which matches the FIGlet spec.
  `<` + `>` does not smush; only `><` gives `X`.
- **Right justification:** pads with `width - len - 1` spaces, one fewer than you might expect. Center uses
  `(width - len) / 2`, rounded toward zero.
- **Last line:** at the end of the text, the last line is printed only if its **first row** is non-empty.
- **Smush width check:** the "narrower than 2" test looks at both the previous glyph and the current one, so
  a 1-column glyph never overlaps its neighbours.
- **Hangs:** if a glyph is exactly as wide as `-w`, pyfiglet loops forever. The test cases leave these inputs out.
- **Crashes:** if a glyph is wider than `-w`, pyfiglet raises `CharNotPrinted` and prints nothing. These are left
  out too, because errors aren't graded.
- **`make clean` alone** does not fix the missing-fonts error. The font files have to be copied in.

**Mistake in an earlier draft:** the first draft gave rule 6's bit as `16→32`. The correct value is `32`.

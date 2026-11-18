# pyfiglet notes

## 1. Repository

https://github.com/pwaller/pyfiglet

## 2. Commit

`255827efbc09d662875dad50fa0edc1e1263bf45`
("Return False from isValidFont for a missing font (#161)")

## 3. Commit date

`2026-08-02T22:51:18+01:00` (committer date).
The author date of the same commit is `2026-08-03T03:21:18+05:30`.

## 4. License

MIT License (`LICENSE`: "The MIT License (MIT)", Copyright © 2007-2018
Christopher Jones, Stefano Rivera, Peter Waller and contributors).

## 5. Why `python -m pyfiglet hello` fails on a fresh clone

Fonts are only ever loaded from the `pyfiglet.fonts` package (plus the
fallback directories `./`, `/usr/local/share/pyfiglet/`,
`/usr/local/share/figlet`, `/usr/share/figlet`). The repository does not
contain a `pyfiglet/fonts/` directory: the fonts are kept in
`pyfiglet/fonts-standard/` and `pyfiglet/fonts-contrib/`, and `pyfiglet/fonts/`
is only created at build time.

The call chain is `main()` → `Figlet.__init__` → `setFont()` →
`FigletFont.__init__` → `preloadFont()`, whose first lookup is:

```python
path = importlib.resources.files('pyfiglet.fonts').joinpath(fn)
```

Because `pyfiglet/fonts` does not exist, this raises before any fallback
directory is searched:

```
ModuleNotFoundError: No module named 'pyfiglet.fonts'
```

This is not a `FontNotFound`, so `main()`'s
`except FontNotFound` handler does not catch it, and you get a raw traceback.

**What fixes it:** this Makefile step, which both `make minimal` and
`make full` run:

```make
cp pyfiglet/fonts-standard/* pyfiglet/fonts
```

It runs after the `clean` prerequisite has created the directory
(`rm -rf pyfiglet/fonts/` then `mkdir pyfiglet/fonts/`). `make full`
also runs `cp pyfiglet/fonts-contrib/* pyfiglet/fonts`. With the standard
fonts in place, `pyfiglet.fonts` resolves (as a namespace package) and
`standard.flf` (the `DEFAULT_FONT`) is found.

`setup.py` does the same copy as a side effect when `pyfiglet/fonts` is
missing (`shutil.copytree(repo_src, pkg_src)`), so running any `setup.py`
or `pip install` command also creates it.

## 6. Kerning and the six smushing rules

### How the layout value is read

Each font header gives `oldLayout` and an optional `fullLayout`
(`FigletFont.loadFont`). If `fullLayout` is missing, it is derived from
`oldLayout`: `0` → `64` (kerning), negative → `0` (full width), otherwise
`(oldLayout & 31) | 128`. The result is stored as `Font.smushMode`.
`FigletBuilder` tests these bits (constants copied from figlet 2.2.2):

| Bit | Constant | Meaning |
|---|---|---|
| 1 | `SM_EQUAL` | Rule 1: equal character |
| 2 | `SM_LOWLINE` | Rule 2: underscore |
| 4 | `SM_HIERARCHY` | Rule 3: hierarchy |
| 8 | `SM_PAIR` | Rule 4: opposite pair |
| 16 | `SM_BIGX` | Rule 5: big X |
| 32 | `SM_HARDBLANK` | Rule 6: hardblank |
| 64 | `SM_KERN` | Kerning (fitting) |
| 128 | `SM_SMUSH` | Smushing |

For example, `standard.flf` has `smushMode` 24463: the low byte is 143
(128 smush + rules 1–4), and the higher bits are vertical-layout bits that
pyfiglet ignores. Its hardblank is `$`.

### How the engine applies them

- `smushAmount()` works out, row by row, how far the next character can
  slide left. This is the gap between the last non-space character in the
  buffer and the first non-space character of the new glyph. It adds one
  more column when the left edge is empty or `smushChars()` accepts the
  pair. The minimum over all rows wins. If neither `SM_SMUSH` nor
  `SM_KERN` is set, the amount is 0 (full width).
- `smushChars(left, right)` decides which character appears where two
  characters overlap. It returns `None` when they can't be smushed, and
  then only kerning happens. Before any rule is checked:
  - a space always gives way to the other character;
  - nothing is smushed if the previous or current glyph is narrower than
    2 columns;
  - without `SM_SMUSH`, nothing is smushed (kerning only).
- If `SM_SMUSH` is set but none of rules 1–6 (`smushMode & 63 == 0`) are,
  **universal smushing** applies. A hardblank gives way to the visible
  character; otherwise the later character wins (`right`, or `left` when
  the direction is right-to-left).
- For controlled smushing, a hardblank can only smush with another
  hardblank (rule 6). Hardblanks are turned into spaces at output time
  (`replaceHardblanks`).

### Examples

All of the outputs below were produced by running this repository's code
on throwaway one-row test fonts. Each font has two glyphs, `A` and `B`,
and the text rendered is `"AB"`. The hardblank is `$`. The middle column
is the font's `fullLayout` value.

**Full width, kerning and smushing compared**

| Mode | fullLayout | A | B | Output |
|---|---|---|---|---|
| Full width | 0 | `x\| ` | ` \|y` | `x\|  \|y` |
| Kerning only | 64 | `x\| ` | ` \|y` | `x\|\|y` |
| Smush, rule 1 | 129 | `x\| ` | ` \|y` | `x\|y` |

Kerning (fitting) slides the characters together until they touch, but
never overlaps them. Smushing moves them one more column so the edge
characters share a cell.

**Rule 1 – Equal character (1).** Two identical characters merge into one.
Hardblanks are excluded.

| fullLayout | A | B | Output |
|---|---|---|---|
| 129 | `x\| ` | ` \|y` | `x\|y` |
| 129 | `x\|` | `/y` | `x\|/y` (no match, so kerning only) |

**Rule 2 – Underscore (2).** `_` is replaced by any of `|/\[]{}()<>`.

| fullLayout | A | B | Output |
|---|---|---|---|
| 130 | `x_` | `\|y` | `x\|y` |

Checked directly with `smushChars`: `/` + `_` → `/`, `_` + `<` → `<`,
`_` + `-` → no smush.

**Rule 3 – Hierarchy (4).** The classes, in order, are `|`, `/\`, `[]`,
`{}`, `()`, `<>`. When the two characters are in different classes, the
one from the later class wins.

| fullLayout | A | B | Output |
|---|---|---|---|
| 132 | `x\|` | `/y` | `x/y` |
| 132 | `x{` | `<y` | `x<y` |

Checked directly: `[` + `)` → `)`, `(` + `[` → `(`, `|` + `|` → no smush
(same class).

**Rule 4 – Opposite pair (8).** `[]`, `][`, `{}`, `}{`, `()`, `)(` become
`|`.

| fullLayout | A | B | Output |
|---|---|---|---|
| 136 | `x[` | `]y` | `x\|y` |
| 136 | `x)` | `(y` | `x\|y` |

Checked directly: `<` + `>` → no smush.

**Rule 5 – Big X (16).** `/\` → `|`, `\/` → `Y`, `><` → `X`. `<>` is not
smushed.

| fullLayout | A | B | Output |
|---|---|---|---|
| 144 | `x/` | `\y` | `x\|y` |
| 144 | `x\` | `/y` | `xYy` |
| 144 | `x>` | `<y` | `xXy` |

**Rule 6 – Hardblank (32).** Two hardblanks merge into one hardblank,
which is printed as a space.

| fullLayout | A | B | Output |
|---|---|---|---|
| 160 | `x$` | `$y` | `x y` |
| 64 (kerning) | `x$` | `$y` | `x  y` |

Checked directly: `$` + `|` → no smush under rule 6.

**Universal smushing (128, no rule bits).** The later character overwrites
the earlier one, but a hardblank always gives way to a visible character.

| fullLayout | A | B | Output |
|---|---|---|---|
| 128 | `xa` | `by` | `xby` |
| 128 | `x$` | `by` | `xby` |

## 7. CLI options

These are defined in `main()` using `optparse`. All of the outputs below
come from running `python -m pyfiglet` with the standard fonts installed.

### `-f FONT`, `--font`

Chooses the font. The default is `standard`. The name is looked up as
`FONT.tlf`, then `FONT.flf` (see §5). If no font is found, the command
prints `pyfiglet error: requested font 'nosuchfont' not found.` and exits
with status 1.

```
$ pyfiglet -f slant Hi
    __  ___ 
   / / / (_)
  / /_/ / / 
 / __  / /  
/_/ /_/_/   
```

### `-w COLS`, `--width`

Sets the output width (default 80), used for both wrapping and
justification. When adding a character would make the line at least
`COLS` wide, the builder breaks the line at the last space (or the last
character, if the line has no space). A single character wider than
`COLS` raises `CharNotPrinted` (for example, `-w 5 Hi`).

```
$ pyfiglet -w 15 "Hi Hi"
 _   _ _ 
| | | (_)
| |_| | |
|  _  | |
|_| |_|_|

 _   _ _ 
| | | (_)
| |_| | |
|  _  | |
|_| |_|_|
```

### `-j SIDE`, `--justify`

Takes `auto`, `left`, `center` or `right`. With `auto`, the text is
left-aligned for left-to-right output and right-aligned for right-to-left
output. `right` pads each row to `width - len - 1`, and `center` adds
`(width - len) // 2` spaces of padding.

```
$ pyfiglet -w 30 -j right Hi
                     _   _ _ 
                    | | | (_)
                    | |_| | |
                    |  _  | |
                    |_| |_|_|
```

### `-D DIRECTION`, `--direction`

Takes `auto`, `left-to-right` or `right-to-left`. With `auto`, the
direction comes from the font header's `printDirection` field (1 means
right-to-left; anything else means left-to-right). Right-to-left places
each new character to the left of the previous ones. With `-j auto`, it
also right-justifies the output to the width (80 by default).

```
$ pyfiglet -D right-to-left Hi      (shown without the 70 leading spaces)
 _ _   _ 
(_) | | |
| | |_| |
| |  _  |
|_|_| |_|
```

### `-r`, `--reverse`

Mirrors the output horizontally (`FigletString.reverse`). Each row is
passed through `__reverse_map__`, which swaps `/`↔`\`, `(`↔`)`, `[`↔`]`,
`{`↔`}` and `<`↔`>`, and is then reversed.

```
$ pyfiglet -r Hi
 _ _   _ 
(_) | | |
| | |_| |
| |  _  |
|_|_| |_|
```

### `-F`, `--flip`

Flips the output upside down (`FigletString.flip`). The row order is
reversed and each row is passed through `__flip_map__`, which maps
`_`→`-`, `^`→`v`, `v`→`^`, `/`↔`\` and a few letters (`A`→`V`, `M`→`W`,
`w`→`m`, and so on).

```
$ pyfiglet -F Hi
         
|-| |-|-|
|  -  | |
| |-| | |
| | | (-)
 -   - - 
```

### `-s`, `--strip-surrounding-newlines`

Removes blank leading and trailing lines
(`strip_surrounding_newlines`). Blank lines inside the figure are kept.
If both `-s` and `-n` are given, `-s` wins, because `main()` checks it
first (`if`/`elif`).

```
$ pyfiglet -s Hi
 _   _ _ 
| | | (_)
| |_| | |
|  _  | |
|_| |_|_|
```

### `-n`, `--normalize-surrounding-newlines`

Strips the surrounding blank lines as `-s` does, then adds exactly one
empty line before and after the figure (`'\n' + stripped + '\n'`).

```
$ pyfiglet -n Hi

 _   _ _ 
| | | (_)
| |_| | |
|  _  | |
|_| |_|_|

```

When several of these options are combined, `main()` applies them in
this order: render, then `-r`, then `-F`, then `-s` or `-n`.

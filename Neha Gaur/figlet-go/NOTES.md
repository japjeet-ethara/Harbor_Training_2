## Repository Information
- Repository: https://github.com/pwaller/pyfiglet
- Commit SHA: 255827efbc09d662875dad50fa0edc1e1263bf45
- Commit Date: 2026-08-02T22:51:18+01:00
# pyfiglet notes

## 1. Licence name

The licence is the **MIT License** (also written “The MIT License (MIT)” in `LICENSE` and `setup.py`).

## 2. Why `python -m pyfiglet hello` fails right after cloning

The program always starts by loading a font. The default font name is `standard`.

`FigletFont.preloadFont` looks for that font inside the Python package `pyfiglet.fonts` (`standard.flf` or `standard.tlf`).

A fresh clone does not have a `pyfiglet/fonts` folder. The font files live in two other folders:

- `pyfiglet/fonts-standard` (includes `standard.flf` and an `__init__.py`)
- `pyfiglet/fonts-contrib`

So the import of `pyfiglet.fonts` fails immediately:

`ModuleNotFoundError: No module named 'pyfiglet.fonts'`

`setup.py` would copy `fonts-standard` into `pyfiglet/fonts` when the package is installed, but only if that folder is missing. Running `python -m pyfiglet` does not run `setup.py`, so the copy never happens.

## 3. Makefile step that fixes it

`make minimal` fixes it.

That target first runs `clean` (which creates an empty `pyfiglet/fonts` folder), then copies every file from `pyfiglet/fonts-standard` into `pyfiglet/fonts`. That copy includes `standard.flf` and `__init__.py`, so `pyfiglet.fonts` becomes a real package and the default font can be loaded.

`make full` also fixes it. It copies `fonts-standard` and then `fonts-contrib`. `make clean` alone does not fix it: it only makes an empty folder, with no font files.

## 4. Six smushing rules and kerning

Smushing is how two letters share a column when they meet. `FigletBuilder.smushChars` decides which single character is kept. The font header turns each rule on or off.

These examples are the two touching characters and the one character that replaces them. A rule only runs when the font has that rule switched on, and the letters are wide enough to overlap (at least 2 columns).

### Rule 1 — equal characters (`SM_EQUAL`, value 1)

If both sides are the same character, keep one of them. Hardblanks are not handled here.

Example: `|` and `|` become `|`

### Rule 2 — underscore (`SM_LOWLINE`, value 2)

An underscore loses to any of these characters: `| / \ [ ] { } ( ) < >`. The other character is kept.

Example: `_` and `|` become `|`

### Rule 3 — hierarchy (`SM_HIERARCHY`, value 4)

Characters are ranked. A higher class replaces a lower class:

1. `|` (lowest)
2. `/` and `\`
3. `[` and `]`
4. `{` and `}`
5. `(` and `)`
6. `<` and `>` (highest)

Example: `|` and `/` become `/`

### Rule 4 — opposite pair (`SM_PAIR`, value 8)

A matching pair of brackets becomes a vertical bar. The code checks `[]`, `{}`, and `()`.

Example: `[` and `]` become `|`

### Rule 5 — big X (`SM_BIGX`, value 16)

Three special pairs:

- `/` then `\` becomes `|`
- `\` then `/` becomes `Y`
- `>` then `<` becomes `X`

Example: `>` and `<` become `X`

The comment above the constant says `/` + `\` becomes `X`. The code does not do that. It returns `|`, `Y`, or `X` as listed above.

### Rule 6 — hardblank (`SM_HARDBLANK`, value 32)

A hardblank is a blank stored as a special character so it is not trimmed like a normal space. In the standard font that character is `$`. Two hardblanks become one hardblank. A hardblank next to any other character is not smushed. When the picture is finished, hardblanks are printed as normal spaces.

Example: `$` and `$` become `$` (shown later as a space)

### Kerning (`SM_KERN`, value 64)

Kerning slides letters together until their ink touches. Empty spaces between them are removed. The visible characters are not replaced by a new character.

Kerning is used when the font has the kern bit set. If the separate “smush” bit (`SM_SMUSH`, value 128) is off, `smushChars` refuses to overlap two real characters, so only the gaps close.

Example, one row of each letter:

- left: `## `
- right: ` ##`
- kerned: `####`

The two `#` marks sit next to each other. They do not become one `#`.

If neither the kern bit nor the smush bit is set, letters are placed side by side and nothing slides.

There is also a mode where the smush bit is on but all six rules are off. Then any two characters may overlap, and the character later in the text wins (or the earlier one, if the direction is right to left). A hardblank loses to a visible character. That is universal overlapping, not one of the six rules.

## 5. Command-line options

Defined in `main()` in `pyfiglet/__init__.py`.

### `-f` (`--font`)

Chooses the font name. The default is `standard`, which means the file `standard.flf`.

### `-w` (`--width`)

Sets the line width in columns. The default is 80. Lines wrap at the last space before this width. If there is no space to break on, the current letter starts a new line. If one letter is wider than this width, rendering stops with an error.

### `-j` (`--justify`)

Sets horizontal alignment. Choices are `auto`, `left`, `center`, and `right`. The default is `auto`.

- `auto` uses `left` for left-to-right text and `right` for right-to-left text.
- `left` adds no extra spaces.
- `center` puts spaces on the left so the line sits in the middle of the width.
- `right` puts spaces on the left so the line sits on the right. The code uses one fewer padding space than the full width.

### `-D` (`--direction`)

Sets the writing direction. Choices are `auto`, `left-to-right`, and `right-to-left`. The default is `auto`.

`auto` reads the font header. `0` means left to right. `1` means right to left. Any other value is treated as left to right. The standard font uses `0`.

Right-to-left swaps the two sides before smushing, so overlap follows the order of the text.

### `-r` (`--reverse`)

Prints a mirror image. Each row is reversed, and paired characters are swapped (`/` with `\`, `(` with `)`, `[` with `]`, and so on).

### `-F` (`--flip`)

Turns the picture upside down. The rows are reversed from bottom to top, and paired characters are swapped (`^` with `v`, `/` with `\`, and so on). If both `-r` and `-F` are used, reverse happens first, then flip.

### `-s` (`--strip-surrounding-newlines`)

Removes empty lines at the start and the end. Blank lines in the middle of the picture stay. If `-s` and `-n` are both set, `-s` is the one that runs.

### `-n` (`--normalize-surrounding-newlines`)

Does the same stripping as `-s`, then puts exactly one empty line before the picture and one empty line after it.

# pyfiglet notes

- URL: https://github.com/pwaller/pyfiglet
- Commit: 255827efbc09d662875dad50fa0edc1e1263bf45
- Commit date: 2026-08-02T22:51:18+01:00

Sources: `pyfiglet/LICENSE`, `pyfiglet/setup.py`, `pyfiglet/Makefile`, `pyfiglet/pyfiglet/__init__.py` (`FigletFont.loadFont`, `FigletBuilder`, `main`), and the first line of each `.flf` under `pyfiglet/pyfiglet/fonts-standard/`.

## 1. Licence

The licence name is **The MIT License (MIT)**.

`pyfiglet/LICENSE` opens with that name. `pyfiglet/setup.py` sets `license='MIT'` and the classifier `License :: OSI Approved :: MIT License`. The same MIT notice is copied into the module docstring of `pyfiglet/pyfiglet/__init__.py`.

## 2. Why `python -m pyfiglet hello` fails right after cloning

Run from the repo root, the command reaches `main()`, builds a `Figlet` with the default font `standard`, and dies in `FigletFont.preloadFont`:

```text
ModuleNotFoundError: No module named 'pyfiglet.fonts'
```

`preloadFont` loads `standard.tlf` or `standard.flf` from the importable package `pyfiglet.fonts`:

```python
path = importlib.resources.files('pyfiglet.fonts').joinpath(fn)
```

A fresh clone has no `pyfiglet/pyfiglet/fonts/` directory. The `.flf` files, and the empty `fonts` package `__init__.py`, live in `pyfiglet/pyfiglet/fonts-standard/` (and extra fonts in `fonts-contrib/`). Nothing copies them until a build step runs. `setup.py` does copy `fonts-standard` onto `pyfiglet/fonts` if that directory is missing, but only when `setup.py` itself is executed, which `python -m pyfiglet` does not do.

The Makefile step that fixes it is **`make minimal`**. That target depends on `clean` (which deletes `pyfiglet/fonts/` and recreates an empty directory) and then copies the standard fonts into the package the loader imports:

```make
minimal:    clean
	cp pyfiglet/fonts-standard/* pyfiglet/fonts
	python3 -m build
```

That copy includes `fonts-standard/__init__.py` and `standard.flf`, so `pyfiglet.fonts` imports and the default font is found.

`make full` also fixes it: it performs the same `fonts-standard` copy and then adds `fonts-contrib`. `make clean` does not. It leaves `pyfiglet/fonts/` empty, with no `__init__.py` and no `standard.flf`.

## 3. Smushing rules

`FigletBuilder` stores the font's layout integer on `font.smushMode` and tests these bits:

```python
self.SM_EQUAL = 1       # smush equal chars (not hardblanks)
self.SM_LOWLINE = 2     # smush _ with any char in hierarchy
self.SM_HIERARCHY = 4   # hierarchy: |, /\, [], {}, (), <>
self.SM_PAIR = 8        # hierarchy: [ + ] -> |, { + } -> |, ( + ) -> |
self.SM_BIGX = 16       # / + \ -> X, > + < -> X
self.SM_HARDBLANK = 32  # hardblank + hardblank -> hardblank
self.SM_KERN = 64
self.SM_SMUSH = 128
```

`smushAmount` moves the next glyph leftward only when kerning or smushing is enabled (`smushMode & (SM_SMUSH | SM_KERN)`). `smushChars` decides what happens to the two characters that then occupy one cell. The hardblank below is the font's hardblank character; in these eight fonts it is `$`.

In `smushChars`, if either character is a space the other one is used (`' '` + `X` → `X`, and `X` + `' '` → `X`). That check is first. After it, nothing overlaps when `prevCharWidth` or `curCharWidth` is less than 2: the function returns without a replacement, so a glyph of width 0 or 1 is not smushed. From `pyfiglet/pyfiglet/__init__.py`:

```787:795:pyfiglet/pyfiglet/__init__.py
        if left == ' ':
            return right
        if right == ' ':
            return left

        # Disallows overlapping if previous or current char has a width of 1 or
        # zero
        if (self.prevCharWidth < 2) or (self.curCharWidth < 2):
            return
```

### Rule 1 — equal character (`SM_EQUAL`, bit 1)

Two identical non-hardblank characters become one copy of that character.

`A` + `A` → `A`

```python
if self.font.smushMode & self.SM_EQUAL:
    if left == right:
        return left
```

### Rule 2 — underscore (`SM_LOWLINE`, bit 2)

`_` smushes with any of `|/\[]{}()<>`. The other character replaces the underscore.

`_` + `|` → `|`

```python
if self.font.smushMode & self.SM_LOWLINE:
    smushes += (('_', r'|/\[]{}()<>'),)

for a, b in smushes:
    if left in a and right in b:
        return right
    if right in a and left in b:
        return left
```

### Rule 3 — hierarchy (`SM_HIERARCHY`, bit 4)

Later classes replace earlier ones. From low to high: `|`, then `/\`, then `[]`, then `{}`, then `()`, then `<>`.

`|` + `/` → `/`

```python
if self.font.smushMode & self.SM_HIERARCHY:
    smushes += (
        ('|', r'/\[]{}()<>'),
        (r'\/', '[]{}()<>'),
        ('[]', '{}()<>'),
        ('{}', '()<>'),
        ('()', '<>'),
    )
```

The same loop as rule 2 returns the character from the higher class.

### Rule 4 — opposite pair (`SM_PAIR`, bit 8)

`[]`, `{}`, or `()` in either order become `|`.

`[` + `]` → `|`

```python
if self.font.smushMode & self.SM_PAIR:
    for pair in [left+right, right+left]:
        if pair in ['[]', '{}', '()']:
            return '|'
```

### Rule 5 — big X (`SM_BIGX`, bit 16)

The constant's comment says `/` + `\` becomes `X`. The code does not do that. `/` followed by `\` becomes `|`, `\` followed by `/` becomes `Y`, and `>` followed by `<` becomes `X`. The opposite order `<` + `>` is not smushed.

`/` + `\` → `|`
`\` + `/` → `Y`
`>` + `<` → `X`

```python
if self.font.smushMode & self.SM_BIGX:
    if (left == '/') and (right == '\\'):
        return '|'
    if (right == '/') and (left == '\\'):
        return 'Y'
    if (left == '>') and (right == '<'):
        return 'X'
```

### Rule 6 — hardblank (`SM_HARDBLANK`, bit 32)

Outside universal smushing, if either side is the hardblank nothing overlaps, unless both sides are hardblanks and rule 32 is set. Both hardblanks then become the left one (`$` + `$` → `$`). One hardblank next to any other character returns no overlap even when rule 32 is set. Universal smushing is the earlier branch and is not this rule: there a hardblank loses to a visible character. From `pyfiglet/pyfiglet/__init__.py`:

```817:824:pyfiglet/pyfiglet/__init__.py
        if self.font.smushMode & self.SM_HARDBLANK:
            if (left == self.font.hardBlank
                    and right == self.font.hardBlank):
                return left

        if (left == self.font.hardBlank
                or right == self.font.hardBlank):
            return
```

### Kerning (`SM_KERN`, bit 64)

Kerning is the fitting mode used when smushing is off. Glyphs slide together until they touch, including by absorbing spaces, but two visible characters are not replaced by one. `smushChars` returns `None`, so both characters stay.

`A` + `B` → `AB` (adjacent, not overlapped)

```python
if (self.font.smushMode & (self.SM_SMUSH | self.SM_KERN)) == 0:
    return 0

# kerning only
if (self.font.smushMode & self.SM_SMUSH) == 0:
    return
```

The first snippet is the start of `smushAmount` (no kern and no smush means the glyphs are not moved together at all). The second is in `smushChars` (kerning without `SM_SMUSH` refuses the overlap).

### Universal smushing

Used when `SM_SMUSH` (bit 128) is set and none of the six rule bits are set (`smushMode & 63 == 0`). Any two visible characters overlap. The later character in the user's text wins: the right one for left-to-right, the left one for right-to-left. A hardblank loses to a visible character.

`A` + `B` → `B` (left-to-right)

```python
# smushing by universal overlapping
if (self.font.smushMode & 63) == 0:
    # Ensure preference to visiable characters.
    if left == self.font.hardBlank:
        return right
    if right == self.font.hardBlank:
        return left

    # Ensures that the dominant (foreground)
    # fig-character for overlapping is the latter in the
    # user's text, not necessarily the rightmost character.
    if self.direction == 'right-to-left':
        return left
    else:
        return right
```

`SM_SMUSH` has to be on to reach this block. With `SM_SMUSH` off, the kerning-only return above already exited.

## 4. Which rules each font uses

`loadFont` takes the layout from the first line of the `.flf`. After the magic number (`flf2a` plus the hardblank character) the fields are height, baseline, max length, old layout, comment lines, print direction, **full layout**, codetag count. The full layout is `header[7]` and is stored as `smushMode`. All eight fonts have that field, so the old-layout fallback is not used.

Horizontal bits this renderer actually tests are 1, 2, 4, 8, 16, 32, 64, and 128. Higher bits in the same integer are vertical-layout flags and do not select any rule in `smushChars`.

| Font | First line | Full layout | Rules this code applies |
| --- | --- | --- | --- |
| standard | `flf2a$ 6 5 16 15 46 0 24463 229` | 24463 | equal, underscore, hierarchy, opposite pair (bits 1+2+4+8, with smush bit 128). Not big X, hardblank, kerning, or universal. |
| slant | `flf2a$ 6 5 16 15 41 0 18319 96` | 18319 | same four: equal, underscore, hierarchy, opposite pair, with smush on. |
| small | `flf2a$ 5 4 13 15 41 0 22415 96` | 22415 | same four: equal, underscore, hierarchy, opposite pair, with smush on. |
| big | `flf2a$ 8 6 59 15 41 0 24463 153` | 24463 | same as standard. |
| mini | `flf2a$ 4 3 10 0 41 0 1920 96` | 1920 | universal smushing. Bit 128 is set and bits 1–32 are clear. |
| banner | `flf2a$ 8 7 54 0 43 0 64 185` | 64 | kerning only (bit 64). Smush is off, so none of the six rules and not universal. |
| shadow | `flf2a$ 5 4 16 0 41 0 4992 96` | 4992 | universal smushing, same horizontal pattern as mini (bit 128 only). |
| script | `flf2a$ 7 5 16 0 41 0 3904 96` | 3904 | kerning only (bit 64). Same horizontal pattern as banner. |

`standard` and `big` share 24463. `slant` (18319) and `small` (22415) differ from those two only in vertical-layout bits; the low byte is 143 (`128+8+4+2+1`) for all four.

## 5. Options, and the order of `-r`, `-F`, `-s`, `-n`

| Option | Long form | What it does |
| --- | --- | --- |
| `-f` | `--font` | Font name to render with. Default `standard`. |
| `-w` | `--width` | Terminal width in columns, used for wrapping and for center/right justification. Default 80. |
| `-j` | `--justify` | Horizontal justification: `auto`, `left`, `center`, or `right`. Default `auto`, which follows the print direction (left for left-to-right, right for right-to-left). |
| `-D` | `--direction` | Direction of the text: `auto`, `left-to-right`, or `right-to-left`. Default `auto`, which reads the font's print-direction field (0 or anything other than 1 means left-to-right; 1 means right-to-left). |
| `-r` | `--reverse` | Mirror each row. `reverse` swaps characters with `__reverse_map__` (for example `/` and `\`, `(` and `)`) and then reverses the row. |
| `-F` | `--flip` | Flip the rendered text over. `flip` reverses the order of the rows and swaps characters with `__flip_map__`. |
| `-s` | `--strip-surrounding-newlines` | Drop empty leading lines, then `rstrip()` the end: trailing newlines and the spaces at the end of the last row go. Interior empty rows of a glyph are kept. |
| `-n` | `--normalize-surrounding-newlines` | Same trimming as `-s`, then returns `'\n' + text + '\n'`. `main` prints one more newline, so the output has exactly one empty line before the drawing and one empty line after it. |

`-f`, `-D`, `-j`, and `-w` are arguments to the `Figlet` constructor, so they affect rendering itself. `-r`, `-F`, `-s`, and `-n` are applied afterwards, in this order:

1. `-r` (`r = r.reverse()`)
2. `-F` (`r = r.flip()`)
3. `-s`, and only if `-s` is not set, `-n`

```python
r = f.renderText(text)
if opts.reverse:
    r = r.reverse()
if opts.flip:
    r = r.flip()
if opts.strip_surrounding_newlines:
    r = r.strip_surrounding_newlines()
elif opts.normalize_surrounding_newlines:
    r = r.normalize_surrounding_newlines()
```

`-s` and `-n` are mutually exclusive in `main`. If both are passed, `-s` runs and `-n` does not.

`-r` calls `FigletString.reverse`. That swaps characters with `__reverse_map__` and then reverses each row. The map's printable punctuation line puts `)` in the slot for `(`, `(` in the slot for `)`, and `\` in the slot for `/`; the later line puts `/` in the slot for `\`. `-F` calls `FigletString.flip`, which reverses the order of the rows and then swaps characters with `__flip_map__`. Both maps are 256 characters, one per code point from 0 through 255, so they only cover characters below 256. From `pyfiglet/pyfiglet/__init__.py`:

```399:399:pyfiglet/pyfiglet/__init__.py
        ' !"#$%&\')(*+,-.\\'
```

```433:444:pyfiglet/pyfiglet/__init__.py
    def reverse(self) -> FigletString:
        out: list[str] = []
        for row in self.splitlines():
            out.append(row.translate(self.__reverse_map__)[::-1])

        return self.newFromList(out)

    def flip(self) -> FigletString:
        out: list[str] = []
        for row in self.splitlines()[::-1]:
            out.append(row.translate(self.__flip_map__))

        return self.newFromList(out)
```

`__reverse_map__` is lines 396–412 and `__flip_map__` is lines 415–431. Each string is 16 lines of 16 characters.

## 6. Extra characters after the German letters

`loadFont` reads required glyphs in order: ASCII 32–126, then the seven German letters `ÄÖÜäöüß`. After that it only loads code-tagged characters whose tag starts with `0x` or `0X`. Decimal tags are not loaded.

```python
# Load German Umlaute - the follow directly after standard character 127
if data:
    for i in 'ÄÖÜäöüß':
        width, letter = __char(data)
        if ''.join(letter) != '':
            self.chars[ord(i)] = letter
            self.width[ord(i)] = width

# Load ASCII extended character set
while data:
    line = data.pop(0).strip()
    i = line.split(' ', 1)[0]
    if (i == ''):
        continue
    hex_match = re.search('^0x', i, re.IGNORECASE)
    if hex_match is not None:
        i = int(i, 16)
        width, letter = __char(data)
        if ''.join(letter) != '':
            self.chars[i] = letter
            self.width[i] = width
```

A line such as `160  NO-BREAK SPACE` in `standard.flf` is popped and then ignored, because `160` does not match `^0x`. The following glyph lines are not passed to `__char`. A line such as `0x0302 circumflex` is parsed with `int(i, 16)` and the next `height` lines become that character. So the extra set is **hex codes only**, not decimal codes and not both.

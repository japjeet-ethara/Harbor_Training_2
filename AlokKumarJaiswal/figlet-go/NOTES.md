# NOTES: pinned pyfiglet source (Step 1)

These notes describe the checkout at this repository root. They were produced by reading the source and by checking `git log` / filesystem presence. Pyfiglet was not imported, installed, or executed. Makefile targets were not run. Character-pair results below are traces of `smushChars`, not rendered font output.

Claim labels used below:

- **Source:** the behavior is written in the cited file.
- **Inference:** the conclusion follows from combining those lines, but this step did not execute it.
- **Not confirmed:** a runtime result, an exception class, or a whole-tree fact this step did not establish.

## 1. Source

- URL: https://github.com/pwaller/pyfiglet
- SHA: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: `2026-08-02T22:51:18+01:00`

**Source:** `git rev-parse HEAD` and `git log -1 --format=%cI` at this repository root both returned that SHA and that ISO timestamp. The commit subject is `Return False from isValidFont for a missing font (#161)`. `setup.py:56` records the same GitHub URL.

**Not confirmed:** this step did not fetch the commit from GitHub again. The URL match is the local `setup.py` field plus the requested pin, not a fresh remote comparison.

## 2. Licence

**Source:** `LICENSE:3` names the licence exactly:

```text
The MIT License (MIT)
```

The copyright block under that name is `Copyright © 2007-2018` (`LICENSE:5`). The same name and the same 2007–2018 copyright are repeated in the module docstring (`pyfiglet/__init__.py:25-26`).

`setup.py:28` and `setup.py:33` use the short name `MIT` and the setuptools classifier `License :: OSI Approved :: MIT License`. Those are packaging labels, not the heading in `LICENSE`.

**Source, separate from LICENSE:** `README.md:146-147` also says `The MIT License (MIT)` but prints `Copyright © 2007-2023`. The licence file itself says 2007–2018.

## 3. Why a render cannot see the standard fonts immediately after clone

### Where the standard fonts are stored

**Source:** the checkout stores the redistributable font files in `pyfiglet/fonts-standard/`. That directory contains `standard.flf` (the default font name; see section 5) and an empty `pyfiglet/fonts-standard/__init__.py`. Contributed fonts are a separate tree, `pyfiglet/fonts-contrib/`. `README.md:58-64` describes this split.

**Source:** `pyfiglet/fonts/` is not in the tree. A direct existence check during this step reported `pyfiglet/fonts` absent, and both `pyfiglet/fonts-standard` and `pyfiglet/fonts-contrib` present. `.gitignore` does not ignore `pyfiglet/fonts/`.

### Where the code loads fonts

**Source:** rendering calls `FigletFont.preloadFont` (`pyfiglet/__init__.py:138-175`). For `font` it tries extensions `tlf`, then `flf`, and looks up each name with:

```text
importlib.resources.files('pyfiglet.fonts').joinpath(fn)
```

(`pyfiglet/__init__.py:146-151`). The import package is `pyfiglet.fonts`, which is the directory `pyfiglet/fonts/`, not `pyfiglet/fonts-standard/`. Nothing in `preloadFont` builds a path containing `fonts-standard` or `fonts-contrib`.

If that package file does not exist, the same function continues through `("./",) + FONT_DIRECTORIES` (`pyfiglet/__init__.py:153-157`):

- On `win32`, `FONT_DIRECTORIES` is only `%APPDATA%\pyfiglet` (`pyfiglet/__init__.py:61-63`). `APPDATA` is read when the module is imported.
- Otherwise it is `/usr/local/share/pyfiglet/`, `/usr/local/share/figlet`, and `/usr/share/figlet` (`pyfiglet/__init__.py:65-70`).

`FontNotFound` is raised only when those lookups finish with no data (`pyfiglet/__init__.py:172-175`). `main` turns that into `pyfiglet error: requested font {name!r} not found.` and returns 1 (`pyfiglet/__init__.py:1016-1018`).

**Inference:** on a fresh clone the package directory `pyfiglet/fonts` does not exist, so `importlib.resources.files('pyfiglet.fonts')` fails before the `./` and `FONT_DIRECTORIES` fallbacks and before `FontNotFound`. This step did not import the module, so the exception class was not observed. `setup.py:57` also lists the installable package as `pyfiglet.fonts`, which is the missing directory.

Extension search is not a simple “`.tlf` wins” rule. `break` at `pyfiglet/__init__.py:151` leaves the extension loop only after a package hit. `break` at `pyfiglet/__init__.py:157` leaves only the directory loop. **Source:** a `.tlf` found only on the fallback path can therefore be replaced by a `.flf` found on the next extension pass. A `.tlf` inside `pyfiglet.fonts` stops the search immediately.

### Makefile font setup, separated from other setup

**Source:** `Makefile:7-19`.

`clean` does not install fonts. It deletes the directory and leaves it empty:

```make
clean:
	rm -rf build/
	rm -rf pyfiglet/fonts/
	mkdir pyfiglet/fonts/
```

The font-copy commands are only these `cp` lines:

```make
minimal:    clean
	cp pyfiglet/fonts-standard/* pyfiglet/fonts
	python3 -m build

full:    clean
	cp pyfiglet/fonts-standard/* pyfiglet/fonts
	cp pyfiglet/fonts-contrib/* pyfiglet/fonts
	python3 -m build
```

- `minimal` copies standard fonts, including `fonts-standard/__init__.py` and the `.flc` files in that directory, into `pyfiglet/fonts/`.
- `full` does that copy and then copies `fonts-contrib/*` into the same directory. A contrib file with the same name replaces the standard file because the contrib `cp` is second. Both trees contain `standard.flf`. Their headers differ (`fonts-standard/standard.flf:1` has comment count 46; `fonts-contrib/standard.flf:1` has comment count 13).
- `python3 -m build` is the packaging step after the copy. It does not place the fonts.
- `clean`’s `mkdir` only creates the empty destination. `rm -rf build/` is unrelated artifact removal.
- `publish` (`Makefile:21-23`) uploads `dist/*`. It does not copy fonts.

**Source:** `setup.py:8-13` is a second, non-Makefile setup path. If `pyfiglet/fonts` is not already a directory, it `shutil.copytree`s `pyfiglet/fonts-standard` onto `pyfiglet/fonts`. It copies standard fonts only. It does not copy `fonts-contrib`. It runs only when `setup.py` itself runs.

**Inference:** `make clean` alone blocks that `copytree`, because `clean` leaves `pyfiglet/fonts/` present and empty, and `setup.py` copies only when the destination directory is absent. A fresh clone has neither the copy nor an empty `pyfiglet/fonts/`.

`.flc` files are part of the `cp` because the shell glob is `*`. **Source:** no Python file under `pyfiglet/` references `.flc`. The loader accepts only `.tlf` and `.flf` (`pyfiglet/__init__.py:146` and `:179`).

## 4. Horizontal smushing

There is no vertical-smushing implementation in this file. `FigletFont.smushMode` is the font `fullLayout` integer (`pyfiglet/__init__.py:321`). Later code tests only the masks below. Higher bits are stored and ignored.

Constants (`pyfiglet/__init__.py:533-541`):

| Constant | Bit | Comment in source |
| --- | ---: | --- |
| `SM_EQUAL` | 1 | smush equal chars (not hardblanks) |
| `SM_LOWLINE` | 2 | smush `_` with any char in hierarchy |
| `SM_HIERARCHY` | 4 | hierarchy: `\|`, `/\`, `[]`, `{}`, `()`, `<>` |
| `SM_PAIR` | 8 | `[` + `]` -> `\|`, `{` + `}` -> `\|`, `(` + `)` -> `\|` |
| `SM_BIGX` | 16 | `/` + `\` -> X, `>` + `<` -> X |
| `SM_HARDBLANK` | 32 | hardblank + hardblank -> hardblank |
| `SM_KERN` | 64 | kerning gate, not one of the six rules |
| `SM_SMUSH` | 128 | master switch for the six rules and for universal smushing |

The six controlled rules are bits 1, 2, 4, 8, 16, and 32. Their code does not run unless bit 128 is also set. See kerning below for bit 64.

The hardblank is not a fixed character. It is header field 0, the single character after the magic number (`pyfiglet/__init__.py:295` and `:319`). In `standard.flf` that character is `$` (`fonts-standard/standard.flf:1`, `flf2a$`).

### When rules are reached

**Source:** `smushChars` (`pyfiglet/__init__.py:779-862`) decides the overlapping column. `smushAmount` (`pyfiglet/__init__.py:728-776`) decides whether a column of overlap is allowed at all.

Order inside `smushChars`:

1. ASCII space `' '` on the left returns the right character. ASCII space on the right returns the left character (`:787-790`). This uses `== ' '`, not `isspace`.
2. If `prevCharWidth < 2` or `curCharWidth < 2`, return `None` (`:794-795`). Overlap of ink is refused for a narrow glyph. Spaces have already returned.
3. If bit 128 (`SM_SMUSH`) is clear, return `None` (`:798-799`). This is kerning-only mode.
4. If `(smushMode & 63) == 0`, universal smushing runs and returns immediately (`:802-815`). Mask 63 is `1+2+4+8+16+32`, so this is “none of the six rule bits”.
5. Otherwise the six rules run in the order in the next subsection.

`return` with no value is `None`, meaning “do not smush”.

### The six rules, in the order the code applies them

Examples below trace `smushChars` only. Shared assumptions, unless an example says otherwise:

- bit 128 is set
- only the rule bit being illustrated is set
- both character widths are at least 2
- neither character is ASCII space
- `hardBlank` is `$`
- direction is `left-to-right`, so `smushRow` has not swapped the pair (`pyfiglet/__init__.py:659-660`)

They are not pictures of two rendered letters.

#### 1. Hardblank smushing — `SM_HARDBLANK`, bit 32

**Source:** `pyfiglet/__init__.py:817-824`.

Condition: both characters are the font hardblank. Result: the left character.

If either character is the hardblank and the other is not, the function returns `None` immediately. Later rules, including equal-character smushing, do not see a hardblank.

```text
mode bits 128|32
'$' + '$' -> '$'
'$' + 'A' -> None
'A' + '$' -> None
```

#### 2. Equal character — `SM_EQUAL`, bit 1

**Source:** `pyfiglet/__init__.py:826-828`.

Condition: the characters are equal, neither was rejected as a hardblank, and bit 1 is set. Result: the left character.

```text
mode bits 128|1
'A' + 'A' -> 'A'
'A' + 'B' -> None
'$' + '$' -> None
```

The `$` pair is `None` here because bit 32 is off, so step 1’s “either hardblank” veto runs before equality.

#### 3. Underscore — `SM_LOWLINE`, bit 2

**Source:** `pyfiglet/__init__.py:832-833`, applied by the loop at `:844-848`.

The pair table entry is:

```text
('_', '|/\[]{}()<>')
```

The underscore is class `a`. The other set is class `b`. If one side is in `a` and the other is in `b`, the character from `b` is kept.

```text
mode bits 128|2
'_' + '|' -> '|'
'|' + '_' -> '|'
'_' + '/' -> '/'
'_' + '<' -> '<'
'_' + 'A' -> None
'_' + '_' -> None
```

`A` is not in the second set. `_` is not in the second set either, so two underscores do not match this rule.

#### 4. Hierarchy — `SM_HIERARCHY`, bit 4

**Source:** `pyfiglet/__init__.py:835-842` and the same loop `:844-848`.

Class pairs, from earliest to latest. The character in the second class replaces the character in the first class:

```text
('|' , '/\[]{}()<>')
('\/', '[]{}()<>')
('[]', '{}()<>')
('{}', '()<>')
('()', '<>')
```

So the replacement order is:

```text
|   then   /\   then   []   then   {}   then   ()   then   <>
```

The first matching pair wins. `/` and `\` sit in the second class of the first pair and in the first class of the second pair, so `|` + `/` is decided by the first pair.

```text
mode bits 128|4
'|' + '/' -> '/'
'/' + '|' -> '/'
'/' + '[' -> '['
'[' + '{' -> '{'
'{' + '(' -> '('
'(' + '<' -> '<'
'<' + '|' -> '<'
'/' + '\' -> None
'[' + ']' -> None
'<' + '>' -> None
```

Same-class pairs do not match. `<` and `>` occur only as the last class, never as a left-hand class, so they do not smush with each other under this rule. `_` is not in these strings; without bit 2, `'_' + '|'` is `None`.

#### 5. Opposite pairs — `SM_PAIR`, bit 8

**Source:** `pyfiglet/__init__.py:850-853`.

Condition: the two characters in either order are `[]`, `{}`, or `()`. Result: `|`.

```text
mode bits 128|8
'[' + ']' -> '|'
']' + '[' -> '|'
'{' + '}' -> '|'
'}' + '{' -> '|'
'(' + ')' -> '|'
')' + '(' -> '|'
'<' + '>' -> None
```

This runs after hierarchy. With both bit 4 and bit 8 set, `[` + `<` still follows hierarchy and returns `<`. `[` + `]` does not match hierarchy, so pair smushing can return `|`.

#### 6. Big X — `SM_BIGX`, bit 16

**Source:** `pyfiglet/__init__.py:855-861`. The comment on `:538` says `/ + \ -> X` and `> + < -> X`. The function body does not do that.

```text
mode bits 128|16
'/' + '\' -> '|'
'\' + '/' -> 'Y'
'>' + '<' -> 'X'
'<' + '>' -> None
```

There is no fourth arm for `<` + `>`. Hierarchy does not match that pair either, so with bits 4 and 16 both set the result is still `None`.

### Precedence when several rule bits are set

**Source:** the statements above are sequential. The first success returns.

For two non-space, wide-enough characters, with bit 128 set and at least one of bits 1–32 set:

1. Both hardblanks and bit 32 set: return the left hardblank.
2. Either hardblank: return `None`.
3. Bit 1 and the characters are equal: return the left character.
4. Bit 2, then bit 4, in the class-pair order listed above. The later class wins.
5. Bit 8: opposite bracket pair becomes `|`.
6. Bit 16: the three `BIGX` arms.
7. Otherwise `None`.

```text
mode bits 128|1|2|4
'|' + '|' -> '|'      equality wins; hierarchy never sees it
'_' + '|' -> '|'      underscore rule is earlier than hierarchy
'A' + 'A' -> 'A'

mode bits 128|32|1
'$' + '$' -> '$'      hardblank rule wins over equality
```

A font whose low six bits are all set, and whose bit 128 is set, exercises this full order. Header integers with that mask include `fonts-contrib/3d-ascii.flf` (`fullLayout` 191), `fonts-contrib/train.flf` (24511), and `fonts-contrib/cards.flf` (8127). The bit split was calculated from those header integers. The fonts were not rendered.

### Universal smushing

**Source:** `pyfiglet/__init__.py:801-815`.

This is not a seventh rule bit. It runs when bit 128 is set and `(smushMode & 63) == 0`: none of the six rule bits are set. Bit 64 may be set or clear; it is outside mask 63, but the bit-128 test above has already passed.

Behavior:

- left hardblank returns the right character
- right hardblank returns the left character
- if direction is `right-to-left`, return the left character
- otherwise return the right character

```text
mode bit 128 only, hardBlank '$', left-to-right
'A' + 'B' -> 'B'
'$' + 'A' -> 'A'
'A' + '$' -> 'A'
'$' + '$' -> '$'
```

The source comment (`:809-811`) says the overlapping foreground character should be the later character in the user’s text, not automatically the rightmost glyph. **Inference from `smushRow` plus this return:** `smushRow` swaps the two sides before calling `smushChars` when direction is `right-to-left` (`:659-660`). After that swap, returning `left` is the newly added character. In left-to-right mode the new character is `right` and the code returns `right`. In both directions the later input character wins, including against a hardblank. This combined reading was not executed.

`fonts-contrib/cricket.flf:1` has `fullLayout` 16256, whose tested bits are only `SM_SMUSH`. That is a bundled universal-smushing header. Not rendered.

Universal smushing still stops at the width veto. A character of width 0 or 1 never reaches it (`:794-795`).

### Kerning, separate from the six rules

**Source:** `pyfiglet/__init__.py:737-738` and `:798-799`.

Kerning is horizontal movement that eats ASCII spaces between glyphs. It is not one of the six overlap rules.

- If neither bit 64 nor bit 128 is set, `smushAmount` returns 0. Glyphs are placed at full width. Rule bits have no effect. `fonts-contrib/filter.flf:1` has `fullLayout` 3903, which sets all six rule bits and sets neither 64 nor 128. `fonts-contrib/horizontal_left.flf` is 7999 and has the same property. `fonts-contrib/icl-1900.flf` is 24383 and has the same property. Not rendered.
- If bit 64 is set and bit 128 is not, `smushAmount` still measures a slide, but `smushChars` returns `None` for any non-space pair. Ink does not occupy the same column. Spaces still resolve, because the space returns happen before the bit-128 test.
- If bit 128 is set, the slide is measured whether or not bit 64 is set, and one extra overlapping column is allowed when `smushChars` returns a character.

Per row, after an optional right-to-left swap (`:744-745`):

- The left edge is the last non-ASCII-space on the left row (`rstrip(' ')`).
- The right edge is the first non-ASCII-space on the right row (`lstrip(' ')`).
- The base slide is `charbd + len(lineLeft) - 1 - linebd`.
- That slide grows by one when the left edge is empty or space, or when `smushChars` accepts the two edge characters (`:768-772`).
- The character’s slide is the minimum across font rows, starting from the current character’s stored width (`:740`, `:774-775`).

`banner.flf` in the standard tree has `fullLayout` 64, so its tested bit is kerning only (`fonts-standard/banner.flf:1`). The same is true of `fonts-standard/script.flf` (3904), `fonts-standard/lean.flf` (576), `fonts-standard/block.flf` (576), and `fonts-contrib/rammstein.flf` (8256). Not rendered.

### Old-layout conversion

**Source:** `pyfiglet/__init__.py:306-314`. `fullLayout` is header field 7, present only when the header has more than seven fields (`:303-304`). `oldLayout` is header field 4.

If `fullLayout` is absent:

- `oldLayout == 0` becomes 64 (kerning only)
- `oldLayout < 0` becomes 0 (no kerning and no smushing)
- otherwise `(oldLayout & 31) | 128`

Mask 31 keeps bits 1, 2, 4, 8, and 16. It does not keep bit 32, and the conversion forces bit 128. **Source:** an old-style header cannot turn on hardblank smushing through this conversion.

If the `fullLayout` field is present, `oldLayout` is ignored, including when `fullLayout` is 0.

- `fonts-contrib/georgi16.flf:1` is `flf2a$ 16 2 25 15 20 0 0`. `oldLayout` is 15 and explicit `fullLayout` is 0, so none of the smush or kern bits are set.
- `fonts-standard/blocky_25.flf:1` is `flf2a$ 5 5 20 0 1 0 0`. Explicit `fullLayout` 0 does not become kerning.
- `fonts-contrib/ticks.flf:1` is `flf2a$ 6 5 20 0 9 0`. The `fullLayout` field is absent and `oldLayout` is 0, so the conversion produces 64.
- `fonts-contrib/hex.flf:1` is `flf2a$ 1 1 11 -1 12 0`. The `fullLayout` field is absent and `oldLayout` is negative, so the conversion produces 0.

`standard.flf` does include `fullLayout` 24463 (`fonts-standard/standard.flf:1`). Its tested bits are 1, 2, 4, 8, and 128: equal, underscore, hierarchy, pairs, and the master smush bit. Bits 16, 32, and 64 are clear. `fonts-standard/slant.flf` (18319) and `fonts-standard/ivrit.flf` (16271) have that same tested set. `fonts-standard/digital.flf` is 16513, which is bits 1 and 128 only. These splits were calculated from the header integers, not by rendering.

### Direction-dependent behavior in the builder

**Source:** direction is a string. `Figlet.direction` (`pyfiglet/__init__.py:886-897`) maps `auto` from the font header: `0` to `left-to-right`, `1` to `right-to-left`, and any other value, including a missing header field (`printDirection is None`, `:298-302`), to `left-to-right`. Any non-`auto` string is returned unchanged.

The builder treats only the exact string `right-to-left` as right-to-left (`:659`, `:744`, `:812`). Every other string is assembled left to right.

**Source:** in right-to-left mode `smushRow` swaps the existing row with the new row before overlap, then stores `addLeft + addRight[maxSmush:]` (`:669-671`). The new character is prepended. In left-to-right mode the new character is appended. `smushAmount` swaps before it measures the slide (`:744-745`), so a glyph with different leading and trailing spaces can slide by a different amount in the two directions.

Direction changes rule results only where the two orders are not symmetric:

- Universal smushing returns `left` in right-to-left mode and `right` otherwise, after the swap described above.
- `BIGX` returns `|` for `/` then `\`, `Y` for `\` then `/`, and `X` only for `>` then `<`. The swap can move a pair from one of those arms to another, or off the `>` `<` arm entirely.
- Hardblank and equal smushing return `left`. After a swap, `left` is the other side, but both characters are the same glyph, so the visible character does not change.
- Underscore, hierarchy, and pair smushing return a class winner or `|`. Checking both orders makes those results independent of the swap.

`fonts-standard/ivrit.flf:1` and `fonts-contrib/ivrit.flf:1` are the headers found with print direction `1`. A search of `flf2a` / `tlf2a` headers did not turn up another non-zero print direction. **Not confirmed:** that search is not a proof that no font file uses a multi-digit or unusual print-direction field the pattern could miss. Those `ivrit` files were not rendered.

Auto justification follows this resolved direction. That is separate from smushing and is described with `-j` below.

## 5. CLI behavior of `-f`, `-w`, `-j`, `-D`, `-r`, `-F`, `-s`, and `-n`

The console entry is `pyfiglet = pyfiglet:main` (`setup.py:60-62`). `python -m pyfiglet` calls the same function (`pyfiglet/__main__.py:3-7`). Parsing is `optparse.OptionParser` (`pyfiglet/__init__.py:945-984`). Defaults below are the `add_option` defaults in this file, and they match `Figlet.__init__` (`:870-871`).

`main` joins remaining arguments with one space (`:1009`). With no arguments it prints help and returns 1 (`:1002-1004`). It does not treat an empty string as “no arguments”.

Output order in `main` (`:1020-1039`):

1. `Figlet(...).renderText(text)`, which justifies inside the builder.
2. `-r` reverse, if set.
3. `-F` flip, if set.
4. `-s` strip, if set; otherwise `-n` normalize, if set.
5. On Python 3, `sys.stdout.detach()`, then write optional color, the text encoded UTF-8, and one extra `b'\n'`.

The extra newline is unconditional. `renderText` already ends each flushed line group with `\n` (`replaceHardblanks`, `:723-726`).

### `-f` / `--font`

**Source:** `pyfiglet/__init__.py:52` and `:948-950`.

- Default: `standard`.
- Help text: `font to render with (default: %default)`, so the default is displayed as `standard`.
- The value is a font stem. `preloadFont` adds `.tlf` and `.flf` itself. The lookup order is the one in section 3.
- A missing font raises `FontNotFound`. `main` prints `pyfiglet error: requested font {opts.font!r} not found.` and returns 1.
- The default font file on disk is `pyfiglet/fonts-standard/standard.flf`, which the loader does not open until a setup step has copied or installed it into `pyfiglet.fonts` or another searched directory.

`-i` / `--info_font` prints `FigletFont.infoFont` for this font and exits before rendering (`:994-996`). That flag is outside the requested set; it uses the same `-f` value.

### `-w` / `--width`

**Source:** `pyfiglet/__init__.py:960-962`, builder uses it at `:556-574` and `:710-721`.

- Default: `80`.
- Type: `int`. The parser accepts any integer `optparse` accepts, including negative values. This file does not clamp it.
- Help text: `set terminal width for wrapping/justification (default: %default)`.
- The same integer is both the wrap limit and the justification width.

Wrap check (`:556-574`):

- If `width <` the current character’s stored width, raise `CharNotPrinted("Width is not enough to print this character")`. `main` does not catch `CharNotPrinted`.
- Otherwise `currentTotalWidth = len(buffer[0]) + curCharWidth - maxSmush`.
- If `currentTotalWidth >= width`, the character is not added. `handleNewLine` flushes at the last recorded space or newline (`:703-708`). If none exists, `cutBufferAtLastChar` flushes the current buffer and moves the iterator back by one (`:688-691`), which cancels the upcoming increment and retries the current character on an empty buffer.
- A character is added only in the `else` branch, when `currentTotalWidth < width`.

**Inference:** a glyph whose stored width is exactly `width` fails the `>=` test even on an empty buffer (`maxSmush` against an empty buffer is 0; see `:747-769`). `cutBufferAtLastChar` then rewinds onto that same character. That control flow does not make progress. This was not executed. A width strictly smaller than the glyph raises `CharNotPrinted` instead.

Spaces and newlines push a blank marker before the width test (`:549-551`, `:563-564`). **Inference:** a space that itself crosses the limit becomes the cut point and is not painted; `goToNextChar` then moves past it. The newline test at `:566-568` is unreachable because `:549-552` already returned.

A newline flushes through `handleNewLine` without drawing a newline glyph (`:549-552`).

Characters absent from the font return `None` from `getCharAt` (`:611-619`) and are skipped (`:554-555`). They do not raise. A skipped space cannot be a wrap marker.

### `-D` / `--direction`

**Source:** `pyfiglet/__init__.py:951-955` and `:886-897`.

- Default: `auto`.
- Choices: `auto`, `left-to-right`, `right-to-left`.
- Help text: `set direction text will be formatted in (default: %default)`.
- `auto` uses the font `printDirection`: `0` left-to-right, `1` right-to-left, anything else left-to-right. A header without that field stays left-to-right.
- An explicit choice replaces the font direction. It does not have to match the font. `ivrit.flf` with `-D left-to-right` is left-to-right because the property returns the explicit string before looking at the font.
- The string selects assembly order and the smush swaps in section 4. It does not itself insert justification spaces. Justification is the next flag.

### `-j` / `--justify`

**Source:** `pyfiglet/__init__.py:956-959`, `:900-908`, and `:710-721`.

- Default: `auto`.
- Choices: `auto`, `left`, `center`, `right`.
- Help text: `set justification, defaults to print direction`.

The help sentence is looser than the code. `auto` follows the resolved direction, not always the font header:

- resolved `left-to-right` -> `left`
- resolved `right-to-left` -> `right`
- an explicit `-D` wins over the font’s print direction before this choice is made
- an explicit `-j` wins over both

**Inference:** if a library caller passed `justify='auto'` and a direction string that is neither of those two, `justify` falls off the end of the property and returns `None`. `justifyString` would then pad nothing. The CLI choices cannot produce that pair. It was not executed.

Padding is applied after a line has been built, once per flushed buffer, and before hardblanks become spaces (`:600-609`, `:723-726`). Each font row is padded from its own length, not from the longest row:

```text
right:  (' ' * (width - len(row) - 1)) + row
center: (' ' * int((width - len(row)) / 2)) + row
left:   row unchanged
```

`left` is also the result for any value other than `right` or `center`.

Relative to the selected width:

- Right alignment targets `width - 1`, not `width`. The `- 1` is in the source.
- Center alignment inserts `floor((width - len(row)) / 2)` spaces on the left and does not add a right pad. The leftover column, when the gap is odd, stays on the right.
- Left alignment does not pad out to the width.
- Because a character is added only when `currentTotalWidth < width`, a row that actually grew by the full stored width should already be at most `width - 1` long before padding. Short glyph rows are shorter than that and are padded individually. **Inference, not measured:** right alignment of a full row lands on column `width - 1`, leaving the last column of the requested width unused. If a row were already longer than `width - 1`, the Python repeat count would be negative and the pad would be empty. The builder’s `< width` check is what makes that situation unlikely for a full-width row; this was not executed.

Direction and justification interact only through the `auto` mapping and through the fact that padding is ordinary spaces added after assembly. `-D right-to-left -j left` prepends glyphs and then adds no pad. `-D left-to-right -j right` appends glyphs and then left-pads each row to `width - 1`. `-D right-to-left` with `-j auto` is right alignment because auto follows the resolved direction.

Hardblank replacement happens after padding, so a hardblank counts as one column while the pad is computed, then becomes a space (`:723-726`).

### `-r` / `--reverse`

**Source:** `pyfiglet/__init__.py:963-964`, `:1021-1022`, and `FigletString.reverse` `:433-438`.

- Default: off (`store_true`).
- Help text: `shows mirror image of output text`.
- Applied after rendering and justification, before flip and before `-s` / `-n`.
- Each line is translated with `__reverse_map__` and then reversed. Line length is unchanged because the map is one character to one character.

**Source:** the printable ASCII entries that this map changes, read from `__reverse_map__` at `:396-404`, are only:

```text
'(' -> ')'
')' -> '('
'/' -> '\'
'<' -> '>'
'>' -> '<'
'[' -> ']'
'\' -> '/'
']' -> '['
'{' -> '}'
'}' -> '{'
```

The reversed line therefore also moves justification spaces to the opposite end. Right alignment followed by `-r` puts those spaces on the right. That interaction is a consequence of the order in `main`; it was not executed.

### `-F` / `--flip`

**Source:** `pyfiglet/__init__.py:969-970`, `:1023-1024`, and `FigletString.flip` `:440-445`.

- Default: off (`store_true`).
- Help text: `flips rendered output text over`.
- Applied after `-r` and before `-s` / `-n`.
- The line list is reversed vertically, then every character is translated with `__flip_map__` (`:415-423`). Rows are not individually reversed.

**Source:** printable ASCII characters this map changes:

```text
'/' -> '\'
'A' -> 'V'
'M' -> 'W'
'P' -> 'b'
'R' -> 'b'
'V' -> 'A'
'W' -> 'M'
'\' -> '/'
'^' -> 'v'
'_' -> '-'
'b' -> 'P'
'm' -> 'w'
'v' -> '^'
'w' -> 'm'
```

The map is not a pair of swaps. `P` and `R` both become `b`, while `b` becomes `P`. `_` becomes `-`, and `-` stays `-`. The comment on `:414` says the map is for flipping `^` to `v`; the table also maps `v` back to `^`.

Because flip runs after justification, a padded line keeps its padding on the same side and moves to the opposite vertical position. Empty rows move with it, and `-s` / `-n` then see the flipped lines. Not executed.

### `-s` / `--strip-surrounding-newlines` and `-n` / `--normalize-surrounding-newlines`

**Source:** `pyfiglet/__init__.py:965-968`, `:1025-1028`, `:449-462`.

- Both default off.
- `-s` help: `removes empty leading and trailing lines`.
- `-n` help: `output has one empty line before and after`.
- If both are set, the `if opts.strip_surrounding_newlines` / `elif opts.normalize_surrounding_newlines` structure applies `-s` only. `-n` is not also applied.

`-s` walks the rendered lines (`:449-459`):

- It does not strip the whole string first. The comment on `:447-448` says that would remove leading spaces on the first font line, and that stripping each row would remove empty lines inside a glyph.
- Leading rows whose `row.strip()` is empty are dropped.
- Once a non-whitespace row has been seen, later rows are kept, including empty ones.
- `newFromList` joins with newlines and adds a trailing newline (`:464-465`).
- The final `.rstrip()` removes that trailing newline and any other trailing whitespace. **Source:** `rstrip()` with no argument strips spaces and tabs as well as newlines, so trailing spaces on the last kept line are removed. Internal blank lines stay. Leading spaces on the first kept line stay, because the drop test is `row.strip() != ""`.

`-n` is that strip, then one newline before the result and one newline after it (`:461-462`):

```text
'\n' + strip_surrounding_newlines() + '\n'
```

Then `main` writes one more newline. **Inference, not executed:** CLI `-s` output is the stripped body plus the single extra newline from `main`. CLI `-n` output is a leading newline, the stripped body, the newline added by `normalize_surrounding_newlines`, and the extra newline from `main`.

`-s` and `-n` do not change width, direction, smushing, or justification. They only edit the finished string, and they run after `-r` and `-F`.

`pyfiglet/tests/test_cli.py:15-27` and `:67-81` contain expected stdout strings for `pyfiglet -f slant -s 0` and `pyfiglet -f slant -n 0`. Those tests were not run in this step. The test file’s comment on `:66` says normalize is strip with padding, which matches `:461-462`.

### Other CLI interactions that change these flags

- Color is outside the requested set, but it wraps the same stdout write. The default `-c` value is `:` (`:977`), and `parse_color` of an empty foreground and background writes no ANSI (`:938-942`). `--color=list` exits before rendering.
- `-l` lists fonts and exits before `-f` is rendered (`:986-988`). Its scan uses `pyfiglet.fonts` plus `FONT_DIRECTORIES`, and does not include `./` (`:211-215`). A file that `preloadFont` can see in the current directory can therefore be missing from `-l`.
- On Python 3, `stdout.detach()` switches the stream to binary before the write (`:1030-1032`). Failure of `detach` is not handled in this function. Not executed.

## 6. Candidate cases for later testing

Every item in this list is a candidate derived from the source. None of them was executed in this step. Do not treat the pair traces in section 4 as captured program output.

### Font discovery

- Fresh clone: `pyfiglet/fonts/` is absent, while `standard.flf` is under `pyfiglet/fonts-standard/`. The loader asks for package `pyfiglet.fonts` and does not catch failure of `importlib.resources.files`. The exception class was not observed.
- `make clean` alone leaves an empty `pyfiglet/fonts/`. `setup.py` will not `copytree` into a directory that already exists. `preloadFont` can then get past the package lookup, miss every file, and raise `FontNotFound`.
- `make minimal` copies only `fonts-standard/*`. `make full` copies standard and then contrib, so contrib `standard.flf` replaces the standard one. Their comment counts differ (46 versus 13).
- Package `.tlf` hides every later candidate. A fallback `.tlf` does not hide a later `.flf`, because the extension loop continues.
- `preloadFont` searches `./`. `getFonts` does not. `-f` and `-l` can disagree about a font that exists only in the current directory.
- `isValidFont` opens a bare filename by searching `FONT_DIRECTORIES` and then the package (`:186-192`). `getFonts` calls it with `font.name`, not the full path (`:216-218`). Two different files with the same basename can be discovered in one place and validated from another.
- `.flc` files are copied by the Makefile globs and are never opened by the Python loader.
- A zip font contributes only `namelist()[0]` (`:162-165`, and the same pattern in `isValidFont` at `:196-200`).
- On Windows, importing the module reads `os.environ["APPDATA"]` immediately (`:61-63`). An environment without `APPDATA` fails at import. Not executed. Non-Windows directory constants are not used on `win32`.

### Font parsing

- Magic is `^[tf]lf2.` (`:126`). The last character of that match is the hardblank and can be any single character. Bundled headers use `$`, `?` (`fonts-contrib/heart_left.flf`), `{` (`fonts-contrib/icl-1900.flf`), and the DEL byte shown as `flf2a` plus `0x7F` (`fonts-standard/term.flf`, `fonts-standard/digital.flf`, `fonts-standard/bubble.flf`, `fonts-contrib/cards.flf`).
- A hardblank that is also a smush glyph changes the veto in section 4. With controlled rules, any hardblank next to a different character refuses the overlap, even if the visible character would have matched hierarchy or equality. In universal mode a hardblank loses to the other character instead.
- Fewer than six header fields raises `FontError` (`:292-293`). A wrong `commentLines` consumes glyph lines as comments, or leaves comment text in the glyph stream (`:324-325`). The whole `loadFont` body then becomes `FontError('problem parsing ...')` (`:380-381`).
- U+0085, U+2028, and U+2029 are replaced with a normal space before parsing (`:280`).
- ASCII 32 through 126 is loaded by position. Empty glyphs are dropped except space (`:352-356`). The space glyph is kept even when its body is empty.
- If any data remains, the next seven glyphs are forced to `ÄÖÜäöüß` (`:358-364`). A font that continues after ASCII but does not contain those seven German glyphs will have later glyphs read as those letters.
- Extended glyphs are loaded only when the code token matches `^0x` (`:372-378`). A decimal code token is skipped and its following glyph lines are not consumed, which desynchronizes later glyphs. **Not confirmed:** this step did not scan every font file for a decimal code line. `standard.flf` extended entries that were inspected use `0x` (`fonts-standard/standard.flf:2200` onward), including `0xCA0`.
- `getFonts` keeps only the text before the first dot (`split('.', 2)[0]`, `:216-217`). A filename with more than one dot would be listed under the first component.
- `infoFont(..., short=True)` returns `infos[0]` with no empty check (`:239`). The CLI `-i` path uses `short=False`. An empty info block is a library candidate, not the CLI default.

### Layout integers worth rendering later

Calculated from headers, not rendered:

| Candidate | Header fact | Why it is useful |
| --- | --- | --- |
| `fonts-standard/standard.flf` | `fullLayout` 24463: bits 1, 2, 4, 8, 128 | default font; no big-X, no hardblank rule, no kern bit |
| `fonts-standard/digital.flf` | 16513: bits 1 and 128; hardblank DEL | equality only |
| `fonts-contrib/dancing_font.flf` | 129: bits 1 and 128 | equality only, ordinary `$` hardblank |
| `fonts-standard/banner.flf` | 64: kern bit only | slide without ink overlap |
| `fonts-contrib/cricket.flf` | 16256: bit 128 only | universal smushing |
| `fonts-contrib/3d-ascii.flf` | 191: bits 1–32 and 128 | all six rules at once |
| `fonts-contrib/cards.flf` | 8127: bits 1–32 and 128; hardblank DEL | all six rules plus a non-`$` hardblank |
| `fonts-contrib/filter.flf` | 3903: bits 1–32, no 64, no 128 | rule bits present and inactive |
| `fonts-contrib/georgi16.flf` | `oldLayout` 15, explicit `fullLayout` 0 | old layout ignored |
| `fonts-standard/blocky_25.flf` | explicit `fullLayout` 0 | not converted to kerning |
| `fonts-contrib/ticks.flf` | no `fullLayout`, `oldLayout` 0 | conversion to 64 |
| `fonts-contrib/hex.flf` | no `fullLayout`, `oldLayout` -1, height 1 | conversion to 0, narrow glyphs |
| `fonts-standard/term.flf` | height 1, `fullLayout` 0, hardblank DEL | width veto and no movement |
| `fonts-standard/ivrit.flf` | print direction 1, `fullLayout` 16271 | auto right-to-left with the standard rule set |

`SM_BIGX`’s comment promises `X` for `/` + `\`. The code returns `|` or `Y` depending on order. A font with bit 16 set, such as `3d-ascii.flf`, is the candidate for checking that comment against the function.

### Builder and wrapping

- Width exactly equal to a glyph’s stored width: the `>=` test refuses the glyph, and `cutBufferAtLastChar` rewinds onto the same index. Candidate non-terminating loop. Not executed.
- Width strictly smaller than the glyph: `CharNotPrinted`, uncaught in `main`.
- A line is extended only when the projected width is strictly less than `-w`. Right justification then pads toward `width - 1`.
- Justification uses each row’s own length. A glyph whose rows were stored shorter than its width (`loadFont` does not pad rows; `:342-346`) can receive different left padding on different rows under `-j right` or `-j center`.
- `smushRow` indexes `addRight[i]` for every column of `maxSmush` (`:662-664`). `maxSmush` is bounded by the character width and by the minimum row calculation, but an empty or short row can still report a larger slide when the other side has trailing spaces (`:766-772`). Candidate `IndexError`. Not executed.
- The first character is not a special exception handler. `smushAmount`’s docstring (`:731-735`) says the first character throws and is caught. The function has no `try`. Empty-buffer arithmetic is ordinary and yields a zero slide when the left side is empty.
- Missing code points are skipped, including a missing space. Wrapping then has no blank marker unless a newline was seen.
- A leading newline stores an empty buffer as a completed line (`:549-551` then `:703-706`).
- Joined CLI arguments cannot preserve distinct amounts of separating whitespace: `' '.join(args)` collapses the separators to one space (`:1009`). A newline reaches the builder only when one argument contains it.
- No arguments: help text and return code 1. An empty-string argument is still an argument and is rendered, then `main` adds its extra newline.
- `-D right-to-left` prepends. Combined with `-j auto`, it also right-aligns to `width - 1`. Combined with `-j left`, it prepends and does not pad. Combined with `-r`, the already prepended line is mirrored again, after padding.
- `-j` padding is inside `renderText`. `-r` and `-F` run later, so they transform the padding as well.
- `-s` together with `-n` keeps only `-s`.
- `-s` drops leading all-whitespace rows and `rstrip`s the end, including trailing spaces on the last kept line. It keeps internal empty rows.
- `-n` is that strip plus one newline on each side, and `main` adds another final newline.
- Flip’s translation is not invertible (`R` and `P` both become `b`; `_` becomes `-`). Reverse’s bracket map is symmetric, and the whole line is then reversed.
- `InvalidColor` is not caught by `main`. The RGB branch raises a message whose `{}` is not formatted (`:923`). CLI color syntax was not executed.
- `stdout.detach()` on Python 3 was not executed. The Python 2 decode branch at `:1006-1007` is unreachable under `python_requires=">=3.9"` (`setup.py:23`), but the branch is still in `main`.

## What this step could not confirm

- Any rendered sample, exit code, traceback, or infinite loop. Nothing here was produced by running pyfiglet, `make`, or `setup.py`.
- The exact exception raised when `pyfiglet.fonts` is missing. The call is uncaught in `preloadFont`; the class was not observed.
- Whether every bundled font’s extended-character section uses `0x` codes. The loader’s rule is confirmed; the full font corpus was not scanned for decimal codes.
- Whether any font header has a print direction other than the `0` and `1` values found by the header search. `ivrit.flf` is the only `1` that search found.
- That the local tree byte-matches a fresh download from GitHub. The local HEAD and commit timestamp match the requested pin, and `setup.py` records the GitHub URL.
- Behavior of `optparse` for an unrecognized flag. This file does not define a custom error handler.
- Windows behavior with `APPDATA` unset, and `stdout.detach()` on a non-console stream.

## Runtime verification

Results verified from user-provided terminal output:

- Standard fonts were copied from `pyfiglet/fonts-standard/*` into `pyfiglet/fonts/`, including `__init__.py`.
- Python 3.12.16 imported the original from:
  `C:\Users\AlokJaiswal\Desktop\16-Nov-second-harbor\pyfiglet\pyfiglet\__init__.py`
- Running the original CLI with `-f slant hello` rendered giant slanted letters and exited 0.
- The sibling `verify_smushing.py` called the original `smushChars`:
  - underscore mode 130: `'_'` + `'/'` returned `'/'`.
  - Big-X mode 144: `'/'` + `'\\'` returned `'|'`.
  - Both assertions passed; the script exited 0.
- The Big-X implementation result takes precedence over the misleading source comment. Preserve this discrepancy as a rare-case finding.
- These checks verify basic rendering and two isolated smushing examples, not all rendering behavior.

All other unexecuted candidate cases remain explicitly unverified.

`fonts-contrib` examples are research context only and are excluded from the assignment's task image and generated cases.

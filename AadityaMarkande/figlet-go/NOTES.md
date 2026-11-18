# pyfiglet notes

Reference only. Taken from `pyfiglet/pyfiglet/__init__.py` (`FigletBuilder`, `Figlet`, `main`), `Makefile`, `setup.py`, and `LICENSE`. Nothing in `pyfiglet/` was modified.

## 1. Source

- Repo: https://github.com/pwaller/pyfiglet
- Commit: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Date: `2026-08-02T22:51:18+01:00`
- Package version in that tree: `1.0.4` (`pyfiglet/version.py`)

## 2. Licence

The MIT License (MIT). Copyright 2007-2018 Christopher Jones, Stefano Rivera, Peter Waller, and contributors. `setup.py` sets `license='MIT'`.

## 3. Why `python -m pyfiglet hello` fails right after cloning

`FigletFont.preloadFont` loads `{font}.tlf` then `{font}.flf` from the importable package `pyfiglet.fonts` (`importlib.resources.files('pyfiglet.fonts')`). The default font is `standard`.

A fresh clone does not contain `pyfiglet/fonts/`. The font files live in:

- `pyfiglet/fonts-standard/` (redistributable fonts, including `standard.flf`)
- `pyfiglet/fonts-contrib/` (extra fonts)

`python -m pyfiglet` runs `__main__.py` → `main()` and never copies those directories, so the `pyfiglet.fonts` package is missing and the default font cannot be opened.

`setup.py` does copy `fonts-standard` onto `pyfiglet/fonts` if that directory is absent, but only when setup itself runs (install or build), not when the module is launched from the checkout.

Makefile step that fixes it: `make minimal`. Its recipe is `clean`, then `cp pyfiglet/fonts-standard/* pyfiglet/fonts`, then `python3 -m build`. `clean` deletes `pyfiglet/fonts/` and recreates it empty. After that copy, `standard.flf` is where the loader looks.

`make full` does the same `fonts-standard` copy and also copies `fonts-contrib`. Either target populates `pyfiglet/fonts`; `minimal` is the one that supplies the default font.

## 4. Smushing rules and kerning

Horizontal overlap is controlled by `FigletFont.smushMode` (the font's full-layout integer, or a value converted from the old layout). `FigletBuilder` uses these bits:

| Bit | Name | Value |
| --- | --- | --- |
| 0 | `SM_EQUAL` | 1 |
| 1 | `SM_LOWLINE` | 2 |
| 2 | `SM_HIERARCHY` | 4 |
| 3 | `SM_PAIR` | 8 |
| 4 | `SM_BIGX` | 16 |
| 5 | `SM_HARDBLANK` | 32 |
| 6 | `SM_KERN` | 64 |
| 7 | `SM_SMUSH` | 128 |

`standard.flf` header is `flf2a$ 6 5 16 15 46 0 24463 229`. Hardblank is `$`, height 6, print direction 0, full layout `24463` (`0x5F8F`). The low byte is `0x8F` = `SM_SMUSH | SM_EQUAL | SM_LOWLINE | SM_HIERARCHY | SM_PAIR`. Standard therefore uses rules 1–4. It does not set `SM_BIGX`, `SM_HARDBLANK`, or `SM_KERN`. Higher bits (vertical layout in classic FIGlet) are ignored; pyfiglet never smushes vertically.

Overlap is attempted only when `SM_SMUSH` or `SM_KERN` is set. A space (`' '`, ASCII 0x20 only) is transparent: it disappears and the other character remains. Two visible characters merge only when `SM_SMUSH` is set and a rule below accepts them. A glyph of width 0 or 1 never overlaps a visible neighbour.

The comment on `SM_BIGX` says `/ + \ -> X`. The code does not do that. See rule 5.

### Rule 1 — equal (`SM_EQUAL`)

Two identical characters become one of them. Hardblanks are not handled here; they are decided earlier and do not take this rule.

Example: `|` and `|` → `|`

```
| + |  =>  |
```

### Rule 2 — underscore (`SM_LOWLINE`)

`_` loses to any character in the hierarchy class `|/\[]{}()<>`. The other character replaces the underscore.

Example: `_` and `/` → `/`

```
_ + /  =>  /
```

### Rule 3 — hierarchy (`SM_HIERARCHY`)

Classes from weaker to stronger:

```
|   <   /\   <   []   <   {}   <   ()   <   <>
```

When the two characters are in different classes, the stronger one replaces the weaker one. Same-class pairs are not merged by this rule.

Example: `|` and `/` → `/`

```
| + /  =>  /
```

### Rule 4 — opposite pair (`SM_PAIR`)

A bracket pair, in either order, becomes a vertical bar. Only these three pairs:

Example: `[` and `]` → `|`

```
[ + ]  =>  |
{ + }  =>  |
( + )  =>  |
```

### Rule 5 — big X (`SM_BIGX`)

Three specific ordered pairs. The code result is what is listed, not the `/` + `\` → `X` comment.

Example: `/` and `\` → `|`

```
/ + \  =>  |
\ + /  =>  Y
> + <  =>  X
```

`<` followed by `>` does not match this rule.

### Rule 6 — hardblank (`SM_HARDBLANK`)

The hardblank is the first header field after the magic number (`$` in `standard.flf`). It is a real character while the line is being built, and it is turned into a normal space only when the finished line is emitted.

Two hardblanks become one hardblank. A hardblank next to any other character refuses the overlap (the pair stays two columns), unless universal smushing is in effect (see Rare cases).

Example, with `$` standing for the hardblank: `$` and `$` → `$` (printed later as a space)

```
$ + $  =>  $
$ + |  =>  no overlap
```

### Kerning (`SM_KERN`)

Kerning moves glyphs together by eating blank columns. It does not combine two visible characters. `smushChars` returns immediately when `SM_SMUSH` is off, so rule 1–6 never run. The overlap distance is the run of ASCII spaces between the last non-space of the left glyph and the first non-space of the right glyph.

Example: a right-hand `#` and a left-hand `#` with spaces between them close up until the ink touches.

```
  #        #
 #   +    #    =>     # #
#        #           #   #
                     #     #
```

`SM_KERN` alone enables that. `SM_SMUSH` also enables the same blank-eating, and then tries the six rules on the two characters that actually touch. If neither bit is set, glyphs are placed side by side with no overlap (`maxSmush` is 0).

If `SM_SMUSH` is set and bits 0–5 are all clear (`smushMode & 63 == 0`), the mode is universal overlapping: any non-space pair occupies one column. A visible character beats a hardblank. The character that comes later in the input wins (the right-hand one when printing left-to-right, the left-hand one when printing right-to-left).

## 5. Options

CLI entry is `main()`. Arguments are joined with a single space and passed to `Figlet.renderText`. There is no stdin read. With no text arguments the help is printed and the process returns 1.

After rendering, `-r` is applied, then `-F`, then `-s`, and `-n` only when `-s` was not set. The program then writes the string and always writes one extra `\n`. On Python 3 it detaches `sys.stdout` and writes bytes.

### `-f` / `--font` (default `standard`)

Font name without extension. Lookup tries `{name}.tlf` then `{name}.flf`, first inside package `pyfiglet.fonts`, then `./`, then the shared directories (`/usr/local/share/pyfiglet/`, `/usr/local/share/figlet`, `/usr/share/figlet` on Unix). A missing font prints `pyfiglet error: requested font '...' not found.` and returns 1. `FontNotFound` is the only render error `main` catches.

### `-w` / `--width` (default `80`)

Column budget for wrapping and for right/center padding. A character is kept on the current line only when the line width after overlap is **strictly less than** `width`, so stored lines are at most `width - 1` columns. If the glyph's own width (before overlap) is greater than `width`, `CharNotPrinted` is raised and `main` does not catch it.

### `-j` / `--justify` (default `auto`)

Choices: `auto`, `left`, `center`, `right`.

- `left`: the line is left as built. No padding, no trimming.
- `center`: each row is prefixed with `int((width - len(row)) / 2)` spaces. No spaces are added on the right, so the line is not padded out to `width`.
- `right`: each row is prefixed with `(width - len(row) - 1)` spaces. The finished row is `width - 1` columns, one short of `width`. A negative count (row already longer than `width - 1`) adds no spaces.
- `auto`: follow the resolved print direction. Left-to-right becomes `left`. Right-to-left becomes `right`. Padding is computed per row, and hardblanks still count as characters at this point (they become spaces afterwards).

### `-D` / `--direction` (default `auto`)

Choices: `auto`, `left-to-right`, `right-to-left`.

- `left-to-right`: each new glyph is appended on the right.
- `right-to-left`: the builder swaps the current line and the new glyph before overlapping, so each new glyph is placed on the left. Input is still walked from the first character to the last.
- `auto`: use the font header's print-direction field. `0` means left-to-right, `1` means right-to-left, and any other value (including a missing field) means left-to-right. `standard.flf` is `0`, so auto is left-to-right and auto justify is left.

`auto` justify is decided from that resolved direction, not from the original string `"auto"`.

### `-r` / `--reverse` (default off)

Mirror each finished row left-to-right, then map characters so the mirrored strokes still look like the same kind of stroke: `(`↔`)`, `/`↔`\`, `<`↔`>`, `[`↔`]`, `{`↔`}`. Other ASCII is left as-is. Applied to the whole rendered block, including justification spaces.

### `-F` / `--flip` (default off)

Reverse the row order (bottom row becomes the top), then map characters through the flip table so a vertical turn still reads sensibly: `/`→`\`, `\`→`/`, `^`→`v`, `_`→`-`. The table is the classic FIGlet flip map and is not a mirror of itself (`A`→`V`, `V`→`M`, and so on). If both `-r` and `-F` are set, reverse runs first and flip runs on that result.

### `-s` / `--strip-surrounding-newlines` (default off)

Walk the rows. Drop leading rows whose `strip()` is empty. Once a row contains a non-space character, keep every later row, including internal blank rows. Join them and `rstrip()` the whole string, which removes trailing blank rows and also trailing spaces on the last remaining row. Leading spaces on the first kept row stay. The usual extra `\n` from `main` is still written, so the output ends with a single newline.

### `-n` / `--normalize-surrounding-newlines` (default off)

Do the same strip, then put one `\n` before the text and one `\n` after it. `main` writes yet another `\n`. The visible result is one empty line before the art and one empty line after it. Ignored when `-s` is also set (`-s` wins).

## 6. Rare cases

### Hardblank

- The hardblank is whatever single character the font header gives (`$` in standard). It is not a space during layout.
- ASCII space is the only transparent character. `rstrip(' ')` / `lstrip(' ')` in the overlap math ignore other Unicode whitespace, so a non-ASCII blank is treated as ink.
- Equal-smushing never sees a hardblank. A hardblank paired with anything else returns "no overlap" before the equal test, except the two special cases below.
- `SM_HARDBLANK`: hardblank + hardblank → one hardblank.
- Universal smushing (`SM_SMUSH` set, bits 0–5 clear): a visible character replaces a hardblank; two hardblanks keep the later character in input order.
- Substitution to a real space happens in `replaceHardblanks`, after justify. Trailing hardblanks therefore affect padding width, then show up as trailing spaces.
- A hardblank holds a column open so kerning cannot eat the gap it occupies.

### Unknown characters

- A codepoint with no glyph is skipped. `getCurChar` returns `None` and `addCharToProduct` returns without changing the line, so the character adds no width and is not a wrap point.
- Loaded glyphs are: ASCII 32 through 126 (127 is not required), then the seven German letters `ÄÖÜäöüß` in that order with no codepoint label, then any later lines whose first token matches `0x` / `0X` (hex codepoint) followed by a glyph.
- The space glyph is stored even when its rows are empty. Every other required character whose rows join to `''` is omitted and later behaves as unknown.
- An explicit `\n` (codepoint 10) is not looked up as a glyph. It is handled before the unknown-character return and forces a line break.
- Font bytes are decoded as UTF-8 with `'replace'`. Inside the font text, U+0085, U+2028, and U+2029 are turned into spaces before parsing, so they do not act as line breaks.
- The glyph terminator is the last non-space character on the glyph's first row. One or two of those characters, plus trailing whitespace, are stripped from each row.
- A font file that is a zip uses the first zip member as the font text.
- `.tlf` is preferred over `.flf` for the same font name.

### Wrapping

- Wrap test is `currentTotalWidth >= width` after subtracting the overlap. A line that would land exactly on `width` wraps. Content width is at most `width - 1`.
- On a space, the builder stores a copy of the line **before** that space, and the index of the space. On overflow it restores that copy and sets the cursor back on the space. The following `goToNextChar` steps past the space, so the space is not drawn at the end of the previous line or the start of the next one.
- The latest space wins. Characters after that space are rebuilt on the next line.
- If the current line has no space, the unfinished line is emitted without the character that did not fit, the cursor steps back onto that character, and it becomes the first character of the next line. Words can split.
- A space that itself does not fit is dropped the same way (the saved copy excludes it, then the cursor skips it).
- A raw glyph wider than `width` raises `CharNotPrinted` (`"Width is not enough to print this character"`) before any wrap. `main` does not catch it.
- There is a second `\n` check lower in `addCharToProduct`. It never runs: a newline already returned at the top of the function, and a newline is not a font glyph so the unknown-character return would also have fired.

### Trailing spaces

- Rows are not padded to the glyph width. Short rows stay short, and spaces that the font encoded stay in the row.
- Overlap measurement trims only ASCII spaces, and only to find the touching edges. Those spaces remain in the output unless a later overlap consumes them.
- Left justify does not strip trailing spaces.
- Right and center add spaces on the left only. They do not pad the right edge, and right justify stops at `width - 1`.
- Because padding is per row, a font row that is shorter than the row above gets more left padding. Right or center justify can shear a glyph whose rows have different lengths.
- `-s` / `-n` `rstrip()` the finished block, so trailing spaces survive on every row except the last kept row.

### Empty lines

- An explicit `\n` flushes the current line even when the buffer is still empty. An empty buffer is `height` empty strings, joined into `height` blank output lines (one blank band as tall as the font). Consecutive newlines stack another band each time.
- The final buffer is flushed only when `buffer[0] != ''`. A last line whose top row is still empty (no character ever drew on row 0, and row 0 was not spaces) is dropped. A space glyph in standard is hardblanks, so its top row is `$`, not empty.
- Unknown characters do not produce lines. A string of only unknown characters leaves the buffer empty, `renderText` returns `''`, and the CLI still writes its extra `\n`.
- `-s` removes leading rows that `strip()` considers empty (any Unicode whitespace, not only ASCII space) and removes trailing blank rows via `rstrip()`. Internal empty rows after the first ink row are kept. `-n` is that result with one newline on each side, plus `main`'s extra newline.
- Default CLI output (neither `-s` nor `-n`) already ends with a blank line, because each flushed block ends in `\n` and `main` writes one more.

### Right-to-left

- Enabled by `-D right-to-left`, or by `-D auto` when the font's print-direction field is `1`.
- Each new glyph is overlapped onto the left end of the line. The cursor still advances left-to-right through the input, so the first character ends up on the right.
- Universal smushing keeps the later input character, which is the one just inserted on the left.
- `auto` justify switches to `right`, so the finished RTL line is padded toward the right edge (using the `width - 1` pad).
- Word wrap still breaks on spaces in input order. The saved line is the RTL line as built so far, not a re-ordered LTR line.

### Reversed and flipped output

- `-r` reverses each row's characters and applies the bracket/slash mirror map. It does not change row order. Justification spaces are reversed with the art, so a right-justified line becomes left-justified after the mirror, and the other way around.
- `-F` reverses row order and applies the flip map (`/`, `\`, `^`, `_`, and the letter pairs in `__flip_map__`). It does not reverse characters within a row. The map is lossy: it is not an involution, so flipping twice does not restore the original text.
- Both flags: reverse, then flip. Neither flag changes wrapping or smushing; they only rewrite the finished block.
- `-s` after those flags strips using the rewritten rows. A flip can move a blank font row from the bottom to the top, and `-s` will then drop it as a leading empty line.

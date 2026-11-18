# pyfiglet notes for a Go port

Inspection of the pyfiglet source for a standard-library Go port of the standard-font renderer and command-line interface. Behavior below is taken from this checkout’s `LICENSE`, `Makefile`, `setup.py`, and `pyfiglet/__init__.py`. Classic FIGlet descriptions are not a substitute for the branches in `FigletBuilder`.

## Repository information
- Project: pyfiglet
- URL: https://github.com/pwaller/pyfiglet
- Commit: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: 2026-08-02T22:51:18+01:00
- Inspection date: 2026-11-17
- Version string in `pyfiglet/version.py`: `1.0.4`
- License: MIT, recorded in `LICENSE` and repeated in `pyfiglet/__init__.py` as `__copyright__`

The copyright holders named in `LICENSE` are Christopher Jones `<cjones@insub.org>`, Stefano Rivera `<stefano@rivera.za.net>`, Peter Waller `<p@pwaller.net>`, and various contributors (see git history), for 2007–2018.

The MIT grant allows any person to use, copy, modify, merge, publish, distribute, sublicense, and/or sell the software. The condition is that the copyright notice and the permission notice must be included in all copies or substantial portions of the software. The software is provided “as is”, without warranty of any kind, and the authors and copyright holders are not liable for claims or damages arising from its use.

`setup.py` declares `license='MIT'`, package `pyfiglet` plus `pyfiglet.fonts`, and package data `*.flf`, `*.flc`, and `*.tlf`. The console entry point is `pyfiglet = pyfiglet:main`. `python -m pyfiglet` is `pyfiglet/__main__.py`, which calls `main()`.

## Font setup

`python -m pyfiglet -f slant "hello"` fails on a fresh checkout of this commit because the font files are not on the lookup path yet.

For a font name, preloadFont checks these locations in order:
1. The packaged {name}.tlf resource.
2. If absent, ./{name}.tlf and the platform FONT_DIRECTORIES.
3. The packaged {name}.flf resource.
4. If absent, ./{name}.flf and the platform FONT_DIRECTORIES.

1. The `pyfiglet.fonts` package resource, first `{name}.tlf`, then `{name}.flf`, via `importlib.resources.files('pyfiglet.fonts')`.
2. If that file is absent, `./{name}.tlf` or `./{name}.flf`, then the platform `FONT_DIRECTORIES`. On Windows that tuple is only `%APPDATA%\pyfiglet`. On other platforms it is `/usr/local/share/pyfiglet/`, `/usr/local/share/figlet`, and `/usr/share/figlet`.

`git ls-files pyfiglet/fonts` is empty at this commit. `pyfiglet/fonts` is a build directory, not stored content. Until it exists and contains `slant.flf`, step 1 cannot see the slant font. If the `pyfiglet.fonts` package directory is missing, `importlib.resources.files('pyfiglet.fonts')` raises `ModuleNotFoundError`, and `main()` does not catch that exception. If the package exists but the file does not, `preloadFont` raises `FontNotFound`, and `main()` prints `pyfiglet error: requested font 'slant' not found.` and returns 1.

`setup.py` copies `pyfiglet/fonts-standard` to `pyfiglet/fonts` only when `pyfiglet/fonts` is not already a directory, and only when setup itself runs. Running the module does not perform that copy.

The Makefile `minimal` target is what fills the package directory:

```make
minimal:    clean
	cp pyfiglet/fonts-standard/* pyfiglet/fonts
	python3 -m build
```

`clean` deletes `pyfiglet/fonts/` and recreates it empty, then `minimal` copies every file from `pyfiglet/fonts-standard/` into it. After that copy, `preloadFont` can open `slant.flf` from the `pyfiglet.fonts` package.

`fonts-contrib` is outside this task’s scope. The `full` target copies `fonts-standard` and then `fonts-contrib` into `pyfiglet/fonts`. The README describes that split as a licensing split: `fonts-standard` is the set with a clear redistribution license, and `fonts-contrib` is not. A port of this task should load only the standard set.

The standard font setup was verified locally on 2026-11-17. `pyfiglet/fonts-standard` contains 72 files: 67 `.flf` files, 4 `.flc` files (`upper`, `lower`, `rot13`, `null`), and an empty `__init__.py`. After the copy, `pyfiglet/fonts` contains the same 72 files. `slant.flf` is one of them. Its header is `flf2a$ 6 5 16 15 41 0 18319 96` (hardblank `$`, height 6, full layout `18319`).

## FIGlet rendering and smushing

Rendering entry points are `figlet_format` and `Figlet.renderText`. Both build a `Figlet` and call `FigletRenderingEngine.render`, which constructs a `FigletBuilder` and calls `addCharToProduct` once per input character.

Two different operations move glyphs closer together. The font’s `smushMode` integer (the header’s full layout, or the converted old layout) selects them. The bit constants in `FigletBuilder.__init__` are:

| Bit | Name | Value | Role in this code |
| --- | --- | ---: | --- |
| equal characters | `SM_EQUAL` | 1 | identical characters collapse to one |
| underscore | `SM_LOWLINE` | 2 | `_` yields to a character in the hierarchy set |
| hierarchy | `SM_HIERARCHY` | 4 | a weaker class yields to a stronger class |
| pair | `SM_PAIR` | 8 | `[]`, `{}`, or `()` become `\|` |
| Big X | `SM_BIGX` | 16 | the three pairs implemented below |
| hardblank | `SM_HARDBLANK` | 32 | hardblank plus hardblank stays hardblank |
| kern | `SM_KERN` | 64 | blank columns may overlap |
| smush | `SM_SMUSH` | 128 | occupied columns may overlap |

`FigletFont.loadFont` stores header field 7 as `smushMode` when it is present. When the font has no full-layout field:

- old layout `0` becomes `64` (kern only)
- old layout less than `0` becomes `0` (no kern and no smush)
- any other old layout becomes `(oldLayout & 31) | 128` (the low rule bits, plus smush)

Kerning is overlap of empty columns. `smushAmount` returns `0` immediately when neither `SM_KERN` nor `SM_SMUSH` is set, so glyphs are concatenated at full width. When kerning is enabled, blank columns between the right edge of the left row and the left edge of the right row can disappear. Character combination is a separate step: `smushChars` returns `None` as soon as it sees that `SM_SMUSH` is clear, after it has handled ASCII spaces. Two visible glyphs therefore do not share a cell. They may sit next to each other once the blanks between them are removed.

Smushing is overlap of visible cells. `SM_SMUSH` must be set. `smushAmount` then allows one extra overlapping column when `smushChars` returns a character for the two edge glyphs. `smushRow` writes that character into the left-hand row and `addCurCharRowToBufferRow` appends only `addRight[maxSmush:]`, so the overlapped columns are stored once.

`smushChars(left, right)` is the whole character-combination policy. It returns a one-character string, or `None` when the pair must not occupy one cell. The checks run in this order, and the first match wins:

1. ASCII space. `left == ' '` returns `right`. `right == ' '` returns `left`. This happens before the smush-mode checks, so spaces still collapse when the font is kerning only. The code uses `== ' '` on purpose, and does not call `isspace()`.
2. Narrow glyphs. If `prevCharWidth < 2` or `curCharWidth < 2`, return `None`. A character of width 0 or 1 does not smush visible glyphs.
3. Kerning only. If `SM_SMUSH` is unset, return `None`.
4. Universal smushing, described in the next section. If it applies, the function returns immediately and the six rules below are not used.
5. The six rules.

### The six rules in `FigletBuilder.smushChars`

Examples assume the corresponding bit is set, both glyph widths are at least 2, the characters are not ASCII spaces, and an earlier rule has not already returned. Hierarchy classes are ordinary strings tested with `in`, not regular expressions. `r'|/\[]{}()<>'` is the eleven characters `|/\[]{}()<>`. `r'\/'` is the two characters `\/`.

**Equal characters (`SM_EQUAL`).** If `left == right`, return `left`. Hardblanks do not reach this test; they are handled by the hardblank rule first.

- `A` + `A` produces `A`
- `|` + `|` produces `|`

**Underscore (`SM_LOWLINE`).** The pair `('_', '|/\[]{}()<>')` is appended to the class list. If `left` is in the first string and `right` is in the second, return `right`. If the opposite is true, return `left`. An underscore therefore disappears in favor of the other character. `_` is not a member of the second string, so this rule does not combine `_` with `_`.

- `_` + `|` produces `|`
- `|` + `_` produces `|`
- `_` + `/` produces `/`
- `/` + `_` produces `/`

**Hierarchy (`SM_HIERARCHY`).** These pairs are appended after the underscore pair, in this order. The left string is the weaker class. The stronger character is kept:

| Weaker class | Yields to |
| --- | --- |
| `\|` | `/ \ [ ] { } ( ) < >` |
| `/ \` | `[ ] { } ( ) < >` |
| `[ ]` | `{ } ( ) < >` |
| `{ }` | `( ) < >` |
| `( )` | `< >` |

- `|` + `/` produces `/`
- `/` + `[` produces `[`
- `[` + `{` produces `{`
- `{` + `(` produces `(`
- `(` + `<` produces `<`
- `<` + `(` produces `<` (the angle bracket is in the stronger class)

`[` and `]` are in the same hierarchy class, and that class does not yield to itself, so a closing bracket is left for the pair rule.

**Pair (`SM_PAIR`).** After the class list, the code tests `left+right` and `right+left` against `[]`, `{}`, and `()`. A match returns `|`. Angle brackets are not a pair.

- `[` + `]` produces `|`
- `]` + `[` produces `|`
- `{` + `}` produces `|`
- `(` + `)` produces `|`

**Big X (`SM_BIGX`).** The constant comment says `/ + \ -> X, > + < -> X`. The function body does not do that. These are the only three comparisons, in this order:

- `left == '/'` and `right == '\\'` returns `'|'`
- `right == '/'` and `left == '\\'` returns `'Y'`
- `left == '>'` and `right == '<'` returns `'X'`

Checked against the method:

- `/` + `\` produces `|`
- `\` + `/` produces `Y`
- `>` + `<` produces `X`
- `<` + `>` produces `None`

The slash rule is not symmetric, and `<` followed by `>` is not smushed.

**Hardblank (`SM_HARDBLANK`).** The hardblank is header token 0. In `slant.flf` it is `$`. If the bit is set and both characters are the hardblank, return `left` (still the hardblank). Then, whether or not the bit is set, if either character is the hardblank, return `None`. A hardblank therefore combines only with another hardblank, and only when the bit is set. It does not combine with a letter under these rules. Universal smushing, below, is the path that can replace a hardblank with a visible character.

- `$` + `$` produces `$` when `$` is the font hardblank and the bit is set
- `$` + `A` produces `None`
- `A` + `$` produces `None`

If none of the rules match, `smushChars` returns `None`.

### Universal smushing and direction-sensitive character selection

Universal smushing is `(smushMode & 63) == 0`. The mask `63` is the six rule bits. None of them are set, and step 3 already required `SM_SMUSH`, so any two glyphs may overlap. The chosen character is:

- if `left` is the hardblank, return `right`
- if `right` is the hardblank, return `left`
- if `direction == 'right-to-left'`, return `left`
- otherwise return `right`

With rule bits clear and `SM_SMUSH` set:

- left-to-right `A` + `B` produces `B`
- right-to-left `A` + `B` produces `A`
- hardblank + `A`, and `A` + hardblank, produce `A`

The direction branch prefers the later character in the user’s text. `smushRow` makes that line up with the buffer. For `right-to-left` it swaps the current row with the buffered row before combining, builds onto that swapped left side, and the caller stores `addLeft + addRight[maxSmush:]`. The new character is therefore prepended, and inside `smushChars` the later character is `left`. For `left-to-right` there is no swap, the new character is `right`, and it is appended after the overlapped prefix. The dominant glyph is the one that occurs later in the input, which is the rightmost glyph only when the direction is left-to-right.

`smushAmount` uses the same swap before it measures edges, so the overlap distance is computed in that same order. For each font row it takes the last non-space of the left line (`rstrip(' ')` only) and the first non-space of the right line (`lstrip(' ')` only). The candidate overlap is the blank columns between those edges, plus one when the left edge is empty or a space, or when `smushChars` accepts the two edge characters. The smallest candidate across all rows is `maxSmush`, and it starts capped at the current character width.

`slant.flf` full layout `18319` (`0b100011110001111`) has `SM_EQUAL`, `SM_LOWLINE`, `SM_HIERARCHY`, `SM_PAIR`, and `SM_SMUSH`. It does not have `SM_BIGX`, `SM_HARDBLANK`, or `SM_KERN`. Because some of the low six bits are set, slant does not take the universal-smushing branch. Its overlaps are the individual rules whose bits are present.

## CLI options

`main()` uses `optparse`. With no text arguments it prints help and returns 1. Otherwise the remaining arguments are joined with single spaces and passed to `Figlet`.

`-f` / `--font` chooses the font name. The default is `standard` (`DEFAULT_FONT`). The name is passed to `FigletFont`, which loads `{name}.tlf` or `{name}.flf` as described above. A missing font raises `FontNotFound` and the process prints `pyfiglet error: requested font {name!r} not found.`

`-w` / `--width` is an integer column width. The default is `80`. It is both the wrap width and the width used by justification. See the width rules in the next section.

`-j` / `--justify` is one of `auto`, `left`, `center`, `right`. The default is `auto`. The help text says justification defaults to the print direction. `Figlet.justify` implements `auto` as `left` when the resolved direction is `left-to-right`, and `right` when it is `right-to-left`. Any other explicit value is returned unchanged. `justifyString` then:

- `left`: leaves each row as built
- `right`: prefixes `width - len(row) - 1` spaces
- `center`: prefixes `int((width - len(row)) / 2)` spaces

There is no trailing pad. The right-justification expression subtracts an extra 1, so a right-justified row is one column shorter than `width` after padding.

`-D` / `--direction` is one of `auto`, `left-to-right`, `right-to-left`. The default is `auto`. `auto` reads the font header’s print direction: `0` is left-to-right, `1` is right-to-left, and a missing or other value is left-to-right. An explicit setting is used as given. Direction changes which side the new glyph is smushed onto, as described above. It also selects the automatic justification.

`-r` / `--reverse` sets a flag, default false. After `renderText`, `FigletString.reverse` runs. Each line is translated and then reversed, so the art is a mirror image. The translation changes only these ASCII characters; every other byte maps to itself:

- `(` ↔ `)`
- `/` ↔ `\`
- `<` ↔ `>`
- `[` ↔ `]`
- `{` ↔ `}`

`-F` / `--flip` sets a flag, default false. After rendering (and after reverse, if both are set), `FigletString.flip` reverses the order of the lines and translates each line. The translation changes only:

- `A` ↔ `V`, `M` ↔ `W`, `^` ↔ `v`, `m` ↔ `w`, `/` ↔ `\`
- `P` → `b`, `R` → `b`, `b` → `P`, `_` → `-`

`P` and `R` both become `b`, so this map is not an involution. Reverse and flip are post-processing of the finished string. They are not part of `smushChars`.

`-s` / `--strip-surrounding-newlines` calls `strip_surrounding_newlines`. That walk keeps a row once a non-blank row has been seen, which drops leading empty lines and keeps empty lines inside the glyph. It does not strip the line itself, so leading spaces on the first content row stay, and internal all-space rows stay. `newFromList` joins with newlines and adds a trailing newline; `rstrip()` then removes trailing whitespace, including trailing blank lines and trailing spaces on the last content line.

`-n` / `--normalize-surrounding-newlines` is applied only in the `elif`, so `-s` wins when both flags are set. Normalization is `'\n' + strip_surrounding_newlines() + '\n'`: one empty line before the stripped art and one empty line after it.

`main()` encodes the result as UTF-8, writes it to stdout, and then writes one more `\n`.

These original options are outside the required Go-port scope:

- `-l` / `--list_fonts` prints `FigletFont.getFonts()` sorted, one name per line, and exits 0.
- `-i` / `--info_font` prints `FigletFont.infoFont` for the font selected by `-f`, and exits 0. That method returns comment lines from the top of the font file.
- `-L` / `--load` passes a font file or zip to `FigletFont.installFonts`, which copies it into the installed fonts directory or `%APPDATA%\pyfiglet` / `/usr/local/share/pyfiglet/`, and exits 0.
- `-c` / `--color` defaults to `:`. `parse_color` splits foreground and background on the first colon and emits ANSI SGR codes from `COLOR_CODES`, or `38;2;R;G;B` / `48;2;R;G;B`. `--color=list` prints the supported names and exits 0. Invalid names raise `InvalidColor`.

## Important implementation observations

`FigletBuilder` is the renderer. `FigletRenderingEngine.render` only loops `addCharToProduct` and `goToNextChar` until the input is consumed, then returns `returnProduct()`. The builder holds the input as code points, the current index, the font, direction, width, justification, the per-row buffer, the running smush width, and a `FigletProduct` queue of finished line buffers.

`getCharAt` returns `None` when the code point is absent from `font.chars`. `addCharToProduct` then skips it. Unknown characters are omitted, not replaced.

**Width and newlines.** `currentTotalWidth` is `len(buffer[0]) + curCharWidth - maxSmush`. The glyph is smushed into the buffer only when that value is strictly less than `width`. A prospective line of length `width` or more calls `handleNewLine` instead of appending the glyph. Rows therefore stay shorter than `width` before justification.

Before that comparison, a space records a blank marker: a copy of the current buffer and the current index. An input newline is handled at the top of `addCharToProduct`: it records the same kind of marker and flushes immediately. `handleNewLine` pops the newest marker. If one exists, that saved buffer is emitted and the index is moved back to the marker, so `goToNextChar` resumes on the following character. The space or newline that caused the break is not drawn on the next line. If no marker exists, the current buffer is emitted and the index is decremented, so the glyph that did not fit is tried again on the empty line.

If `width < curCharWidth`, the builder raises `CharNotPrinted` (`"Width is not enough to print this character"`) before the wrap test. On an empty buffer, `smushAmount` is the smallest number of leading ASCII spaces across the glyph’s rows, which is `0` when any row starts with a non-space. A glyph of stored width `width` with no leading space on some row therefore has `currentTotalWidth == width`, fails the fit test, and `cutBufferAtLastChar` retries that same character.

`returnProduct` flushes a non-empty final buffer. `formatProduct` runs `justifyString` and then `replaceHardblanks` on each queued buffer. Hardblank replacement joins the rows with `\n`, adds a trailing `\n`, and replaces every hardblank character with a normal space. Justification therefore still sees hardblanks, and they count toward the row length.

**Smush amount and character combination.**

- `FigletBuilder.smushAmount` decides how many columns may overlap. `currentSmushAmount` is the call used for the glyph about to be added.
- `FigletBuilder.smushChars` decides what a single overlapping pair becomes, or refuses the pair with `None`.
- `smushRow` and `addCurCharRowToBufferRow` apply those two results to one font row. `getLeftSmushedChar` indexes the last `maxSmush` columns of the left row, and the right side contributes columns `0` through `maxSmush - 1`.

**Font loading that the builder assumes.** `loadFont` requires a header matching `^[tf]lf2.`. That prefix (`flf2a`, `tlf2a`, and so on) is stripped, and the remaining tokens are hardblank, height, baseline, max length, old layout, comment-line count, optional print direction, and optional full layout. Fewer than six remaining tokens is `FontError`. Comment lines are discarded. Each glyph is `height` lines. The end marker is the last non-space character of the glyph’s first line; one or two of those markers, plus trailing whitespace, are stripped from every line. The stored width is the longest remaining line. Rows are not padded out to that width. Required characters are code points 32 through 126. An empty glyph is dropped, except space (32), which is kept even when its rows are empty because wrapping looks spaces up. If data remains, the loader then reads `ÄÖÜäöüß`, then extended characters introduced by a `0x` hex code. A zip font contributes its first zip member.

`FigletString` is the returned string type. `newFromList` joins lines with `\n` and always ends with `\n`. The CLI may strip, normalize, and then append one more newline, as described above.

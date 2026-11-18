Repo URL: https://github.com/pwaller/pyfiglet
SHA and date: 255827efbc09d662875dad50fa0edc1e1263bf45 2026-08-02T22:51:18+01:00
Licence: MIT

# pyfiglet notes

## Licence

The MIT License (MIT).

`LICENSE` and the copyright block in `pyfiglet/__init__.py` both name it that way. `setup.py` sets `license='MIT'` and the classifier `License :: OSI Approved :: MIT License`.

## Why `python -m pyfiglet hello` fails after a clone

The command loads the default font, `standard`, through `FigletFont.preloadFont`. That looks up `standard.flf` (then `standard.tlf`) with `importlib.resources.files('pyfiglet.fonts')`.

A fresh clone has no `pyfiglet/fonts` package. The font files live in `pyfiglet/fonts-standard` and `pyfiglet/fonts-contrib`, and nothing copies them until a build step runs. The import therefore raises:

`ModuleNotFoundError: No module named 'pyfiglet.fonts'`

`make minimal` fixes it. That target depends on `clean` (which creates an empty `pyfiglet/fonts/`) and then runs:

```
cp pyfiglet/fonts-standard/* pyfiglet/fonts
```

`standard.flf` is in `fonts-standard`, so after that copy the default font imports and `python -m pyfiglet hello` renders. `make full` does the same copy and then adds `fonts-contrib`. `make clean` alone leaves `pyfiglet/fonts/` empty, so the command still fails.

## Smushing rules

`FigletBuilder` takes these layout bits from the font header (`fullLayout`). A rule runs only when its bit is set. The six smush rules also need `SM_SMUSH` (128). If neither `SM_SMUSH` nor `SM_KERN` is set, characters are placed end to end and `smushAmount` returns 0.

Spaces are not ink: a space next to any character is replaced by that character. Two glyphs whose characters are narrower than 2 columns are not overlapped. In right-to-left mode the two sides are swapped before these rules run.

| Bit | Name | What it does | Example |
| --- | --- | --- | --- |
| 1 | Equal (`SM_EQUAL`) | Two identical characters become one. Hardblanks are not equal-smushed. | `A` + `A` → `A` |
| 2 | Underscore (`SM_LOWLINE`) | `_` against a character in `\|/\[]{}()<>` is replaced by that character. | `_` + `\|` → `\|` |
| 4 | Hierarchy (`SM_HIERARCHY`) | Classes, weaker to stronger: `\|`, `/\`, `[]`, `{}`, `()`, `<>`. The stronger class replaces the weaker one. | `\|` + `/` → `/` |
| 8 | Opposite pair (`SM_PAIR`) | A bracket and its opposite become a bar. | `[` + `]` → `\|`, `{` + `}` → `\|`, `(` + `)` → `\|` |
| 16 | Big X (`SM_BIGX`) | Three diagonal or angle pairs collapse to one character. | `/` + `\` → `\|`, `\` + `/` → `Y`, `>` + `<` → `X` |
| 32 | Hardblank (`SM_HARDBLANK`) | Two hardblanks (the font’s hardblank, `$` in `standard`) become one hardblank. A hardblank next to any other character does not smush under this rule. Printed output later turns the hardblank into a space. | `$` + `$` → `$` |
| 64 | Kerning (`SM_KERN`) | Shift the next character left by the blank columns between the two glyphs, and stop when two non-space characters would share a cell. | row `"X "` + row `" Y"` → `"XY"` |

`SM_SMUSH` (128) is the switch that allows the six rules to overlap glyphs. With that bit set and none of the six rule bits set, overlapping is universal: a hardblank loses to the other character, and otherwise the later character in the text wins (`right` left-to-right, `left` right-to-left).

## Command-line options

Parsed in `main()` and applied in this order: render, then `-r`, then `-F`, then `-s` or else `-n`.

- **`-f` / `--font`** — Font name to render with. Default is `standard`. Passed to `Figlet` as `font`. A missing font prints `pyfiglet error: requested font ... not found.` and exits 1.
- **`-w` / `--width`** — Column width used for wrapping and justification. Default is 80. The builder starts a new line when the next character would make the line at least this wide, breaking at the last space when there is one. A character wider than this width raises `CharNotPrinted`.
- **`-j` / `--justify`** — Horizontal alignment: `auto`, `left`, `center`, or `right`. `auto` follows the print direction (`left` for left-to-right, `right` for right-to-left). `left` adds no padding. `center` inserts `(width - line length) / 2` spaces. `right` inserts `width - line length - 1` spaces.
- **`-D` / `--direction`** — `auto`, `left-to-right`, or `right-to-left`. `auto` reads the font header: 0 is left-to-right, 1 is right-to-left, anything else is left-to-right. Right-to-left swaps the two sides while smushing, so characters accumulate in reverse, and it also changes the default for `-j`.
- **`-r` / `--reverse`** — Mirror each output row: swap mirrored glyphs (`/` with `\`, `(` with `)`, `<` with `>`, `[` with `]`, and the rest of the reverse map) and then reverse the row.
- **`-F` / `--flip`** — Turn the output over: reverse the row order top to bottom, then map glyphs through the flip table (`A`↔`V`, `M`↔`W`, `^`↔`v`, `b`↔`P`, and so on).
- **`-s` / `--strip-surrounding-newlines`** — Drop empty lines before the first line that has ink, and strip trailing newlines. Blank lines inside the figure stay. If `-s` and `-n` are both set, `-s` is the one applied.
- **`-n` / `--normalize-surrounding-newlines`** — Same trim as `-s`, then one empty line before the figure and one after it. Used only when `-s` is not set.

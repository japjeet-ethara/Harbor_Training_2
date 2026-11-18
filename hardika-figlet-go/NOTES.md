# Upstream behavior

- Repository [pwaller/pyfiglet](https://github.com/pwaller/pyfiglet).
- Reference commit [`255827efbc09d662875dad50fa0edc1e1263bf45`](https://github.com/pwaller/pyfiglet/commit/255827efbc09d662875dad50fa0edc1e1263bf45), committed on `2026-08-02T22:51:18+01:00`.
- pyfiglet is MIT-licensed. [LICENSE.pyfiglet](LICENSE.pyfiglet) reproduces its license and notices. The FIGlet fonts are upstream's standard selection and retain their individual headers.
- The loader looks in the package's `pyfiglet/fonts` resource and then in its font search directories. The source checkout keeps the standard definitions in `pyfiglet/fonts-standard`. `Makefile` prepares the destination in `clean` and copies the standard files in `minimal`. `setup.py` also copies the standard package if the destination doesn't exist.

```sh
mkdir -p pyfiglet/fonts
cp pyfiglet/fonts-standard/* pyfiglet/fonts/
python -B -m pyfiglet -f slant hello
```

## Horizontal layout

The header's full-layout value takes precedence. In its absence, old layout zero selects kerning, negative selects full width, and positive selects controlled smushing with the old layout's low five bits. The following tiny examples were checked against the pinned `FigletBuilder.smushChars` with glyph widths at least two where required. See [upstream-smoke.log](evidence/upstream-smoke.log).

| Rule | Bit | Checked example and behavior |
| --- | ---: | --- |
| Equal | 1 | `\|` plus `\|` becomes `\|`. Hardblanks are handled before this rule. |
| Underscore | 2 | `_` plus `/` becomes `/`. The underscore yields to a character in the hierarchy. |
| Hierarchy | 4 | `\|` plus `{` becomes `{`. Higher groups win in the order pipe, slashes, brackets, braces, parentheses, angle brackets. |
| Opposite pair | 8 | `[` plus `]` becomes `\|`. Matching opposite square brackets, braces and parentheses work in either order. |
| Big X | 16 | Slash plus backslash becomes `\|`. Backslash plus slash becomes `Y`. Greater-than plus less-than becomes `X`. |
| Hardblank | 32 | Two `$` hardblanks in the standard font become one hardblank. It serializes to a space. |
| Kerning | 64 | A normal space can give way to `/`. Touching slashes cannot merge under kerning alone. |

Bit 128 enables smushing. With zero controlled-rule bits, the latter input glyph wins unless its edge is a hardblank and the other edge is visible. Controlled smushing protects hardblanks except for the enabled hardblank pair. Glyph widths below two preclude a colliding ink merge.

## Command-line behavior

| Option | Pinned behavior |
| --- | --- |
| `-f FONT` | Picks the font. Default `standard`. |
| `-w COLS` | Defaults to 80. Wrapping tests the current first-row width plus the next glyph's width minus overlap. Reaching the configured width triggers a break. A leading zero selects legacy octal when there is no explicit binary or hexadecimal prefix. |
| `-D MODE` | `auto` follows the font's print direction when specified, otherwise left to right. Explicit `left-to-right` or `right-to-left` supersedes the font. RTL adds each next glyph on the left. |
| `-j SIDE` | `auto` selects left for LTR and right for RTL. Right padding on the left is width minus row length minus one. Center padding on the left is half the spare width rounded toward zero. |
| `-r` | Mirrors each output row and swaps slash, bracket, brace, parenthesis and angle-bracket pairs. |
| `-F` | Reverses row order and translates characters, including slash pairs, `A/V`, `M/W`, `m/w`, `^/v`, `_` to `-`, `P` and `R` to `b`, and `b` to `P`. The mirror runs first if both flags are set. |
| `-s` | Drops whitespace-only leading rows and trailing whitespace. Takes precedence over `-n`. |
| `-n` | Adds one newline on each side of the stripped rendering before the CLI adds its final newline. |

The CLI joins multiple text arguments with a single space. The renderer ignores unavailable glyphs, breaks on an explicit newline, and rolls back to the latest input space when it wraps. Trailing spaces, hardblank replacement, empty rows, and the final CLI newline affect the exact byte sequence. The corpus includes paired punctuation, line breaks at spaces, spaces touching a line edge, unsupported characters next to supported text, Latin-1, narrow RTL, and interacting transforms.

The pinned parser reads the standard ASCII glyphs and legacy German entries. It recognizes hexadecimal extended tags, and counts row width in Unicode code points. The standard selection contains 67 `.flf` files, four control files, and one Python package marker. Only the `.flf` definitions are embedded by the Go executable.

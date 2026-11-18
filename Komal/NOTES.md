# Upstream notes

- Source [pwaller/pyfiglet](https://github.com/pwaller/pyfiglet)
- Commit [255827efbc09d662875dad50fa0edc1e1263bf45](https://github.com/pwaller/pyfiglet/commit/255827efbc09d662875dad50fa0edc1e1263bf45)
- Commit date `2026-08-02T22:51:18+01:00`
- MIT copyright © 2007–2018 Christopher Jones, Stefano Rivera, Peter Waller, and various contributors. See the [full upstream notice](solution/UPSTREAM-LICENSE). The standard font files retain their embedded notices.

`FigletFont` loads fonts from the `pyfiglet.fonts` package. A clean checkout stores them in `pyfiglet/fonts-standard`. The Makefile's `clean` target creates `pyfiglet/fonts` and the `minimal` target copies the standard font files into it. `setup.py` also copies the standard directory when the destination is missing.

```bash
mkdir -p pyfiglet/fonts
cp pyfiglet/fonts-standard/* pyfiglet/fonts/
python -m pyfiglet -f slant hello
```

## Horizontal overlap

The examples are cell pairs in display order. Controlled smushing requires the respective layout flag and both glyph widths to be at least two. The source resolves hardblanks before checking equal, lowline, hierarchy, opposite pairs, and Big X.

| Rule | Behavior and checked pair |
| --- | --- |
| Kerning (64) | Spaces around occupied cells allow overlap. Space plus a vertical bar keeps the bar. Two occupied cells cannot collide under kerning alone. |
| Equal (1) | Two identical ordinary cells collapse, such as two vertical bars. |
| Lowline (2) | An underscore yields to a hierarchy character. `_` plus `/` yields `/`. |
| Hierarchy (4) | The higher class wins across classes. The ranks are vertical bar, slash and backslash, square brackets, braces, parentheses, then angle brackets. `/` plus `[` yields `[`. |
| Opposite pair (8) | Complementary square brackets, braces, or parentheses collapse in either order. `]` plus `[` yields a vertical bar. |
| Big X (16) | Slash plus backslash yields a vertical bar. Backslash plus slash yields `Y`. `>` plus `<` yields `X`. |
| Hardblank (32) | Two hardblanks merge. With the standard font's hardblank, `$` plus `$` yields `$`, which becomes a space at output time. |

When smushing is active and the layout has no controlled rule bits, universal smushing chooses the incoming glyph's occupied cell. A visible cell takes precedence over a hardblank. The examples were checked against the pinned `FigletBuilder.smushChars`. [Execution record](evidence/upstream-checks.txt)

## CLI

| Option | Pinned behavior |
| --- | --- |
| `-f FONT` | Selects a font, default `standard`. |
| `-w COLS` | Sets wrap and justification width, default 80. A projected width at the limit triggers wrapping. |
| `-j auto/left/center/right` | Sets row alignment, default `auto`. Right prepends width minus row length minus one spaces. Center uses half the difference, truncated to an integer. |
| `-D auto/left-to-right/right-to-left` | Sets assembly direction, default `auto`. Auto reads the font header and falls back to left-to-right. Automatic alignment follows the resolved direction. |
| `-r` | Mirrors each row with the reverse character map. |
| `-F` | Reverses row order and applies the flip character map. Runs after mirroring. |
| `-s` | Removes leading whitespace-only rows and trims trailing whitespace. Takes precedence over `-n`. |
| `-n` | Applies the strip operation and adds a newline before and after. The CLI appends its own final newline after any transform. |

The CLI joins positional arguments with one space. Rendering skips unsupported code points, flushes at literal newlines, and saves the canvas before input spaces for word wrapping. Font row lengths count Unicode code points. Opposing slash order, unknown characters inside words, successive spaces, font-directed RTL, and combined transforms informed the corpus.

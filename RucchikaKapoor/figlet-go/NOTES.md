# pyfiglet notes

- Repository: https://github.com/pwaller/pyfiglet
- Commit: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: `2026-08-02T21:51:18Z`.
- License: MIT.
- Font setup: `make minimal` removes/recreates `pyfiglet/fonts/`, copies `pyfiglet/fonts-standard/*` into it, then builds the package. The Python package also copies the standard fonts during `setup.py` if `pyfiglet/fonts/` does not exist.

## Kerning and smushing

`FigletBuilder` supports kerning and six smushing rules. Kerning overlaps edge columns that are spaces without replacing visible characters.

1. **Equal character:** two equal, non-space characters overlap into one (`|` + `|` -> `|`).
2. **Underscore:** `_` overlaps a hierarchy character, which replaces it (`_` + `/` -> `/`).
3. **Hierarchy:** the character later in the hierarchy order wins (`|` + `/` -> `/`). The order is `|`, `/\`, `[]`, `{}`, `()`, `<>`.
4. **Opposite pair:** `[]`, `{}`, or `()` overlap into `|` (`[` + `]` -> `|`).
5. **Big X:** `/` + `\` -> `|`; `\` + `/` -> `Y`; `>` + `<` -> `X`.
6. **Hardblank:** two hardblanks overlap into one hardblank (`$` + `$` -> `$` for a font whose hardblank is `$`); it is rendered as a space at the end.

## CLI options in scope

- `-f FONT`: select the font; defaults to `standard`.
- `-w COLS`: set the wrapping and justification width; defaults to 80.
- `-j auto|left|center|right`: select justification; `auto` follows the print direction.
- `-D auto|left-to-right|right-to-left`: set print direction; `auto` follows the font's direction metadata.
- `-r`: mirror the rendered rows, including reversing FIGcharacters such as `/` and `\`.
- `-F`: reverse the row order and flip FIGcharacters vertically.
- `-s`: strip surrounding blank lines and trailing whitespace.
- `-n`: normalize surrounding newlines; when combined with `-s`, `-s` takes precedence.

The CLI joins positional text arguments with single spaces. Rendering skips characters missing from a font. Output is UTF-8.

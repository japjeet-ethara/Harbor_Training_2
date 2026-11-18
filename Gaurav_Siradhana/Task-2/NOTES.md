# pyfiglet research notes

- Repository: https://github.com/pwaller/pyfiglet
- Pinned commit: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: 2026-08-02T21:51:18Z
- License: MIT

pyfiglet is a Python implementation of FIGlet. It reads FIGlet `.flf` and
`.tlf` font files and renders text as multi-line ASCII art. Its renderer
handles font-defined character data, kerning and smushing, wrapping,
justification, direction, mirroring, and flipping.

## Font setup

A fresh checkout stores fonts in `pyfiglet/fonts-standard/`, while the package
loads them from `pyfiglet/fonts/`. The Makefile's `minimal` target runs
`make clean`, copies `pyfiglet/fonts-standard/*` into `pyfiglet/fonts/`, and
builds the package. The `full` target also copies contributed fonts. For this
task, use only the standard-font copy step:

```sh
mkdir -p pyfiglet/fonts
cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

Without that setup, the package has no bundled font definitions to load, so
`python -m pyfiglet hello` fails while looking for its default `standard` font.

## Layout rules

FIGlet fonts choose the available layout bits in their header. The renderer
supports these six smushing rules (examples assume the rule is enabled):

| Rule | Example | Result |
|---|---|---|
| Equal character | `|` + `|` | `|` |
| Underscore | `_` + `/` | `/` |
| Hierarchy | `/` + `|` | `/` |
| Opposite pair | `[` + `]` | `|` |
| Big X | `>` + `<` | `X`; `/` + `\` gives `|`, while `\` + `/` gives `Y` |
| Hardblank | hardblank + hardblank | one hardblank |

The hierarchy is `|`, `/\`, `[]`, `{}`, `()`, `<>`, from highest to lowest.
At each adjacent hierarchy level, the higher-ranked character wins. A
hardblank is the font header's special character (often `$`); it remains a
hardblank when two meet, and is converted to a space in final output.

Kerning is overlap without combining two visible characters. It allows blank
columns at the edges of neighboring glyphs to overlap. Smushing is enabled
separately by the font layout bits; universal smushing is the smushing layout
with no individual rule bits set.

## Command-line options in scope

- `-f FONT`: select the named font.
- `-w COLS`: set the line width used for wrapping and justification.
- `-j auto|left|center|right`: select line justification. `auto` follows the
  resolved print direction. Centering uses the requested width.
- `-D auto|left-to-right|right-to-left`: select rendering direction.
  `auto` follows the print direction in the font header.
- `-r`: mirror the rendered rows horizontally, including FIGlet's character
  substitutions for mirrorable glyphs.
- `-F`: flip the rendered output vertically and apply FIGlet's character
  substitutions.
- `-s`: strip surrounding blank lines and trailing whitespace from the
  rendered art.
- `-n`: normalize surrounding newlines by adding one empty line before and
  after the rendered art. If `-s` and `-n` are both supplied, stripping wins.

Words given after the options are joined with one space by the CLI. Options
outside this list are not part of this task.

## Rare cases to exercise

- `|_|`, `[]{}()`, `<><>`, and `/\\` exercise interacting smushing rules.
- `___` and `__/` exercise underscore smushing.
- `~` exercises a font's hardblank behavior when present in glyphs.
- `two  spaces` checks that repeated spaces are preserved within an argument.
- `é ß ñ` exercises Latin-1 glyphs; `🙂 ☃` exercises unavailable glyphs.
- A long sentence at width 10 checks wrapping around spaces.
- Right-to-left rendering at a narrow width combines direction, wrapping,
  smushing, and justification.

## Grader sanity checks

- Nothing implemented: `0.0`.
- Build only: `1/6`, approximately `0.166667`.
- Perfect build with all hidden cases: `1.0`.

## Step 8 notes

- Smushing rule: no mismatch remained in the oracle's hidden cases; the
  asymmetric Big-X rule is the highest-risk corner.
- Docker trap: Alpine has no Bash by default; the final stage installs it, and
  the image check finds `/bin/bash`.

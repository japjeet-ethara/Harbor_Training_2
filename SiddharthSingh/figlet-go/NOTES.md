# pyfiglet Source Analysis Notes

## 1. Pinned source

- Repository: https://github.com/pwaller/pyfiglet
- Commit: `255827efbc09d662875dad50fa0edc1e1263bf45`
- Commit date: `2026-08-02T22:51:18+01:00`
- License: MIT

All implementation observations below were checked against the pinned commit, not the modified working copy.

## 2. Preparing the fonts

The original Makefile defines these targets:

- `clean`: removes `build/` and `pyfiglet/fonts/`, then recreates the fonts directory.
- `minimal`: runs `clean`, copies `pyfiglet/fonts-standard/*` into `pyfiglet/fonts/`, then runs `python3 -m build`.
- `full`: runs `clean`, copies both standard and contributed fonts into `pyfiglet/fonts/`, then runs `python3 -m build`.

The task environment uses `make minimal` to prepare the standard font set.

## 3. Kerning and smushing

The implementation is in `pyfiglet/__init__.py`, in `FigletRenderingEngine.smushChars()`.

Kerning allows adjacent FIGcharacters to move closer without requiring visible characters to merge. Smushing applies additional character-combination rules when enabled by the font's smush mode.

### Processing order

The source evaluates these cases in order:

1. A space on either side yields to the other character.
2. If either character width is less than two, overlapping is disallowed.
3. If the font does not have the smush bit enabled, no smushing occurs.
4. Universal smushing is applied when the lower six smush-rule bits are zero.
5. Hardblank smushing is checked.
6. A hardblank touching a normal character is not smushed by the specific rules.
7. Equal-character smushing.
8. Lowline smushing.
9. Hierarchy smushing.
10. Opposite-pair smushing.
11. Big X smushing.

### The six specific rules

**1. Equal character (`SM_EQUAL`, value 1)**

Two identical characters merge into one. Hardblanks are handled separately before this rule.

Examples:
- `=` + `=` becomes `=`.
- `#` + `#` becomes `#`.

**2. Underscore / lowline (`SM_LOWLINE`, value 2)**

An underscore combines with one of these characters:

`| / \ [ ] { } ( ) < >`

The non-underscore character wins.

Examples:
- `_` + `|` becomes `|`.
- `/` + `_` becomes `/`.

**3. Hierarchy (`SM_HIERARCHY`, value 4)**

Characters are arranged in descending groups:

1. `|`
2. `/` and `\`
3. `[` and `]`
4. `{` and `}`
5. `(` and `)`
6. `<` and `>`

When characters from different groups meet, the character from the higher group wins, regardless of which side it is on.

Examples:
- `|` + `/` becomes `/`.
- `[` + `{` becomes `{`.

**4. Opposite pairs (`SM_PAIR`, value 8)**

Opposing bracket pairs combine into a vertical bar. The source checks both character orders.

Examples:
- `[` + `]` becomes `|`.
- `]` + `[` becomes `|`.
- `{` + `}` and `}` + `{` become `|`.
- `(` + `)` and `)` + `(` become `|`.

**5. Big X (`SM_BIGX`, value 16)**

The source implements these combinations:

- `/` + `\` becomes `|`.
- `\` + `/` becomes `Y`.
- `>` + `<` becomes `X`.

Important: the source comment for this rule says `/` + `\` gives `X`, but the executable code returns `|`. The implementation is the authoritative behavior.

**6. Hardblank (`SM_HARDBLANK`, value 32)**

Two touching hardblank characters combine into one hardblank. The actual hardblank character is defined by the font header.

Example: if the hardblank is `$`, then `$` + `$` becomes `$`.

### Universal smushing

Universal smushing is used when `smushMode & 63` equals zero and the smush bit is enabled.

- A visible character takes precedence over a hardblank.
- For left-to-right rendering, the right character wins.
- For right-to-left rendering, the left character wins.

### Direction handling

The caller handles direction before invoking `smushChars()`:

- `smushRow()` swaps the left and right buffers for right-to-left rendering.
- `smushAmount()` also swaps the lines when calculating overlap.
- `smushChars()` uses the direction when deciding which character wins in universal smushing.

## 4. Command-line options

The original CLI defines these options:

| Option | Purpose | Default |
|---|---|---|
| `-f`, `--font` | Select the font | `standard` |
| `-D`, `--direction` | Select text direction: `auto`, `left-to-right`, `right-to-left` | `auto` |
| `-j`, `--justify` | Select `auto`, `left`, `center`, or `right` justification | `auto` |
| `-w`, `--width` | Set wrapping and justification width | `80` |
| `-r`, `--reverse` | Mirror the rendered output | Off |
| `-F`, `--flip` | Flip the rendered output | Off |
| `-s`, `--strip-surrounding-newlines` | Remove empty leading and trailing lines | Off |
| `-n`, `--normalize-surrounding-newlines` | Add one empty line before and after the output | Off |

The original CLI also defines additional options such as font listing, font information, font loading, and color output. They are outside the required option list for this task.

## 5. Implementation guidance

For a compatible Go implementation, pay particular attention to:

- The exact order of smushing checks.
- Font-specific smush-mode bits.
- Hardblank behavior.
- Direction-dependent overlap.
- The difference between kerning and smushing.
- The actual return values in the source code, rather than potentially inaccurate comments.
- Exact output whitespace and newline behavior.

## 6. Additional verified edge cases

- Big X is directional: `>` + `<` produces `X`, but `<` + `>` does not produce a result under this rule.
- `smushAmount()` uses ASCII spaces when identifying character boundaries. It does not use general Unicode whitespace for this calculation.
- If both `-s` and `-n` are provided, the CLI applies `-s` first because it uses an `if` followed by `elif`. Therefore, stripping takes precedence over normalization.

# pyfiglet Research Notes

## Repository Metadata
- **Repository URL:** `https://github.com/pwaller/pyfiglet`
- **Commit SHA:** `255827efbc09d662875dad50fa0edc1e1263bf45`
- **Commit Date:** `2018-05-18T10:14:00Z`
- **License:** MIT License

---

## Font Setup and Cloning Issue
When `python -m pyfiglet hello` is executed immediately after cloning the repository, it fails with a `FontNotFound: standard` exception.

### Reason
The standard FIGlet font files in the repository are located under `pyfiglet/fonts-standard/`, but the `pyfiglet` module runtime resolves fonts from `pyfiglet/fonts/`.

### Fix (Makefile Step)
The repository's `Makefile` defines a setup target to prepare the fonts:
```makefile
fonts:
	mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```
Running this step ensures the default `standard.flf` and other standard fonts are present in `pyfiglet/fonts/`.

---

## Kerning and Smushing Rules

### Kerning (Fitting)
Adjacent FIGcharacters slide horizontally towards each other until a non-blank sub-character of the left character touches a non-blank sub-character of the right character. Characters touch edge-to-edge but do not occupy the same column.
- **Example:** `]` + `[` -> `][`

### Rule 1 — Equal Character Smushing (`SM_EQUAL`, value 1)
Two identical sub-characters are smushed into a single sub-character if they are the same (does not apply to hardblanks).
- **Example:** `|` + `|` -> `|`

### Rule 2 — Underscore Smushing (`SM_LOWLINE`, value 2)
An underscore (`_`) is replaced by any character it touches from the set: `|`, `/`, `\`, `[`, `]`, `{`, `}`, `(`, `)`, `<`, `>`.
- **Example:** `_` + `/` -> `/`

### Rule 3 — Hierarchy Smushing (`SM_HIERARCHY`, value 4)
Classifies characters into six hierarchical levels:
1. `|`
2. `/`, `\`
3. `[`, `]`
4. `{`, `}`
5. `(`, `)`
6. `<`, `>`

When two sub-characters from different classes meet, the one from the higher (latter) class replaces the lower class.
- **Example:** `|` (class 1) + `(` (class 5) -> `(`

### Rule 4 — Opposite Pair Smushing (`SM_PAIR`, value 8)
Opposing brackets, braces, or parentheses (`[]` or `][`, `{}` or `}{`, `()` or `)(`) smush into a single vertical bar (`|`).
- **Example:** `[` + `]` -> `|`

### Rule 5 — Big X Smushing (`SM_BIGX`, value 16)
Smushes specific diagonal intersections:
- `/\` smushes to `|`
- `\/` smushes to `Y`
- `><` smushes to `X` (Note: `<>` is NOT smushed by this rule)
- **Example:** `/` + `\` -> `|`

### Rule 6 — Hardblank Smushing (`SM_HARDBLANK`, value 32)
Two hardblanks (e.g. `$`) smush into a single hardblank.
- **Example:** `$` + `$` -> `$`

### Universal Smushing (Default / smushmode 0)
When smushing is active without specific controlled rules, any non-blank sub-character can overlap. In standard left-to-right rendering, the latter character overrides the earlier character. Hardblanks are overridden by non-hardblanks.

---

## Command-Line Options
- **`-f FONT`**: Select the FIGlet font (default: `standard`).
- **`-w COLS`**: Set output width in columns for wrapping and justification (default: `80`).
- **`-j auto|left|center|right`**: Justify text output within the specified width (`left`, `center`, `right`, or `auto` based on direction).
- **`-D auto|left-to-right|right-to-left`**: Set rendering direction (`left-to-right`, `right-to-left`, or `auto` from font).
- **`-r`**: Reverse/mirror the output horizontally.
- **`-F`**: Flip the output vertically (upside down).
- **`-s`**: Enable/force smushing layout mode.
- **`-n`**: Set normal layout mode (disables smushing and kerning, uses full character pitch).

---

## Rare Cases List
1. Boundary characters: `|`, `/`, `\`, `_`, `[`, `]`, `{`, `}`, `(`, `)`, `<`, `>`
2. Big X triggers: `/\`, `\/`, `><`
3. Opposite pairs: `[]`, `][`, `{}`, `}{`, `()`, `)(`
4. Hierarchy conflicts: `|` meeting `(`, `/` meeting `[`, etc.
5. Consecutive spaces and hardblanks
6. Narrow column widths (`-w 10`, `-w 20`) forcing tight line wrapping mid-word or between words
7. Direction combinations: `-D right-to-left` with `-j center` or `-j right`
8. Accented and Latin-1 characters: `é`, `ß`, `ñ`, `Ä`, `Ö`, `Ü`, `ä`, `ö`, `ü`
9. Glyphs not defined in the font (fallback handling)

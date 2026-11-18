# NOTES.md - pyfiglet analysis

## 1. Repository Details & License
- **URL:** https://github.com/pwaller/pyfiglet
- **SHA:** `255827efbc09d662875dad50fa0edc1e1263bf45`
- **Commit Date:** `2026-08-02T22:51:18+01:00`
- **License:** MIT License (Copyright 2007-2018 Christopher Jones, Stefano Rivera, Peter Waller, and contributors)

## 2. Why `python -m pyfiglet hello` Fails Straight After Cloning
Straight after cloning:
1. `pyfiglet/__main__.py` does not exist in the repository root or top-level package layout without installing or pointing Python to the right package path.
2. Even when invoking the module entrypoint `pyfiglet.main()`, the font loader (`preloadFont` / `importlib.resources.files('pyfiglet.fonts')`) attempts to load fonts from `pyfiglet/fonts/`, but the cloned repository stores them in `pyfiglet/fonts-standard/` (and `fonts-contrib/`). Because `pyfiglet/fonts` directory does not exist or is empty, `ModuleNotFoundError: No module named 'pyfiglet.fonts'` or `FontNotFound` is raised.

### Makefile Fix
The `Makefile` target `minimal` (or `clean` + copy) fixes this setup:
```sh
mkdir -p pyfiglet/fonts
cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```
Once copied into `pyfiglet/fonts/`, `pyfiglet` finds the default and standard fonts.

## 3. Smushing Rules & Kerning
In FIGlet and pyfiglet (`FigletBuilder.smushChars`), characters on adjacent column edges can be smushed when overlapping, subject to font `smushMode` bit flags:

- **Kerning (`SM_KERN = 64`):**
  Moves characters closer together until sub-characters touch, but does not allow any sub-character cell overlap.
  - *Example:* `' '` + `'|'` -> `'|'` (whitespace collapses to boundary).

- **Rule 1: Equal Characters (`SM_EQUAL = 1`):**
  Two identical sub-characters smush into that same character (excluding hardblanks).
  - *Example:* `'|'` + `'|'` -> `'|'` (also verified: `'/'` + `'/'` -> `'/'`).

- **Rule 2: Underscore Hierarchy (`SM_LOWLINE = 2`):**
  An underscore (`'_'`) smushes with any character in `'|/\\[]{}()<>'`, with the other character dominating.
  - *Example:* `'_'` + `'|'` -> `'|'`.

- **Rule 3: Hierarchy (`SM_HIERARCHY = 4`):**
  Smushes characters based on visual class dominance:
  Class 1: `'|'`
  Class 2: `'/'`, `'\\'`
  Class 3: `'['`, `']'`
  Class 4: `'{'`, `'}'`
  Class 5: `'('`, `')'`
  Class 6: `'<'`, `'>'`
  A higher-class character overrides a lower-class character.
  - *Example:* `'|'` + `'/'` -> `'/'`.

- **Rule 4: Opposite Pair (`SM_PAIR = 8`):**
  Opposing brackets/braces/parentheses smush together into a vertical bar `'|'`:
  `'['` + `']'` -> `'|'`, `'{'` + `'}'` -> `'|'`, `'('` + `')'` -> `'|'`.
  - *Example:* `'['` + `']'` -> `'|'`.

- **Rule 5: Big X (`SM_BIGX = 16`):**
  `'/'` + `'\\'` -> `'|'`, `'\\'` + `'/'` -> `'Y'`, and `'>'` + `'<'` -> `'X'`.
  - *Example:* `'>'` + `'<'` -> `'X'` (or `'\\'` + `'/'` -> `'Y'`).

- **Rule 6: Hardblank (`SM_HARDBLANK = 32`):**
  Two hardblank characters (e.g. `'$'` or character specified in the font header) smush into a single hardblank character.
  - *Example:* `'$'` + `'$'` -> `'$'` (when `'$'` is hardblank).

*(Note: Universal smushing `SM_SMUSH = 128` allows foreground overlap when no other rule applies; when `smushMode & 63 == 0`, the later character dominates).*

## 4. Command-Line Options
- `-f, --font <FONT>`: Specifies the FIGlet font to render with (defaults to `standard`).
- `-w, --width <COLS>`: Sets the target terminal column width for text line wrapping and justification (defaults to 80).
- `-j, --justify <SIDE>`: Sets text justification (`auto`, `left`, `center`, `right`). Defaults to print direction.
- `-D, --direction <DIR>`: Sets formatting and rendering direction (`auto`, `left-to-right`, `right-to-left`).
- `-r, --reverse`: Reverses the horizontal output (mirror image of output text).
- `-F, --flip`: Flips the rendered output text vertically upside down.
- `-s, --strip-surrounding-newlines`: Strips empty leading and trailing newline rows from output.
- `-n, --normalize-surrounding-newlines`: Ensures the output has exactly one empty line before and after rendered text.


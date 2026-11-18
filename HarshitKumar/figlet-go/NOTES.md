# pyfiglet: notes for the Harbor task (figlet-go)

- **Repo URL:** https://github.com/pwaller/pyfiglet
- **Commit SHA:** 255827efbc09d662875dad50fa0edc1e1263bf45
- **Commit date:** 2026-08-02T22:51:18+01:00
- **Version:** 1.0.2
- **Licence:** MIT (`LICENSE` file, "MIT License", Copyright (c) Peter Waller <p@pwaller.net> and FIGlet contributors). Allowed.

---

## 1. Why `python -m pyfiglet hello` fails after cloning & Makefile fix

When `pyfiglet` is cloned directly from GitHub, running `python -m pyfiglet hello` raises:
```text
pyfiglet.FontNotFound: standard
```
**Reason:** 
`pyfiglet/__init__.py` searches for fonts under `pyfiglet/fonts/`. However, the repository source keeps standard bundled font files in `pyfiglet/fonts-standard/` and contributed fonts in `pyfiglet/fonts-contrib/`. The `pyfiglet/fonts/` directory does not exist in a clean checkout.

**Makefile fix:**
The project's `Makefile` contains:
```makefile
fonts:
	mkdir -p pyfiglet/fonts
	cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```
Stage 1 of the Dockerfile must execute this exact step (`mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/`).

---

## 2. Six Smushing Rules + Kerning

FIGlet characters ("fig-characters") are composed of sub-characters arranged in matrices. When two characters are joined horizontally, FIGlet applies horizontal smushing (`smushMode`) governed by standard FIGlet 2.2 rules:

### Kerning (rule 0 / default horizontal mode):
- Sub-characters from the right glyph move into whitespace (blanks/spaces) of the left glyph until non-whitespace sub-characters touch. No characters are overwritten or replaced.
- *Example:* `'A'` and `'B'` slide together until their visible outlines touch.

### Rule 1: Equal Character Smushing (`SM_EQUAL = 1`)
- Two identical sub-characters smush into that same sub-character.
- *Rule:* Does not smush two hardblanks (usually `$`).
- *Example:* `'|'` + `'|'` -> `'|'`, `'/'` + `'/'` -> `'/'`.

### Rule 2: Underscore Smushing (`SM_LOWLINE = 2`)
- An underscore (`'_'`) is smushed by any visible character of higher priority: `'|'`, `'/'`, `'\\'`, `'['`, `']'`, `'{'`, `'}'`, `'('`, `')'`, `'<'`, `'>'`.
- *Example:* `'_'` + `'/'` -> `'/'`, `'|'` + `'_'` -> `'|'`.

### Rule 3: Hierarchy Smushing (`SM_HIERARCHY = 4`)
- Classes of characters with hierarchy precedence smush into the higher-class character:
  - Class 1 (lowest): `'|'`
  - Class 2: `'/'` and `'\\'`
  - Class 3: `'['` and `']'`
  - Class 4: `'{'` and `'}'`
  - Class 5: `'('` and `')'`
  - Class 6 (highest): `'<'` and `'>'`
- *Example:* `'|'` + `'/'` -> `'/'`, `'['` + `'{'` -> `'{'`, `'('` + `'<'` -> `'<'`.

### Rule 4: Opposite Pair Smushing (`SM_PAIR = 8`)
- Matching brackets, in either order, smush into `|`:
  - `'['` + `']'` or `']'` + `'['` -> `'|'`
  - `'{'` + `'}'` or `'}'` + `'{'` -> `'|'`
  - `'('` + `')'` or `')'` + `'('` -> `'|'`
- Slashes are NOT part of this rule.

### Rule 5: Big X Smushing (`SM_BIGX = 16`)
- `'/'` + `'\\'` -> `'|'`
- `'\\'` + `'/'` -> `'Y'`
- `'>'` + `'<'` -> `'X'`
- Note: the comment in the source says `/ + \ -> X`, but the code returns `|`. The code is the truth.

### Rule 6: Hardblank Smushing (`SM_HARDBLANK = 32`)
- Two hardblanks (specified by the font header, typically `'$'`) smush into a single hardblank.
- *Example:* `'$'` + `'$'` -> `'$'`.

---

## 3. Supported CLI Options

| Flag | Meaning & Behaviour |
| :--- | :--- |
| **`-f FONT`** | Specifies the font name (e.g., `standard`, `slant`, `small`, `big`, `mini`, `banner`, `shadow`, `script`). Loaded from `<repo>/pyfiglet/fonts/<FONT>.flf`. |
| **`-w COLS`** | Output line width in columns (integer, default 80). Determines word wrapping and line formatting boundaries. |
| **`-j JUST`** | Justification mode: `auto` (default, follows font direction), `left` (left-aligned), `center` (centered within width `-w`), `right` (right-aligned within width `-w`). |
| **`-D DIR`** | Print direction: `auto` (default, follows font header), `left-to-right` (LTR), `right-to-left` (RTL). In RTL mode, the text is built from right to left. |
| **`-r`** | Mirror: shows the mirror image of the rendered text. |
| **`-F`** | Flip: turns the rendered output upside down. |
| **`-s`** | Strip: removes empty lines at the start and end of the output. |
| **`-n`** | Normalize: makes the output have exactly one empty line before and after. If -s is also given, -s takes effect. |

*(Note: Options `-c` [color], `-L` [install], `-l` [list], and `-i` [info] are explicitly excluded by the specification).*

---

## 4. Rare Cases (Gold List)

1. **Hardblanks replacement**: In normal non-smushed space, hardblanks (`$`) must be converted to spaces `' '` at final render time.
2. **Trailing spaces**: Lines wrapped or centered must not emit arbitrary trailing whitespace if the terminal width boundary cuts short.
3. **Punctuation smushing**: Non-alphanumeric punctuation marks like `|`, `/`, `\`, `_` in narrow widths (`-w 10` or `-w 20`) trigger multiple overlapping smushing rules in sequence.
4. **RTL (`-D right-to-left`) with Center Justify (`-j center`)**: Character smushing order is reversed, and line padding is computed from the total rendered width relative to `-w`.
5. **Multi-line and wrapped text**: Text exceeding `-w` must break cleanly on word boundaries (whitespace). Words longer than `-w` break across lines.
6. **Unknown characters**: Characters the font does not draw are skipped and draw nothing. pyfiglet draws printable ASCII plus the letters Ä Ö Ü ä ö ü ß; é ï ñ and others are skipped. Text made only of skipped characters gives a blank line.

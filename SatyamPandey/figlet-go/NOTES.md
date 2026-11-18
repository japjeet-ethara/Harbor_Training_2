# pyfiglet Notes

## Repository URL

<https://github.com/pwaller/pyfiglet>

## Commit SHA and Date

- **SHA:** `255827efbc09d662875dad50fa0edc1e1263bf45`
- **Date:** `2026-08-02T22:51:18+01:00`

## Licence

**MIT**

## Why `python -m pyfiglet hello` Fails After Cloning

After cloning, the directory `pyfiglet/fonts/` does not exist. The code in
`FigletFont.preloadFont()` tries to load font files from the `pyfiglet.fonts`
package (via `importlib.resources.files('pyfiglet.fonts')`), but that package
directory is missing, so Python cannot import it and raises a `FontNotFound`
error (or `ModuleNotFoundError`).

The font source files live in `pyfiglet/fonts-standard/` (and optionally
`pyfiglet/fonts-contrib/`), but they are **not** in the location the code
expects at runtime.

The **`make minimal`** (or `make full`) Makefile target fixes this. It runs:

```
cp pyfiglet/fonts-standard/* pyfiglet/fonts/
```

which copies the standard `.flf` font files into `pyfiglet/fonts/`, making the
`pyfiglet.fonts` package importable and its font files discoverable. (The
`make clean` step, which `minimal` and `full` depend on, first removes any
stale `pyfiglet/fonts/` directory and recreates it empty.)

## Smushing Rules

FIGlet defines six controlled-smushing rules (bit-flags 1–32) plus kerning
(flag 64). The smush mode 128 enables smushing; when it is set the six rules
below are tested. If kerning (64) is set without smushing (128), characters are
moved together until they touch but never overlap.

### 0. Kerning (flag 64 — `SM_KERN`)

| Property | Value |
|----------|-------|
| **What it does** | Moves adjacent characters together by removing inter-character spaces until their bounding boxes touch, but never overlaps or replaces any character. |
| **Example** | `A  B` → `AB` (space removed, no characters merged) |

### 1. Equal Character Smushing (flag 1 — `SM_EQUAL`)

| Property | Value |
|----------|-------|
| **What it does** | Two identical non-hardblank characters are replaced by a single copy of that character. |
| **Example** | `XX` → `X` |

### 2. Underscore Smushing (flag 2 — `SM_LOWLINE`)

| Property | Value |
|----------|-------|
| **What it does** | An underscore `_` is replaced by any character from the set `\|/\[]{}()<>`. The non-underscore character wins. |
| **Example** | `_/` → `/` |

### 3. Hierarchy Smushing (flag 4 — `SM_HIERARCHY`)

| Property | Value |
|----------|-------|
| **What it does** | Characters belong to a hierarchy of classes: `\|` < `/\` < `[]` < `{}` < `()` < `<>`. When two characters from different classes meet, the one from the higher (later) class replaces the other. |
| **Example** | `\|]` → `]` |

### 4. Opposite Pair Smushing (flag 8 — `SM_PAIR`)

| Property | Value |
|----------|-------|
| **What it does** | Matching bracket/brace/parenthesis pairs (`[]`, `{}`, `()`) are replaced by a vertical bar `\|`. |
| **Example** | `[]` → `\|` |

### 5. Big X Smushing (flag 16 — `SM_BIGX`)

| Property | Value |
|----------|-------|
| **What it does** | `/` + `\` are replaced by `\|`, `\` + `/` are replaced by `Y`, and `>` + `<` are replaced by `X`. |
| **Example** | `><` → `X` |

### 6. Hardblank Smushing (flag 32 — `SM_HARDBLANK`)

| Property | Value |
|----------|-------|
| **What it does** | Two adjacent hardblank characters (the font's hardblank marker, e.g. `$`) are smushed into a single hardblank. Hardblanks otherwise prevent smushing. |
| **Example** | `$$` → `$` |

## Command-Line Options

| Option | Long Form | What It Does |
|--------|-----------|--------------|
| `-f` | `--font` | Selects the FIGlet font to render with (default: `standard`). The name refers to a `.flf` or `.tlf` file in the fonts directory. |
| `-w` | `--width` | Sets the output width in columns (default: 80). Used for line wrapping and justification of the rendered text. |
| `-j` | `--justify` | Sets horizontal justification: `left`, `center`, or `right`. Default is `auto`, which follows the print direction (left-to-right → left-justified, right-to-left → right-justified). |
| `-D` | `--direction` | Sets the text direction: `left-to-right`, `right-to-left`, or `auto` (reads the font's built-in print direction). Default is `auto`. |
| `-r` | `--reverse` | Mirrors the rendered output horizontally (left ↔ right), swapping characters like `/` ↔ `\`, `(` ↔ `)`, `[` ↔ `]`, etc. |
| `-F` | `--flip` | Flips the rendered output vertically (top ↔ bottom), swapping characters like `^` ↔ `v`, `/` ↔ `\`, etc. |
| `-s` | `--strip-surrounding-newlines` | Removes empty leading and trailing lines from the output, giving a tightly cropped result. |
| `-n` | `--normalize-surrounding-newlines` | Ensures exactly one empty line before and one empty line after the rendered output. Mutually exclusive with `-s` (if `-s` is given, `-n` is ignored). |

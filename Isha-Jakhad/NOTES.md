# pyfiglet & FIGlet Rendering Analysis (Step 1)

## 1. Repository & Commit Metadata
- **Repository URL**: `https://github.com/pwaller/pyfiglet`
- **Pinned Commit SHA**: `255827efbc09d662875dad50fa0edc1e1263bf45`
- **Commit Date**: `2026-08-02T22:51:18+01:00`
- **Commit Subject**: `Return False from isValidFont for a missing font (#161)`
- **License**: MIT License
  - Copyright © 2007–2018 Christopher Jones, Stefano Rivera, Peter Waller, and various contributors.
  - Full text located in `LICENSE`.

---

## 2. Font Setup & Missing Fonts in Fresh Clones
- **Font issue in a fresh clone before font setup**:
  When freshly cloned from GitHub, `pyfiglet/fonts/` does not exist. When `pyfiglet` is run in Python (`python -m pyfiglet hello`), `FigletFont.preloadFont()` searches for fonts in package resources `pyfiglet.fonts`, the local directory, and system paths. Because `pyfiglet/fonts/` has not been populated yet, `FigletFont` fails with `FontNotFound: standard`. (Note: This is an inherent repo design characteristic of pyfiglet prior to running font setup, distinct from any separate terminal/environment issues such as Python executable availability).
- **The Makefile and setup.py Fix**:
  The repository maintains standard fonts under `pyfiglet/fonts-standard/`.
  - In `Makefile` (`minimal` target):
    ```sh
    cp pyfiglet/fonts-standard/* pyfiglet/fonts/
    ```
  - In `setup.py` (lines 8–13), a self-bootstrapping check copies `fonts-standard` into `fonts` if `pyfiglet/fonts` is missing:
    ```python
    if not path.isdir(pkg_src):
        shutil.copytree(repo_src, pkg_src)
    ```
  - Preparation command:
    ```sh
    mkdir -p pyfiglet/fonts && cp pyfiglet/fonts-standard/* pyfiglet/fonts/
    ```

---

## 3. How FIGlet Text Rendering Works
FIGlet does not construct characters pixel by pixel. Instead, it stitches together pre-rendered multi-line ASCII art glyphs defined in font files (`.flf` / `.tlf`):

1. **Font Parsing (`FigletFont.loadFont`)**:
   - The header specifies font metrics: `height` (number of rows per character), `baseline`, `hardBlank` (a reserved character, commonly `$`, representing non-collapsible whitespace), `printDirection` (0 for LTR, 1 for RTL), and layout/smush flags (`oldLayout` or `fullLayout`).
   - Each character glyph is loaded as an array of `height` strings with trailing end markers stripped.
2. **Line Buffer Accumulation (`FigletBuilder`)**:
   - The builder maintains a multi-line buffer (`self.buffer`), initialized to `height` empty strings.
   - For each input character, it computes the character width and calculates `maxSmush` via `smushAmount()` (how many columns the character can shift left into the previous glyph).
3. **Word Wrapping & Line Breaking**:
   - When `currentTotalWidth = len(buffer[0]) + curCharWidth - maxSmush >= width`, the line overflows.
   - The builder records positions where spaces occur (`self.blankMarkers`).
   - If a space exists on the current line, it wraps at the last space (`cutBufferAtLastBlank`), rewinding the character iterator.
   - If no space exists (e.g. an unbroken string longer than `width`), it breaks at the previous character (`cutBufferAtLastChar`).
4. **Row Merging**:
   - If the character fits, each row of the glyph is merged into `buffer[row]`.
   - Overlapping columns are resolved according to kerning and smushing rules.
5. **Justification & Post-Processing (`formatProduct`)**:
   - When a line buffer is flushed:
     - **Justification**: Left, center, or right alignment pads leading spaces onto each row.
     - **Hardblank Replacement**: All occurrences of the font's `hardBlank` character are replaced with standard spaces (`' '`).

---

## 4. Spacing Modes: Full-Width, Kerning, and Smushing

In FIGlet, horizontal spacing between consecutive characters is controlled by the font's layout mode bitmask:

1. **Full-Width Spacing (`smushMode == 0`)**:
   - Neither kerning (`SM_KERN = 64`) nor smushing (`SM_SMUSH = 128`) is active.
   - `maxSmush = 0`.
   - Glyphs are placed side-by-side using their full bounding box widths, preserving all surrounding whitespace.
2. **Kerning (`SM_KERN = 64`)**:
   - Eliminates whitespace between glyphs by sliding them horizontally until their visible non-space ink characters touch.
   - Touching non-space characters **never** overlap or combine: `smushChars()` returns `None` when `SM_SMUSH` is not set (`pyfiglet/__init__.py` lines 798–800).
3. **Smushing (`SM_SMUSH = 128`)**:
   - Allows characters to shift one column further so touching ink merges into a single character.
   - **Universal Smushing** (`smushMode & 63 == 0`):
     Overlaps touching ink unconditionally. Preference is given to visible characters over hardblanks; otherwise the later character in text direction wins (`pyfiglet/__init__.py` lines 801–815).
   - **Controlled Smushing** (`smushMode & 63 != 0`):
     Governed by six specific bit flags (bits 1–32). Two characters only smush if an active rule permits it.

---

## 5. The Six Controlled Smushing Rules

From `pyfiglet/__init__.py` (lines 534–542 and 817–863):

| Rule | Constant & Bit | Condition | Result | Examples |
| :--- | :--- | :--- | :--- | :--- |
| **Rule 1: Equal Character** | `SM_EQUAL = 1` | `left == right` (excluding hardblanks) | Returns the character itself | `\|` + `\|` $\rightarrow$ `\|`<br>`/` + `/` $\rightarrow$ `/` |
| **Rule 2: Underscore / Lowline** | `SM_LOWLINE = 2` | One char is `_`, other is in `\|/\[]{}()<>` | The other character overrides `_` | `_` + `\|` $\rightarrow$ `\|`<br>`/` + `_` $\rightarrow$ `/` |
| **Rule 3: Hierarchy** | `SM_HIERARCHY = 4` | Both chars belong to the hierarchy classes:<br>Class 1: `\|`<br>Class 2: `/`, `\`<br>Class 3: `[`, `]`<br>Class 4: `{`, `}`<br>Class 5: `(`, `)`<br>Class 6: `<`, `>` | The character from the higher (stronger) class wins | `\|` (1) + `/` (2) $\rightarrow$ `/`<br>`[` (3) + `{` (4) $\rightarrow$ `{`<br>`(` (5) + `<` (6) $\rightarrow$ `<` |
| **Rule 4: Opposite Pair** | `SM_PAIR = 8` | Opposing brackets touch: `[]`, `{}`, or `()` in either orientation | Replaced by `\|` | `[` + `]` $\rightarrow$ `\|`<br>`}` + `{` $\rightarrow$ `\|`<br>`(` + `)` $\rightarrow$ `\|` |
| **Rule 5: Big X** | `SM_BIGX = 16` | Specific slashes and angles meet (`pyfiglet/__init__.py#L855-L862`):<br>• `left == '/'` and `right == '\\'`<br>• `left == '\\'` and `right == '/'`<br>• `left == '>'` and `right == '<'` | • `/` + `\` $\rightarrow$ `\|`<br>• `\` + `/` $\rightarrow$ `Y`<br>• `>` + `<` $\rightarrow$ `X` | `/` + `\` $\rightarrow$ `\|`<br>`\` + `/` $\rightarrow$ `Y`<br>`>` + `<` $\rightarrow$ `X` |
| **Rule 6: Hardblank** | `SM_HARDBLANK = 32` | Both characters are the font's `hardBlank` | Replaced by one `hardBlank` | `$` + `$` $\rightarrow$ `$` (if `$` is hardblank) |

*Note on hardblanks under controlled smushing*: If Rule 6 is not active, or if only one of the characters is a hardblank, controlled smushing explicitly rejects the smush (`return None` on line 824).

---

## 6. Relevant Command-Line Options

From `main()` in `pyfiglet/__init__.py` (lines 948–984 and 1012–1029):

1. **`-f, --font FONT`** (default: `standard`):
   Selects the FIGlet font to render with (e.g. `slant`, `standard`, `banner`).
2. **`-w, --width COLS`** (default: `80`):
   Sets terminal column width used for word wrapping and justification.
3. **`-j, --justify SIDE`** (choices: `auto`, `left`, `center`, `right`; default: `auto`):
   Sets text alignment. In `auto` mode, resolves to `left` for left-to-right fonts and `right` for right-to-left fonts.
4. **`-D, --direction DIRECTION`** (choices: `auto`, `left-to-right`, `right-to-left`; default: `auto`):
   Sets text formatting direction. In `auto` mode, reads the font header's `printDirection`.
5. **`-r, --reverse`** (flag, default: `False`):
   Horizontally mirrors rendered output text using `__reverse_map__`, flipping symmetrical characters like `/` $\leftrightarrow$ `\` and `(` $\leftrightarrow$ `)`.
6. **`-F, --flip`** (flag, default: `False`):
   Vertically flips rendered output text upside down using `__flip_map__`, inverting characters like `^` $\leftrightarrow$ `v`.
7. **`-s, --strip-surrounding-newlines`** (flag, default: `False`):
   Removes empty leading and trailing lines from output while preserving internal spacing.
8. **`-n, --normalize-surrounding-newlines`** (flag, default: `False`):
   Ensures the output text is padded with exactly one empty leading newline and one trailing newline.

*(Note: `-c` color, `-L` load font, `-l` list fonts, and `-i` info font are excluded from this assignment).*

---

## 7. Font Directories & Separation

- **`pyfiglet/fonts-standard/`** (72 files: 67 .flf files, 4 .flc files, and 1 __init__.py):
  Standard fonts with clear open-source / public domain licensing (Fedora compliant). **Included**.
- **`pyfiglet/fonts-contrib/`** (521 fonts):
  Contributed fonts with varied or unverified redistribution terms. **Excluded**.
- **`test-fonts/`** (`TEST_ONLY.flf`):
  Used solely for test fixture verification (`pyfiglet/tests/test_cli.py`). **Excluded**.
- **`pyfiglet/fonts/`**:
  Destination directory containing prepared fonts for package usage. Contains 72 files (67 .flf files, 4 .flc files, and 1 __init__.py).

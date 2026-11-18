# pyfiglet notes
commit SHA:255827efbc09d662875dad50fa0edc1e1263bf45 2026-08-02T22:51:18+01:00
## 1. Licence

The project is licensed under the MIT License (MIT).

This is stated in `LICENSE`, and `setup.py` also declares `license='MIT'` and includes the classifier `License :: OSI Approved :: MIT License`.

## 2. Why `python -m pyfiglet hello` fails straight after cloning

A fresh checkout does not necessarily have a generated `pyfiglet/fonts` directory. The font files are kept under `pyfiglet/fonts-standard` (and `pyfiglet/fonts-contrib`), and the package does not copy them into `pyfiglet/fonts` until a build step runs.

The default font is `standard`, and `FigletFont.preloadFont()` looks for `standard.tlf` / `standard.flf` under `pyfiglet.fonts` before falling back to local system directories. If the generated `pyfiglet/fonts` package is missing, the lookup fails and the command exits with a font-not-found error.

The Makefile step that fixes this is:

- `make minimal`

That target does:

- `clean` (removes `pyfiglet/fonts/` and recreates it), then
- `cp pyfiglet/fonts-standard/* pyfiglet/fonts`

After that, the default font exists and `python -m pyfiglet hello` works.

`make full` does the same plus copies the contributed fonts, so it also fixes the problem. `setup.py` also contains a fallback that copies `pyfiglet/fonts-standard` to `pyfiglet/fonts` if the directory is absent, but in a raw clone the direct `python -m pyfiglet` command still fails until one of those generation steps has run.

## 3. Smushing rules and kerning

`FigletBuilder` defines the smushing bits from the old figlet layout. In the code, these are:

- `SM_EQUAL = 1`
- `SM_LOWLINE = 2`
- `SM_HIERARCHY = 4`
- `SM_PAIR = 8`
- `SM_BIGX = 16`
- `SM_HARDBLANK = 32`
- `SM_KERN = 64`
- `SM_SMUSH = 128`

The six controlled smushing rules are the six bits from `SM_EQUAL` through `SM_HARDBLANK`. Kerning is `SM_KERN`. The code treats a space as a special case: a space always yields to the other character.

### Equal character rule (`SM_EQUAL`)

Two identical glyph edges collapse to one.

Example: `A` + `A` => `A`

### Underscore rule (`SM_LOWLINE`)

An underscore can merge with a member of the hierarchy set (`|`, `/`, `\`, `[`, `]`, `{`, `}`, `(`, `)`, `<`, `>`).

Example: `_` + `|` => `|`

### Hierarchy rule (`SM_HIERARCHY`)

A later, more dominant class wins over an earlier one:

- `|`
- `/\`
- `[]`
- `{}`
- `()`
- `<>`

Example: `|` + `/` => `/`

### Opposite-pair rule (`SM_PAIR`)

Facing pairs collapse to a bar (`|`).

Example: `[` + `]` => `|`

### Big-X rule (`SM_BIGX`)

Specific slash/backslash and angle-bracket combinations collapse into a single glyph.

Example: `/` + `\` => `|`

### Hardblank rule (`SM_HARDBLANK`)

Two hardblank characters collapse to a single hardblank; in the standard font, the hardblank is `$`.

Example: `$` + `$` => `$`

(Visually, the hardblank is later replaced with a space when rendered.)

### Kerning (`SM_KERN`)

Kerning slides the next glyph toward the previous one until the non-space edges touch; it does not overwrite either character.

Example: `## ` next to ` ##` => `####`

(If kerning is on but smushing is off, two literal `#` characters do not merge into one; they just move together as far as allowed.)

## 4. Option meanings

### `-f FONT` / `--font`

Selects the font to render with. Default: `standard`.

`FigletFont.preloadFont()` tries the requested font as `FONT.tlf` then `FONT.flf`, first via the package resources and then in the current directory / configured font directories.

### `-w COLS` / `--width`

Sets the rendering width used for wrapping and justification. Default: `80`.

If a character would make the line too wide, the renderer starts a new line at the last blank when possible.

### `-j SIDE` / `--justify`

Sets justification. Accepted values are `auto`, `left`, `center`, and `right`.

- `auto`: follows the print direction
- `left`: leaves rows as generated
- `center`: centers each line in the target width
- `right`: pads on the left to align to the right margin

### `-D DIRECTION` / `--direction`

Sets the text direction. Accepted values:

- `auto` (default): reads the font's direction metadata
- `left-to-right`
- `right-to-left`

This affects how glyphs are laid out and how smush decisions are applied.

### `-r` / `--reverse`

Shows a mirror image of the rendered text by reversing each row.

### `-F` / `--flip`

Flips the rendered output vertically (top-to-bottom), rather than reversing left-to-right.

### `-s` / `--strip-surrounding-newlines`

Removes empty leading and trailing lines around the rendered output.

This trims surrounding blank lines while keeping the actual glyph rows intact.

### `-n` / `--normalize-surrounding-newlines`

Ensures the output has exactly one blank line before and after the rendered text, after stripping the surrounding blank lines from the actual art.

If both `-s` and `-n` are passed, `-s` wins because it is checked first in `main()`.

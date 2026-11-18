# 1. Repository description

pyfiglet is an MIT-licensed Python implementation of FIGlet that generates ASCII banner art by rendering input text using FIGfonts (`.flf` files), with `standard` as its default font. It parses font definitions and arranges characters using kerning (closing spaces between glyphs) and smushing rules (equal characters, underscore smushing, character hierarchy rankings, opposite bracket pairs, big X smushing, and hardblank handling) to neatly merge or space glyphs. pyfiglet also provides command-line controls for font selection (`-f`), output line width and word wrapping (`-w`), horizontal alignment (`-j`), writing direction (`-D`), horizontal reversal (`-r`), vertical flipping (`-F`), and stripping or normalizing surrounding newlines (`-s` and `-n`).

# 2. What to build

A Go program in `/workspace/repo`. Running `go build -o /tmp/figlet .` must produce a CLI accepting:
- `-f FONT`
- `-w COLS`
- `-j auto|left|center|right`
- `-D auto|left-to-right|right-to-left`
- `-r`
- `-F`
- `-s`
- `-n`

followed by text. Output must match pyfiglet byte for byte.

### CLI Flags and Behavior:

- `-f FONT`: Specifies the font name to render with (default: `standard`, corresponding to `standard.flf`).
- `-w COLS`: Sets the column width for line wrapping and justification (default: `80`).
  - Lines wrap when the current line width reaches or exceeds `COLS` (`currentTotalWidth >= COLS`).
  - Wrapping breaks at the last space before the limit; if no space is present, it breaks at the character.
  - If any single character's rendered width exceeds `COLS`, rendering halts with an error.
  - Justification padding for `center` and `right` is calculated using `COLS`.
- `-j auto|left|center|right`: Sets the horizontal justification of each rendered line (default: `auto`). Each wrapped line of output is justified independently:
  - `auto`: Defaults to the resolved writing direction. If the direction resolves to `left-to-right`, justification is `left`; if the direction resolves to `right-to-left`, justification is `right`.
  - `left`: Adds no leading padding spaces (each line starts at column 0). Explicit `-j left` overrides the direction default.
  - `center`: Prepends leading spaces to each row independently. For each row of a rendered line, the number of prepended spaces is `int((COLS - len(row)) / 2)` (integer division). If `COLS <= len(row)`, no padding is added.
  - `right`: Prepends leading spaces to each row independently. For each row of a rendered line, the number of prepended spaces is `COLS - len(row) - 1` (one fewer padding space than full width). If `COLS - len(row) - 1 <= 0`, no padding is added.
- `-D auto|left-to-right|right-to-left`: Sets the text formatting and rendering direction (default: `auto`).
  - `auto`: Reads the direction from the font header `printDirection` (`0` means `left-to-right`, `1` means `right-to-left`; any other value or omitted is treated as `left-to-right`).
  - `left-to-right`: Characters are rendered left to right, appending each new glyph to the right of the line buffer.
  - `right-to-left`: Characters are rendered right to left, prepending each new glyph to the left of the line buffer (swapping left and right operands during smushing).
  - Interaction with justification: When `-j auto` is active, justification is determined by the effective direction (`left` for `left-to-right`, `right` for `right-to-left`). An explicit `-j` setting (`left`, `center`, or `right`) takes precedence and is applied regardless of direction.
- `-r`: Prints a mirror image of the rendered output. Each row is reversed horizontally, and paired characters (e.g., brackets and slashes) are swapped according to pyfiglet's reverse mapping.
- `-F`: Flips the rendered output upside down. The rows are inverted vertically (from bottom to top), and paired characters (e.g., `^` and `v`, `/` and `\`) are swapped according to pyfiglet's flip mapping. If both `-r` and `-F` are specified, `-r` is applied first, then `-F`.
- `-s`: Strips surrounding empty lines from the output (removes empty leading lines before the first non-empty line and trailing empty lines after the last non-empty line, while preserving internal blank lines). If both `-s` and `-n` are specified, `-s` takes precedence.
- `-n`: Normalizes surrounding newlines by performing the same stripping as `-s`, and then adding exactly one empty line before the picture and one empty line after it.

Following any flags, the remaining arguments are concatenated with a single space to form the input text string. The CLI must output the rendered text followed by a final newline, matching pyfiglet's stdout byte for byte.

# 3. Fonts

`.flf` files must remain inside the repository, and the program must work from any working directory (e.g. running `/tmp/figlet` from any location outside `/workspace/repo`).

# 4. Hard rules

- Go standard library only.
- No `.py` files anywhere.
- `go build ./...` must pass.

# 5. Not checked

- Speed
- Memory
- Error messages
- `-c`, `-L`, `-l`, `-i`

# 6. Grading

Grading is based on hidden automatic cases and partial scores.

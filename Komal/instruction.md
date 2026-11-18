# Port pyfiglet to Go

This repository contains pyfiglet, a Python implementation of FIGlet, and its standard fonts. FIGlet renders text as large characters assembled from font glyphs. Glyphs can overlap according to font-specific kerning and smushing rules.

Build a command-line program in `/workspace/repo`. Running `go build -o /tmp/figlet .` there must produce an executable that runs from any working directory. Use only the Go standard library. Keep the repository's standard `.flf` files in the repository, even if you move or embed them. Remove every `.py` file under `/workspace/repo`. The finished repository must also pass `go build ./...`.

Match the original CLI's stdout byte for byte for successful rendering invocations, including leading spaces, trailing spaces, empty rows, line breaks, and the final newline. Accept the following options followed by text. The original joins multiple positional arguments with one space.

| Option | Meaning |
| --- | --- |
| `-f FONT` | Standard font name. Defaults to `standard`. |
| `-w COLS` | Width used for both wrapping and justification. Defaults to 80. |
| `-j auto\|left\|center\|right` | Align each rendered row within the specified width. Defaults to `auto`, which follows the resolved print direction. |
| `-D auto\|left-to-right\|right-to-left` | Set print direction. Defaults to `auto`, which uses the font's direction when specified. |
| `-r` | Mirror the rendered output horizontally. |
| `-F` | Flip the rendered output vertically after any mirroring. |
| `-s` | Strip surrounding whitespace as the original does. Takes precedence over `-n`. |
| `-n` | Normalize surrounding newlines as the original does. |

Automatic grading awards partial credit. Speed, memory usage, error-message wording, color (`-c`), and font installation, listing, and information (`-L`, `-l`, `-i`) are out of scope.

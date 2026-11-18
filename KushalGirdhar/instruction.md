# Port pyfiglet to Go

The repository in `/workspace/repo` contains the original pyfiglet source and its prepared standard FIGlet fonts. pyfiglet turns text into character art using a font's glyphs, spacing, and overlap rules. Use the provided source and font files as the behavioral reference.

Implement a Go standard-library command-line program. From `/workspace/repo`, `go build -o /tmp/figlet .` must produce an executable that works from any working directory. Accept the following options followed by text. Join multiple text arguments with one space, as pyfiglet does.

| Option | Behavior |
| --- | --- |
| `-f FONT` | Select a prepared standard font. Default `standard`. |
| `-w COLS` | Set the column width for wrapping and justification. Default 80. |
| `-j auto\|left\|center\|right` | Set justification using the selected width. Default `auto`, which follows the effective direction. |
| `-D auto\|left-to-right\|right-to-left` | Set rendering direction. Default `auto`, which uses the font direction and falls back to left-to-right. |
| `-r` | Mirror the rendered output horizontally. |
| `-F` | Flip the rendered output vertically. |
| `-s` | Strip surrounding blank rows using pyfiglet's whitespace handling. |
| `-n` | Normalize surrounding blank rows using pyfiglet's whitespace handling. |

Match pyfiglet stdout byte for byte, including unsupported-character handling, wrapping, all spaces, blank rows, and final newlines. Apply mirroring before flipping. Apply stripping or normalization last, with `-s` taking precedence over `-n`.

Keep the `.flf` files in the repository. You may embed them. `go build ./...` must pass. Remove every `.py` file from the final repository.

Automatic grading gives partial credit for build success and exact output. The Python-removal credit requires a successful build and at least one matching output. Color, font installation, font listing, font information, error wording, speed, and memory are outside the scope.

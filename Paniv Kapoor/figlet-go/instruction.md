# Port PyFiglet to Go

The repository at `/workspace/repo` contains the original PyFiglet source and the standard FIGlet fonts. PyFiglet is a Python port of FIGlet from https://github.com/pwaller/pyfiglet. It converts text into large ASCII-art letters using FIGlet fonts.

Read the Python implementation and reproduce its rendering behavior in Go. When you are finished, the repository must contain only the Go implementation and the fonts it needs.

## Implementation

Implement the program in Go inside `/workspace/repo`.

From `/workspace/repo`, this command must build the CLI:

```bash
go build -o /tmp/figlet .
```

The CLI must accept these options:

| Option | Meaning | Default |
| --- | --- | --- |
| `-f FONT` | FIGlet font name to render with | `standard` |
| `-w COLS` | Terminal width used for wrapping and justification | `80` |
| `-j auto\|left\|center\|right` | Horizontal justification | `auto` |
| `-D auto\|left-to-right\|right-to-left` | Print direction | `auto` |
| `-r` | Show the mirror image of the rendered text | off |
| `-F` | Flip the rendered text vertically | off |
| `-s` | Remove empty leading and trailing lines | off |
| `-n` | Surround the output with exactly one empty line before and after | off |

The remaining positional arguments are the text to render. Join them with a single space, in the same way the original program does.

`auto` direction follows the font's own print direction. `auto` justification is left for left-to-right text and right for right-to-left text.

Apply post-processing in this order: reverse (`-r`), then flip (`-F`), then newline handling. If both `-s` and `-n` are set, `-s` wins. Write the rendered result as UTF-8 and end it with a newline.

Output must match the original Python implementation byte-for-byte, including spaces and newlines.

## Fonts

The `.flf` font files are in the repository. You may move or embed them, but the font files must remain part of the repository. The executable must find the correct font no matter what the current working directory is.

## Hard constraints

- Use the Go standard library only.
- Do not use third-party Go dependencies.
- Do not leave any `.py` files anywhere inside `/workspace/repo`.
- `go build ./...` must succeed.

## Out of scope

- Speed and memory optimization.
- Exact error messages. A missing font or missing text should still cause a non-zero exit.
- Options `-c`, `-L`, `-l`, and `-i`.

## Evaluation

The implementation will be evaluated automatically against the original PyFiglet behavior. Exact output matters, including spaces and newlines. Partial correctness receives partial credit.

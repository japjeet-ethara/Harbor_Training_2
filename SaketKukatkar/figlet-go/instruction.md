# Instructions

## 1. Repository

`/workspace/repo` contains `pyfiglet`, a Python implementation of FIGlet that renders text as character art using font files, wrapping, justification, print direction, kerning, and smushing.

## 2. Required program

Implement a Go command-line replacement in `/workspace/repo`. Running:

```bash
go build -o /tmp/figlet .
```

from `/workspace/repo` must produce a command-line program. It must accept the same options as pyfiglet: `-f FONT`, `-w COLS`, `-j auto|left|center|right`, `-D auto|left-to-right|right-to-left`, `-r`, `-F`, `-s`, and `-n`, followed by the text. Match pyfiglet's output byte for byte, including spaces, blank lines, and final newlines. The defaults are font `standard`, width `80`, automatic justification, and automatic direction.

`-w` sets both the wrapping and justification width. Automatic direction follows the font metadata; automatic justification follows the resolved direction. `-r` mirrors the output, `-F` flips it vertically, `-s` strips surrounding blank lines, and `-n` adds one blank line before and after the rendering. If `-s` and `-n` are both set, `-s` takes precedence. Multiple text arguments are joined with one space.

## 3. Fonts and execution

The `.flf` font files may be moved, but must remain inside the repository. The program must locate its fonts and work when launched from any working directory.

## 4. Hard rules

- Use only the Go standard library.
- There must be no `.py` files anywhere in the repository.
- `go build ./...` must pass.

## 5. Not checked

Speed, memory use, error-message wording, and the options `-c`, `-L`, `-l`, and `-i` are not checked.

## 6. Grading

Automatic hidden cases award partial credit for a successful build, removing Python source while retaining working behavior, and exact output.

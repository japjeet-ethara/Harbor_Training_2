# Harbor task instructions

## 1. Repository

As described in `NOTES.md`, pyfiglet is a Python implementation of FIGlet that renders text as ASCII art using FIGlet fonts. The default font is `standard`, and the repository contains the font data used by the renderer.

## 2. What to build

Build a Go command-line program in `/workspace/repo`. Running `go build -o /tmp/figlet .` from `/workspace/repo` must produce an executable that accepts these pyfiglet options, followed by the text to render:

- `-f FONT`
- `-w COLS`
- `-j auto|left|center|right`
- `-D auto|left-to-right|right-to-left`
- `-r`
- `-F`
- `-s`
- `-n`

For the supported options and input text, the program must print the same output as pyfiglet, byte for byte.

## 3. Fonts

The `.flf` font files may be moved within the repository, but must remain in the repository. The program must locate and use them when invoked from any working directory.

## 4. Hard rules

- Use only the Go standard library; do not add third-party dependencies.
- No `.py` files may remain anywhere in the repository.
- `go build ./...` must pass.

## 5. Not checked

Speed, memory usage, and error messages are not checked. The options `-c`, `-L`, `-l`, and `-i` are not checked.

## 6. Grading

Grading uses hidden automatic cases. Partial credit is available.

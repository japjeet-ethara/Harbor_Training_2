# Instructions

## 1. Repository

pyfiglet is a Python implementation of FIGlet that renders text as ASCII art using FIGlet font files. Its command-line interface supports font selection, text layout, direction, and output transformations.

## 2. What to Build

Build a Go command-line program at `/workspace/repo`. From that directory, `go build -o /tmp/figlet .` must produce the program. It must accept `-f FONT`, `-w COLS`, `-j auto|left|center|right`, `-D auto|left-to-right|right-to-left`, `-r`, `-F`, `-s`, and `-n`, followed by the text, and produce output byte for byte identical to pyfiglet. `-w COLS` sets the width used for wrapping and justification (default 80); `-j` chooses justification within that width.

## 3. Fonts

The `.flf` font files may be moved but must remain in the repository. The program must find and use them when invoked from any working directory.

## 4. Hard Rules

- Use only the Go standard library.
- There must be no `.py` files anywhere in the repository.
- `go build ./...` must pass.

# Port pyfiglet to Go

## Repository
This repository contains `pyfiglet`, a Python implementation of FIGlet that renders ASCII text into large banner characters using `.flf` font files, applying kerning, six horizontal smushing rules, line wrapping, justification, print direction, horizontal reversal, and vertical flipping. Standard `.flf` font files are located inside the repository.

## What to Build
Build a Go command-line program in `/workspace/repo` such that running:
```sh
go build -o /tmp/figlet .
```
inside `/workspace/repo` produces an executable `/tmp/figlet` accepting the same CLI flags and positional text argument as `pyfiglet`:
- `-f FONT`: Font name (default `standard`).
- `-w COLS`: Output width in columns (default `80`).
- `-j auto|left|center|right`: Horizontal justification within the `-w` column width (default `auto`).
- `-D auto|left-to-right|right-to-left`: Print direction (default `auto`).
- `-r`: Reverse each output line horizontally.
- `-F`: Flip the rendered output vertically (upside down).
- `-s`: Enable smushing (default behavior).
- `-n`: Do not wrap lines; render on a single line regardless of width.
- Positional argument(s): The input text to render (joined by spaces if multiple arguments are passed, or empty string if none).

The output printed to stdout must match `pyfiglet` byte for byte.

## Fonts
The `.flf` font files may be relocated or embedded within the repository, provided they remain inside `/workspace/repo`. Your program must locate and load fonts regardless of the current working directory from which `/tmp/figlet` is executed.

## Hard Rules
- Use the Go standard library only (no external modules).
- No `.py` files may remain anywhere inside `/workspace/repo`.
- `go build ./...` in `/workspace/repo` must succeed cleanly.

## Not Checked
- Execution speed or memory usage.
- Error message wording for invalid inputs.
- Unsupported flags (`-c`, `-L`, `-l`, `-i`).

## Grading
Your solution is evaluated automatically against a suite of unseen test cases covering fonts, widths, justification, print directions, transformations, wrapping, and smushing rules. Partial credit is awarded proportionally.

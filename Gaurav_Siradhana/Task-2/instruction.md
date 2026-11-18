# Build a Go FIGlet renderer

## Repository

This repository contains pyfiglet, a Python implementation of FIGlet that
loads `.flf` and `.tlf` font files and renders multi-line text art. Its
renderer implements font-defined kerning and smushing, wrapping,
justification, direction, mirroring, and flipping.

## What to build

Build a Go command-line program in `/workspace/repo`. Running
`go build -o /tmp/figlet .` from `/workspace/repo` must create a program that
accepts `-f FONT`, `-w COLS`, `-j auto|left|center|right`,
`-D auto|left-to-right|right-to-left`, `-r`, `-F`, `-s`, and `-n`, followed
by the text to render. Its standard output must match pyfiglet byte for byte
for those options and text.

The width controls wrapping and is also the alignment width when `-j center`
or `-j right` is selected. `-j auto` follows the resolved text direction.
`-D auto` follows the selected font's print direction.

## Fonts

The `.flf` and `.tlf` font files may be moved, but they must remain inside
this repository. The program must locate its fonts and work when launched
from any current working directory.

## Hard rules

- Use only the Go standard library.
- Do not leave any `.py` files anywhere in the repository.
- `go build ./...` must pass.

## Not checked

Speed, memory use, and error-message wording are not checked. The `-c`,
`-L`, `-l`, and `-i` options are not in scope.

## Grading

Hidden automatic cases are used, and partial credit is available.

# Harbor Task Instructions

## 1. Repository

The source project is pyfiglet, a Python implementation of FIGlet that renders text as ASCII art using FIGlet font definitions. Font layout data determines how adjacent characters fit and smush together, and standard and contributed fonts are organized separately before package preparation.

## 2. What to build

Build a Go program in `/workspace/repo`. Running `go build -o /tmp/figlet .` from `/workspace/repo` must produce a command-line program that accepts the same options as pyfiglet: `-f FONT`, `-w COLS`, `-j auto|left|center|right`, `-D auto|left-to-right|right-to-left`, `-r`, `-F`, `-s`, and `-n`, followed by the text. When `-w COLS` is supplied, `-j center` centers the rendered output within that specified width (`COLS`), not a default width of 80 columns. Its output must match pyfiglet byte for byte.

## 3. Fonts

The `.flf` files may be moved, but must remain in the repository. The program must locate and use them regardless of the current working directory.

## 4. Hard rules

- Use only the Go standard library.
- Do not leave any `.py` files anywhere in the repository.
- `go build ./...` must pass.

## 5. Not checked

Speed, memory usage, and error messages are not checked. The `-c`, `-L`, `-l`, and `-i` options are not required.

## 6. Grading

Grading uses hidden automatic cases and awards partial score.
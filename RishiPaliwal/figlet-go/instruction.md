# Task instructions

## 1. Repository

Pyfiglet is a pure-Python implementation of FIGlet that renders text as ASCII art using FIGlet fonts, with support for font layout, justification, text direction, kerning, and smushing.

## 2. What to build

Implement a Go program in `/workspace/repo`. Running `go build -o /tmp/figlet .` from `/workspace/repo` must produce a command-line program that accepts `-f FONT`, `-w COLS`, `-j auto|left|center|right`, `-D auto|left-to-right|right-to-left`, `-r`, `-F`, `-s`, and `-n`, followed by the text. Its output must match pyfiglet byte for byte.

## 3. Fonts

The `.flf` files may be moved within the repository, but they must remain in the repository. The program must locate and use them regardless of the current working directory.

## 4. Hard rules

- Use only the Go standard library.
- Do not include any `.py` files anywhere in the repository.
- `go build ./...` must pass.

## 5. Not checked

Speed, memory usage, error messages, and the `-c`, `-L`, `-l`, and `-i` options are not checked.

## 6. Grading

Grading uses hidden automatic cases and awards partial credit.
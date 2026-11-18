# FIGlet in Go

## Repository

This repository is pyfiglet, a Python implementation of FIGlet that renders text as large ASCII-art lettering using FIGlet fonts.

## What to build

Implement a Go command-line program in `/workspace/repo`. Running `go build -o /tmp/figlet .` from `/workspace/repo` must produce a program that accepts `-f FONT`, `-w COLS`, `-j auto|left|center|right`, `-D auto|left-to-right|right-to-left`, `-r`, `-F`, `-s`, and `-n`, followed by text. Its output must match pyfiglet byte for byte.

`-w COLS` sets the output width used for wrapping. Justification is applied to the resulting output lines within that width.

## Fonts

The `.flf` files may be moved, but must remain inside the repository. The program must locate its fonts and work regardless of the current working directory.

## Hard rules

- Use only the Go standard library.
- Leave no `.py` files anywhere in the repository.
- `go build ./...` must pass.

## Not checked

Speed, memory use, error messages, `-c`, `-L`, `-l`, and `-i` are not checked.

## Grading

The program is graded automatically on hidden cases and can receive partial credit.

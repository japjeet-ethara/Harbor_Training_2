# Port pyfiglet to Go

## The repository

`/workspace/repo` is a checkout of **pyfiglet** (https://github.com/pwaller/pyfiglet, commit `255827efbc09d662875dad50fa0edc1e1263bf45`), a Python implementation of FIGlet. FIGlet draws text as large ASCII-art letters. The FIGlet font files (`.flf`) are in `pyfiglet/fonts-standard/` (a copy is also in `pyfiglet/fonts/`). The renderer, including the smushing, kerning, wrapping, justification, direction, reverse and flip logic, is in `pyfiglet/__init__.py`. There is no Python interpreter in this environment, so you have to read the code to port it.

## What to build

Write a Go program in `/workspace/repo` (a `go.mod` at the repository root and one `main` package there). `go build -o /tmp/figlet .`, run in `/workspace/repo`, must produce a command-line program that behaves like `python -m pyfiglet`:

```
/tmp/figlet [-f FONT] [-w COLS] [-j auto|left|center|right] [-D auto|left-to-right|right-to-left] [-r] [-F] [-s] [-n] TEXT...
```

- `-f FONT` selects a font by name (any font in `pyfiglet/fonts-standard/`; default `standard`).
- `-w COLS` is the terminal width used for wrapping and justification (default 80).
- `-j` is the justification; `-D` is the print direction; `auto` means what it means in pyfiglet.
- `-r` mirrors the output, `-F` flips it upside down, `-s` strips empty leading and trailing lines, `-n` makes the output have one empty line before and after.
- All options come before the text. Every option value is a separate argument (for example `-w 40`, never `-w40`). Flags may appear in any order.
- The text is all remaining arguments joined with a single space.
- For every combination of these options, the program writes to standard output exactly the bytes that `python -m pyfiglet` writes for the same arguments, including every space and the final newline, and exits with status 0.

## Fonts

The `.flf` files may be moved or embedded, but they must stay inside `/workspace/repo`. The program must give the same output no matter which directory it is run from.

## Hard rules

- Go standard library only (no third-party modules).
- No `.py` file may exist anywhere in `/workspace/repo` when you finish.
- `go build ./...` must succeed in `/workspace/repo`.

## Not checked

Speed, memory use, error messages and exit codes for invalid input or for a width too small to draw a letter, and the options `-c`, `-L`, `-l` and `-i`.

## Grading

Your program is built and run on a set of hidden automatic cases. The score is partial: it depends on the build succeeding, on there being no `.py` files, and, most heavily, on the share of hidden cases whose output matches exactly.

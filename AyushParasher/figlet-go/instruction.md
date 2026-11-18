# Port pyfiglet to Go

/workspace/repo contains pyfiglet (MIT licence), a pure-Python implementation of
FIGlet: it draws text as large ASCII-art letters using FIGlet font files (`.flf`).
Its rendering engine (`FigletBuilder` in `pyfiglet/__init__.py`) places characters
side by side and "smushes" neighbouring characters together using the font's
layout rules, wraps text that is too wide, justifies it, can render right-to-left,
and can mirror or flip the result. The fonts it uses are in `pyfiglet/fonts/`.

## What to build

Replace the Python implementation with a Go program in /workspace/repo.

- The main package is at the repository root: running `go build -o /tmp/figlet .`
  in /workspace/repo produces the command-line program.
- It is run as `/tmp/figlet [options] TEXT...`. Options come first, each as a
  separate argument in its short form, followed by one or more text arguments:
  - `-f FONT` — font name, a file name from `pyfiglet/fonts/` without `.flf`
    (default `standard`)
  - `-w COLS` — output width (default 80)
  - `-j auto|left|center|right` — justification (default `auto`)
  - `-D auto|left-to-right|right-to-left` — print direction (default `auto`)
  - `-r` — mirror the output
  - `-F` — flip the output upside down
  - `-s` — strip empty leading and trailing lines
  - `-n` — exactly one empty line before and after
- Text arguments are UTF-8, never start with `-`, and contain no newlines or
  other control characters. Several text arguments are joined with single spaces.

## Required behaviour

For every combination of these options and any text, standard output must be
byte-for-byte identical to the standard output of the original
`python -m pyfiglet` called with the same arguments (Python 3.12, fonts set up
as they are in this repository), including all spaces and the final newlines.

## Fonts

The `.flf` font files may be moved within /workspace/repo or embedded in the
binary. The built binary must work when it is placed outside the repository (it
is built as `/tmp/figlet`) and run from any working directory.

## Hard rules

- Go standard library only (no modules are downloaded during grading).
- go.mod declares `go 1.23` or lower; the container has Go 1.23.
- `go build ./...` in /workspace/repo compiles without errors.
- No `.py` files may remain anywhere under /workspace/repo.

## Not checked

Speed, memory use, code style, standard error, exit codes, error message
wording, inputs for which the original raises an error or never finishes (for
example a width too narrow for a character), and the options `-c`, `-L`, `-l`
and `-i`. Each case must finish within a few seconds.

## Grading

Hidden automatic tests give partial credit for: the program building; removal of
the Python files (counted only together with a building program that renders at
least some inputs correctly); and the share of hidden cases whose output matches
exactly.

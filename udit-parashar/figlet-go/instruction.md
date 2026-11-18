# Port pyfiglet to Go

## The repository

`/workspace/repo` holds a copy of **pyfiglet** (https://github.com/pwaller/pyfiglet, MIT licence), a Python
implementation of FIGlet. FIGlet turns a short text into giant letters made of ASCII characters, "smushing"
neighbouring letters together using the font's layout rules, wrapping long text to a width, and optionally
justifying, mirroring or flipping the result. The Python code is in `pyfiglet/__init__.py`; the fonts are the
`.flf` files in `pyfiglet/fonts/` (a copy of `pyfiglet/fonts-standard/`). Python itself is not installed.

## What to build

Write a Go program in `/workspace/repo` (a `go.mod` at the repository root, package `main` in the root folder)
so that running

    cd /workspace/repo && go build -o /tmp/figlet .

produces a command-line program `/tmp/figlet` that behaves exactly like `python -m pyfiglet`. Usage:

    /tmp/figlet [-f FONT] [-w COLS] [-j SIDE] [-D DIRECTION] [-r] [-F] [-s] [-n] TEXT...

- `-f FONT`: font name. Default `standard`.
- `-w COLS`: width in columns used for wrapping and justification. Default `80`.
- `-j auto|left|center|right`: justification. Default `auto`.
- `-D auto|left-to-right|right-to-left`: direction. Default `auto`.
- `-r`: mirror image of the output. `-F`: flip the output upside down.
- `-s`: remove empty leading and trailing lines. `-n`: one empty line before and after.
- Options come first, each option's value is a separate argument (for example `-w 40`), and the text is
  everything after the options. The text is all remaining arguments joined with a single space.

The program must print to standard output exactly the bytes that pyfiglet prints for the same arguments:
same characters, same spaces, same line breaks, same final newline. The text and the output are UTF-8.
Characters that the chosen font has no glyph for are skipped, as pyfiglet does. A newline character inside
the text starts a new line of giant letters.

The fonts that must work are: `standard`, `slant`, `small`, `big`, `mini`, `banner`, `shadow`, `script`.

## Fonts

The `.flf` files may be moved or embedded anywhere inside `/workspace/repo`. `/tmp/figlet` is built outside the
repository and is run from other working directories, so it must find its fonts without depending on the
current directory (for example with `go:embed`, or by an absolute path under `/workspace/repo`).

## Hard rules

- Go standard library only: `go.mod` must not list any dependency.
- No `.py` file may remain anywhere in `/workspace/repo` when you finish.
- Both `go build ./...` and `go build -o /tmp/figlet .` must succeed in `/workspace/repo`.

## Not checked

Speed, memory use, error messages and exit codes for bad input, the options `-c`, `-L`, `-l`, `-i`,
and fonts other than the eight listed above.

## Grading

Your result is graded automatically: the program is built, the repository is checked for Python files, and the
program's output is compared with pyfiglet's on a set of hidden cases that exercise all the options listed above.
The score is partial: every case you get exactly right counts.

# Port `pyfiglet` from Python to Go

## The repository

`/workspace/repo` contains **pyfiglet**, a Python implementation of FIGlet. It
renders a short text in large letters made of ASCII characters, using a font
(`.flf` files). When two large letters stand next to each other, FIGlet can
squeeze them together ("smushing" and "kerning") according to rules stored in
the font. The renderer also wraps long text, justifies it, can write right to
left, mirror and flip the result. The font files are already in the repository.

## What to build

Write a **Go program** in `/workspace/repo` (`go.mod` and the `main` package in
the repository root). Running `go build -o /tmp/figlet .` in `/workspace/repo`
must produce a command-line program that accepts the same options as pyfiglet,
followed by the text:

- `-f FONT` font name (default `standard`); any font that exists in the repository's fonts
- `-w COLS` width in columns (default `80`); used for line wrapping, and as the width for `center` and `right` justification
- `-j auto|left|center|right` justification (`auto` = what pyfiglet chooses for the text direction)
- `-D auto|left-to-right|right-to-left` text direction (`auto` = the font's own direction)
- `-r` mirror the output left to right
- `-F` flip the output upside down
- `-s` strip empty lines at the start and end of the output
- `-n` normalize the empty lines at the start and end of the output
- all remaining arguments are the text, joined with one space

For every combination of options and text, the program's standard output must
be **identical, byte for byte,** to what `python -m pyfiglet` prints for the same
arguments.

## Fonts

The `.flf` files may be moved or embedded, but must stay in the repository. The
program must give the same result when run from **any working directory** and
when the binary is copied to another folder.

## Hard rules

1. Go **standard library only**. `go.mod` must not contain any `require` line.
2. When you finish, **no `.py` file may remain anywhere** under
   `/workspace/repo`. You may read the Python code, but you must delete every
   `.py` file before you stop.
3. `go build ./...` must succeed from `/workspace/repo`.
4. You work as a normal user without root rights: write only inside
   `/workspace/repo` and `/tmp`.

## Not checked

Speed, memory use, code style, error messages, and the options `-c`, `-L`, `-l`
and `-i`.

## How it is graded

Hidden automatic cases give a partial score from three checks: the build
passes, no Python files are left (and the program produces right answers), and
the share of hidden cases whose output matches exactly. The output check carries
the most weight.

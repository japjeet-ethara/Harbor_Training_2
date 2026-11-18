# Port pyfiglet to Go

## The repository

`/workspace/repo` is pyfiglet 1.0.4 (MIT), a pure-Python implementation of FIGlet: it
draws text as large letters built from ASCII characters, using `.flf` font files. Its
rendering engine (`FigletBuilder` in `pyfiglet/__init__.py`) smushes neighbouring glyphs
together following the font's layout rules, wraps text to a terminal width, justifies it,
and can print right-to-left, mirror the result or flip it upside down. The fonts are in
`pyfiglet/fonts/` (a copy of `pyfiglet/fonts-standard/`). There is no Python interpreter in
this environment; read the code, do not try to run it.

## What to build

A Go program in `/workspace/repo`. Running `go build -o /tmp/figlet .` in `/workspace/repo`
must produce a command-line program that accepts the same options as `python -m pyfiglet`:

```
figlet [-f FONT] [-w COLS] [-j auto|left|center|right] [-D auto|left-to-right|right-to-left] [-r] [-F] [-s] [-n] TEXT...
```

Options come first, each value as its own argument (`-w 40`, not `-w40` or `--width`).
Everything after the options is the text; several text arguments are joined with one
space, exactly as pyfiglet does. The text is UTF-8 and may contain any character, spaces
included; characters the font has no glyph for are skipped, as pyfiglet does.

For every such command line the program writes to stdout exactly the bytes pyfiglet
writes, byte for byte: the same smushing, kerning, wrapping, justification, direction,
mirroring, flipping and newline handling, with `-j` and `-D` defaulting to `auto` and `-w`
to 80. `-j center` and `-j right` pad to the `-w` width. Where pyfiglet fails with an
exception (for example a glyph wider than `-w`), it writes nothing to stdout; do the same.
Exit codes and stderr are not compared.

## Fonts

The `.flf` files may be moved or renamed, but must stay inside the repository. The built
binary is copied and run from other working directories, so it must not depend on the
current directory to find its fonts (embedding them with `go:embed` is the simplest way).

## Hard rules

- Go standard library only; no third-party modules.
- No `.py` file may remain anywhere under `/workspace/repo` when you are done.
- `go build ./...` run in `/workspace/repo` must pass.

## Not checked

Speed, memory, error messages, exit codes, and the options `-c`, `-L`, `-l`, `-i`.

## Grading

A hidden set of command lines is run against your binary and compared with pyfiglet's
output. The score is partial: the share of command lines that match exactly, plus points
for the build passing and for the Python code being gone.

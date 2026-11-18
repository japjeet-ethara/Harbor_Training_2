# Port pyfiglet to Go

## The repository

`/workspace/repo` contains **pyfiglet** (MIT licence, commit
`255827efbc09d662875dad50fa0edc1e1263bf45`), a pure-Python implementation of
FIGlet. It reads FIGlet font files (`.flf`) and draws text as large ASCII-art
letters. All the logic is in `pyfiglet/__init__.py`: `FigletFont` parses a
font file, `FigletBuilder` places characters next to each other using the
font's kerning/smushing rules, wraps long text at the given width and
justifies each line, and `FigletString` implements mirroring, flipping and
the trimming of surrounding blank lines. The fonts are in `pyfiglet/fonts/`.
There is no Python interpreter in this environment; read the source code.

## What to build

Replace the Python program with a Go program in `/workspace/repo`.

Running `go build -o /tmp/figlet .` inside `/workspace/repo` must produce a
command-line program that behaves like `python -m pyfiglet` for these options:

```
/tmp/figlet [-f FONT] [-w COLS] [-j auto|left|center|right]
            [-D auto|left-to-right|right-to-left] [-r] [-F] [-s] [-n] TEXT...
```

- `-f FONT` font name, e.g. `standard` or `slant` (default `standard`)
- `-w COLS` width used both for wrapping and for justification (default `80`)
- `-j` justification (default `auto`), `-D` print direction (default `auto`)
- `-r` reverse (mirror), `-F` flip, `-s` strip surrounding newlines,
  `-n` normalize surrounding newlines
- Each option and its value are separate arguments (`-w 40`); options come
  before the text. All remaining arguments are the text, joined with single
  spaces, exactly as pyfiglet's `main()` does.

For every such invocation, standard output must be **byte-for-byte identical**
to what pyfiglet's `main()` writes to standard output for the same arguments
(including all spaces, blank lines and the final newline). Behave like
pyfiglet in every detail, including its quirks; pyfiglet's code, not the
original C FIGlet, is the reference. Text may contain any printable characters,
including Latin-1 letters and characters the chosen font does not have.

## Fonts

You may move or convert the `.flf` files, but they must stay inside
`/workspace/repo` (embedding them with `go:embed` is fine). The built binary
must work when run from any working directory, including when `/tmp/figlet`
is run from outside the repository. Fonts from `pyfiglet/fonts/` are used.

## Hard rules

- Go standard library only (no third-party modules).
- When you are done, no `.py` file may exist anywhere under `/workspace/repo`.
- `go build ./...` in `/workspace/repo` must succeed.

## Not checked

Speed, memory use, error messages and exit codes for invalid input, and the
options `-c`, `-L`, `-l` and `-i`. Only inputs for which pyfiglet finishes
normally are used.

## Grading

Your program is graded automatically on hidden inputs covering the options
above. The score is partial: it combines whether the program builds, whether
the Python files are gone, and the share of inputs whose output matches
exactly.

# Port pyfiglet to Go

## The repository

`/workspace/repo` holds pyfiglet 1.0.4 (MIT), a pure-Python implementation of
FIGlet: it renders a line of text as large letters built from ASCII characters,
using `.flf` font files found in `pyfiglet/fonts/` (72 standard fonts). The whole
engine is `pyfiglet/__init__.py`. `FigletFont` parses a font; `FigletBuilder`
places glyphs side by side, overlapping their edges according to the font's
layout header ("kerning" and six "smushing" rules), wraps at a column width,
justifies each line, and `main()` parses the command line and can mirror, flip
and trim the result. There is no Python interpreter in this environment: read the
code, you cannot run it.

## What to build

A Go program in `/workspace/repo`. Running

```
go build -o /tmp/figlet .
```

in `/workspace/repo` must produce a command-line program with this interface:

```
/tmp/figlet [options] TEXT...
```

Options, all optional, given before the text:

| option | values | default |
|---|---|---|
| `-f FONT` | a font name such as `standard`, `slant`, `small`, `big`, `mini`, `banner`, `shadow`, `script` | `standard` |
| `-w COLS` | an integer; the width used for both line wrapping and justification | `80` |
| `-j SIDE` | `auto`, `left`, `center`, `right` | `auto` |
| `-D DIR` | `auto`, `left-to-right`, `right-to-left` | `auto` |
| `-r` | mirror the output | off |
| `-F` | flip the output upside down | off |
| `-s` | strip surrounding blank lines | off |
| `-n` | normalize surrounding blank lines | off |

All remaining arguments, joined with single spaces, are the text. The text may be
empty, contain repeated spaces, Latin-1 letters, or characters the font does not
have.

For every such command line the program must print **exactly** what
`python -m pyfiglet` with the same arguments prints from this repository:
the same bytes on stdout, including every trailing space, every blank line and
the final newline, and exit with status 0. Each option must mean exactly what it
means in `pyfiglet/__init__.py` (for example, `-j center` centres within the
`-w` width, not within 80 columns, and `-s` takes precedence over `-n`).

## Fonts

The `.flf` files may be moved or embedded (`go:embed`), but they must remain
inside the repository. The program is run by absolute path from other working
directories and must still find its fonts.

## Hard rules

- Go standard library only: `go.mod` has no `require` lines.
- When you are done there must be no `.py` file anywhere under `/workspace/repo`.
- `go build ./...` run in `/workspace/repo` must succeed.

## Not checked

Speed, memory, error messages and exit codes for invalid input, and the options
`-c`, `-L`, `-l`, `-i`.

## Grading

Automatic, on hidden command lines. The score is partial: it grows with the
share of command lines whose output matches, plus credit for a program that builds
and for a repository with no Python left in it.

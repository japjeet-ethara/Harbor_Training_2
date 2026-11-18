# Port pyfiglet to Go

## The repository

`/workspace/repo` contains **pyfiglet**, a Python implementation of FIGlet: a program that draws short text in large letters made of ASCII characters. The code is in `pyfiglet/__init__.py` (a `FigletBuilder` class does the layout: spacing, "smushing" of touching letters, line wrapping, justification, text direction). The fonts are the `.flf` files in `pyfiglet/fonts/`. The command line is `python -m pyfiglet [options] text`. The repository is the MIT-licensed upstream project at a fixed commit; its fonts are already installed in `pyfiglet/fonts/`.

## What to build

Write a Go program in `/workspace/repo` (a `go.mod` in `/workspace/repo` and Go source files next to it) so that, run in `/workspace/repo`,

```
go build -o /tmp/figlet .
```

produces a command-line program that accepts the same options as pyfiglet:

| Option | Meaning |
|---|---|
| `-f FONT` | font name, default `standard` |
| `-w COLS` | width in columns, default `80`; used for wrapping **and** for `-j` |
| `-j auto\|left\|center\|right` | justification inside `-w` columns; `auto` follows the text direction |
| `-D auto\|left-to-right\|right-to-left` | text direction; `auto` follows the font |
| `-r` | mirror image of the output |
| `-F` | flip the output upside down |
| `-s` | strip empty leading and trailing lines |
| `-n` | one empty line before and after the output |

Usage is `figlet [options] text...`. The text is everything after the options; several words are joined with one space. For every combination of these options and every text, the program writes **exactly the same bytes to standard output as `python -m pyfiglet` with the same arguments** (same letters, same spaces, same line breaks, same final newline), in UTF-8. Options may be combined freely. Where the original's behaviour is surprising, match the original, not what you think FIGlet should do.

## Fonts

The font files may be moved or embedded, but they must stay inside the repository (for example in `/workspace/repo/fonts/`, embedded with `go:embed`). The program must work from any working directory, and must not read anything outside `/workspace/repo` at run time except its own arguments.

## Hard rules

- Go standard library only (no third-party modules, no network).
- There must be **no `.py` file anywhere** under `/workspace/repo` when you finish. Delete them after you have read what you need.
- `go build ./...` must pass in `/workspace/repo`.
- The build must work as the normal user you are running as, with no extra setup.

## Not checked

Speed, memory use, error messages, exit codes for bad input, and the options `-c`, `-L`, `-l`, `-i`.

## Grading

Your work is graded automatically on a hidden set of cases. The score is partial: it combines whether the program builds, whether the repository is free of Python files (this only counts if the program works), and the share of hidden cases whose output matches the original exactly.

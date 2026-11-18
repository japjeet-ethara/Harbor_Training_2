# Task: Go port of pyfiglet

## What the repository is

This repository contains **pyfiglet** — a Python implementation of the classic FIGlet program. FIGlet takes a short text string and renders it as ASCII art in large letters assembled from slashes, underscores, pipes, and spaces, using font files (`.flf`). The font files are located in `pyfiglet/fonts-standard/` (already copied to `pyfiglet/fonts/`). The rendering engine supports character smushing (squeezing adjacent characters together according to six rules), kerning, text wrapping, justification, and right-to-left rendering.

## What to build

Write a Go program in `/workspace/repo`. Running the command below from `/workspace/repo` must succeed and produce a working binary:

```
go build -o /tmp/figlet .
```

The resulting binary `/tmp/figlet` must accept the same command-line options as pyfiglet and print **byte-for-byte identical output** for every input:

```
/tmp/figlet [OPTIONS] TEXT...
```

Multiple words in `TEXT` are joined with a single space (same as pyfiglet).

### Required options

| Option | Long form | Argument | Description |
|--------|-----------|----------|-------------|
| `-f` | `--font` | `FONT` | Font name to render with. Default: `standard` |
| `-w` | `--width` | `COLS` | Terminal width for wrapping and justification. Default: `80` |
| `-j` | `--justify` | `auto\|left\|center\|right` | Justification mode. Default: `auto` (follows direction: LTR → left, RTL → right) |
| `-D` | `--direction` | `auto\|left-to-right\|right-to-left` | Rendering direction. Default: `auto` (reads from font header) |
| `-r` | `--reverse` | (flag) | Mirror the output horizontally (character map + line reversal) |
| `-F` | `--flip` | (flag) | Flip the output upside-down (character map + row reversal) |
| `-s` | `--strip-surrounding-newlines` | (flag) | Strip all leading and trailing blank lines from the output |
| `-n` | `--normalize-surrounding-newlines` | (flag) | Ensure exactly one blank line before and after the output |

**Option application order:** render → reverse (if `-r`) → flip (if `-F`) → strip or normalize newlines.

**When `-s` and `-n` are both given, `-s` takes priority.**

**Justification with `-j center`** pads each row with `int((width - len(row)) / 2)` spaces on the left. This uses the terminal width (`-w`), not the natural width of the rendered text.

**Justification with `-j right`** pads each row with `width - len(row) - 1` spaces on the left.

## Fonts

The `.flf` font files are located in `pyfiglet/fonts/` within the repository. You may move them to a different subfolder inside the repository, or embed them using `go:embed`. The binary must locate its fonts correctly regardless of the current working directory when invoked.

## Hard rules

- **Go standard library only.** No third-party packages. `go.mod` must declare no external dependencies.
- **No `.py` files anywhere under `/workspace/repo`.** Delete or exclude every Python file before finishing.
- **`go build ./...` must pass** without errors when run in `/workspace/repo`.

## Grading

Grading is automatic. A hidden set of test cases compares the output of `/tmp/figlet` against the reference pyfiglet output for the same arguments. Scoring is partial — more correct cases yield a higher score.

## Not checked

Speed, memory usage, error messages for invalid input, and the options `-c`, `-L`, `-l`, `-i` are not evaluated.

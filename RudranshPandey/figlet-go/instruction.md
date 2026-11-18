# Instruction

## 1. What the Repository Is

This repository contains a checkout of [pyfiglet](https://github.com/pwaller/pyfiglet) (commit `255827ef`), a Python implementation of FIGlet — a program that renders text as large ASCII art using special `.flf` font files. The font files are in `pyfiglet/fonts/`. The Python source is reference material only; your deliverable is a Go reimplementation.

## 2. What to Build

Write a Go program in `/workspace/repo`. Running the following command from `/workspace/repo` must succeed:

```
go build -o /tmp/figlet .
```

The resulting binary `/tmp/figlet` must accept these options followed by the text to render:

| Flag | Type | Default | Meaning |
|------|------|---------|---------|
| `-f` | string | `standard` | Font name (without `.flf` extension) |
| `-w` | int | `80` | Terminal width in columns, used for line wrapping and justification |
| `-j` | string | `auto` | Justification: `auto`, `left`, `center`, or `right`. When `auto`: left-to-right text defaults to `left`, right-to-left text defaults to `right` |
| `-D` | string | `auto` | Direction: `auto`, `left-to-right`, or `right-to-left`. When `auto`, use the font's built-in print direction |
| `-r` | bool | `false` | Reverse (mirror) each output line horizontally |
| `-F` | bool | `false` | Flip output vertically (upside-down) |
| `-s` | bool | `false` | Strip surrounding empty lines from the output |
| `-n` | bool | `false` | Normalize surrounding newlines: exactly one empty line before and after the rendered text |

The output must match `pyfiglet` byte-for-byte for every combination of these options.

When `-w` is given, it controls the column width for both wrapping and justification. For example, `-w 40 -j center` centers within 40 columns, not 80.

## 3. Fonts

The `.flf` font files are in `pyfiglet/fonts/`. You may move or embed them, but they must remain inside the repository. The program must work when run from any working directory (not just `/workspace/repo`).

## 4. Hard Rules

- **Go standard library only.** No third-party modules.
- **No `.py` files** may exist anywhere in the repository when grading runs.
- `go build ./...` must pass with no errors.

## 5. Not Checked

Speed, memory usage, error messages, and the options `-c`, `-L`, `-l`, `-i` are not tested.

## 6. Grading

Your output is compared against hidden automatic test cases. Partial credit is awarded proportionally based on the number of cases that match exactly.


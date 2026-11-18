# Task Instructions: Port pyfiglet to Go

## 1. Repository Overview

This repository is derived from `pyfiglet`, a pure-Python implementation of FIGlet (a utility that renders ordinary text into large ASCII art banners using FIGlet fonts). The repository contains the original source implementation, documentation, and standard FIGlet font definitions (`.flf` files). FIGlet fonts represent characters as multi-line ASCII art glyphs, and the rendering engine lays out characters horizontally based on font-specified layout rules including kerning and smushing, while supporting terminal width wrapping, text justification, text direction, and geometric transformations.

## 2. What to Build

Build a complete Go command-line program located in `/workspace/repo`. Running `go build -o /tmp/figlet .` inside `/workspace/repo` must produce a standalone command-line executable.

The compiled program must accept the same command-line options as `pyfiglet`:
- `-f FONT`: Font to render with (default: `standard`).
- `-w COLS`: Set terminal width for wrapping and justification (default: `80`).
- `-j auto|left|center|right`: Set text justification (default: `auto`).
- `-D auto|left-to-right|right-to-left`: Set direction text is formatted in (default: `auto`).
- `-r`: Reverse output text (horizontal mirror image).
- `-F`: Flip output text (vertical flip).
- `-s`: Strip surrounding empty leading and trailing lines.
- `-n`: Normalize surrounding newlines (ensure exactly one empty line before and after).

The options are followed by positional argument(s) representing the text to render. When executed with any combination of these options and text inputs, the program must print the exact same output as `pyfiglet`, byte for byte.

## 3. Fonts

The `.flf` font files may be moved or reorganized within the repository (for example, into a subpackage or embedded into the binary using Go's `embed` package), but they must stay within the repository. The compiled binary must work correctly when executed from any working directory.

## 4. Hard Rules

- **Go Standard Library Only**: Use only the Go standard library. External modules or third-party packages are strictly forbidden.
- **No Python Files**: Delete all `.py` files from the repository. The repository must not contain any `.py` files anywhere.
- **Valid Compilation**: `go build ./...` run in `/workspace/repo` must pass without errors.

## 5. Not Checked

- Execution speed and memory usage.
- Exact error messages or exit codes on malformed inputs.
- Unchecked command-line options: `-c` / `--color`, `-L` / `--load`, `-l` / `--list_fonts`, and `-i` / `--info_font`.

## 6. Grading

Submissions are evaluated against hidden automatic test cases. Scoring is partial and based on the number of cases your implementation satisfies correctly across supported fonts, smushing rules, wrapping, justification, and flag combinations.

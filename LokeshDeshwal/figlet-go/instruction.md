# FIGlet Go Port

## What this repository is

This repository contains the source code of **pyfiglet**, a Python implementation of FIGlet that renders plain text as large ASCII-art banners using FIGfont (`.flf`) files. The fonts reside in the repository under `pyfiglet/fonts/`. The rendering engine parses font definitions, calculates character smushing and kerning across adjacent glyphs, handles terminal width wrapping, horizontal justification, text direction, character mirroring, and vertical flipping.

## What to build

Build a standalone command-line program in Go located in `/workspace/repo`.

Running the following command from `/workspace/repo` must produce a working binary at `/tmp/figlet`:

```bash
go build -o /tmp/figlet .
```

The compiled program must accept the same command-line options and arguments as pyfiglet:

- `-f FONT`: FIGlet font name to render with (default: `standard`).
- `-w COLS`: Terminal column width for wrapping and justification (default: `80`). When `-w` is given alongside `-j center` or `-j right`, justification is calculated relative to that specified column width (not 80).
- `-j SIDE`: Horizontal justification: `auto`, `left`, `center`, or `right` (default: `auto`, which aligns left for left-to-right text and right for right-to-left text).
- `-D DIRECTION`: Text formatting direction: `auto`, `left-to-right`, or `right-to-left` (default: `auto`, which reads the font's default print direction).
- `-r`: Reverse mode (mirror image of output text).
- `-F`: Flip mode (flip rendered output text upside-down).
- `-s`: Strip surrounding empty lines before and after rendered text.
- `-n`: Normalize surrounding newlines (exactly one empty line before and after). If both `-s` and `-n` are specified, `-s` takes precedence.
- Remaining positional arguments are joined with a single space to form the input text string.

The output must match the output of `python -m pyfiglet` with the identical arguments, byte for byte.

## Fonts

The `.flf` font files in `pyfiglet/fonts/` may be relocated within the repository or embedded into the binary (e.g. via `go:embed`), but they must remain within the repository. The program must function properly regardless of the current working directory from which `/tmp/figlet` is invoked.

## Hard rules

1. **Go standard library only**: No external packages, modules, or dependencies outside the Go standard library.
2. **No Python files**: No `.py` files may exist anywhere in `/workspace/repo` upon completion.
3. **Build passes**: `go build ./...` run in `/workspace/repo` must succeed without errors.

## Not checked

The following will not be checked or graded:
- Execution speed and memory usage.
- Error messages for invalid inputs.
- Unused pyfiglet options: `-c` (color), `-L` (load font), `-l` (list fonts), and `-i` (font info).

## Grading

The solution will be graded automatically against hidden test cases. Scoring is weighted across a successful build, absence of Python files, and byte-for-byte output fidelity against pyfiglet. Partial credit is awarded.
